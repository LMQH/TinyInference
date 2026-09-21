package stream

import(
 "bufio"
 "bytes"
 "encoding/json"
 "errors"
 "io"
 "net/http"
 "strings"
 "time"

 g "mini-inference/services/api/internal/contract/generated"
 "mini-inference/services/api/internal/dmr"
)

type Summary struct{Usage g.Usage;FinishReason string;ToolCalls bool;FirstToken *time.Time}
type Collected struct{Summary Summary;Choices json.RawMessage}
type chunk struct{ID string `json:"id"`;Object string `json:"object"`;Created int64 `json:"created"`;Model string `json:"model"`;Choices []json.RawMessage `json:"choices"`;Usage *dmr.Usage `json:"usage"`}
type chatDelta struct{Role *string `json:"role"`;Content *string `json:"content"`;ReasoningContent *string `json:"reasoning_content"`;ToolCalls []dmr.ToolCall `json:"tool_calls"`}
type chatChoice struct{Index int `json:"index"`;Delta chatDelta `json:"delta"`;FinishReason *string `json:"finish_reason"`}
type completionChoice struct{Index int `json:"index"`;Text *string `json:"text"`;ReasoningContent *string `json:"reasoning_content"`;FinishReason *string `json:"finish_reason"`}
type aggregatedChatChoice struct{Index int `json:"index"`;Message dmr.ChatMessage `json:"message"`;FinishReason *string `json:"finish_reason"`}
type assembledTool struct{id,kind,name string;arguments strings.Builder}
type accumulator struct{content,reasoning,text strings.Builder;contentSeen,reasoningSeen,roleSeen bool;tools map[int]*assembledTool}

func Proxy(w http.ResponseWriter,r io.Reader,publicModel string,reasoningEnabled,chat bool)(Summary,error){
 flusher,ok:=w.(http.Flusher);if !ok{return Summary{},errors.New("streaming unsupported")};w.Header().Set("Content-Type","text/event-stream; charset=utf-8");w.Header().Set("Cache-Control","no-cache, no-transform");w.Header().Set("X-Accel-Buffering","no");w.WriteHeader(http.StatusOK)
 result,err:=consume(r,publicModel,reasoningEnabled,chat,func(out []byte)error{if _,e:=w.Write([]byte("data: "));e!=nil{return e};if _,e:=w.Write(out);e!=nil{return e};if _,e:=w.Write([]byte("\n\n"));e!=nil{return e};flusher.Flush();return nil});return result.Summary,err
}
func Collect(r io.Reader,publicModel string,reasoningEnabled,chat bool)(Collected,error){return consume(r,publicModel,reasoningEnabled,chat,nil)}
func consume(r io.Reader,publicModel string,reasoningEnabled,chat bool,emit func([]byte)error)(Collected,error){
 scanner:=bufio.NewScanner(r);scanner.Buffer(make([]byte,4096),1<<20);result:=Collected{};done,terminal,usageSeen:=false,false,false;acc:=accumulator{tools:map[int]*assembledTool{}};expectedObject:="text_completion";if chat{expectedObject="chat.completion.chunk"};streamID,streamModel:="","";
 for scanner.Scan(){
  line:=scanner.Bytes();if len(line)==0||line[0]==':'{continue};if !bytes.HasPrefix(line,[]byte("data: ")){return result,errors.New("invalid SSE framing")};data:=line[6:];if bytes.Equal(data,[]byte("[DONE]")){done=true;break}
  var raw map[string]json.RawMessage;var c chunk;if json.Unmarshal(data,&raw)!=nil||json.Unmarshal(data,&c)!=nil||c.ID==""||c.Object!=expectedObject||c.Model==""||raw["choices"]==nil{return result,errors.New("invalid SSE chunk")};if streamID==""{streamID, streamModel = c.ID, c.Model}else if c.ID != streamID || c.Model != streamModel{return result,errors.New("inconsistent SSE chunk")}
  if c.Usage!=nil{
   if usageSeen||!terminal||len(c.Choices)!=0{return result,errors.New("invalid usage chunk")};usageSeen=true;if reasoningEnabled&&!c.Usage.ReasoningPresent||!reasoningEnabled&&c.Usage.ReasoningTokens!=0||c.Usage.PromptTokens<0||c.Usage.CompletionTokens<0||c.Usage.ReasoningTokens<0||c.Usage.ReasoningTokens>c.Usage.CompletionTokens||c.Usage.TotalTokens!=c.Usage.PromptTokens+c.Usage.CompletionTokens{return result,errors.New("invalid usage")};result.Summary.Usage=g.Usage{PromptTokens:c.Usage.PromptTokens,CompletionTokens:c.Usage.CompletionTokens,ReasoningTokens:c.Usage.ReasoningTokens,TotalTokens:c.Usage.TotalTokens};usage,_:=json.Marshal(result.Summary.Usage);raw["usage"]=usage
  }else{
   if terminal||usageSeen||len(c.Choices)!=1{return result,errors.New("invalid choice chunk")};var finish *string;var token bool;var err error
   if chat{var choice chatChoice;choice,finish,token,err=decodeChatChoice(c.Choices[0],reasoningEnabled,acc.tools);if choice.Delta.Role!=nil{if acc.roleSeen{return result,errors.New("duplicate assistant role")};acc.roleSeen=true};if emit==nil{if choice.Delta.Content!=nil{acc.contentSeen=true;acc.content.WriteString(*choice.Delta.Content)};if choice.Delta.ReasoningContent!=nil{acc.reasoningSeen=true;acc.reasoning.WriteString(*choice.Delta.ReasoningContent)}}}else{var choice completionChoice;choice,finish,token,err=decodeCompletionChoice(c.Choices[0],reasoningEnabled);if emit==nil{if choice.Text!=nil{acc.text.WriteString(*choice.Text)};if choice.ReasoningContent!=nil{acc.reasoningSeen=true;acc.reasoning.WriteString(*choice.ReasoningContent)}}}
   if err!=nil{return result,err};if token&&result.Summary.FirstToken==nil{now:=time.Now().UTC();result.Summary.FirstToken=&now};if finish!=nil{if *finish!="stop"&&*finish!="length"&&*finish!="tool_calls"{return result,errors.New("invalid finish reason")};terminal=true;result.Summary.FinishReason=*finish}
  }
  if emit!=nil{model,_:=json.Marshal(publicModel);raw["model"]=model;out,e:=json.Marshal(raw);if e!=nil{return result,e};if e=emit(out);e!=nil{return result,e}}
 }
 if e:=scanner.Err();e!=nil{return result,e};if !done||!terminal||!usageSeen{return result,errors.New("incomplete SSE")};if chat&&!acc.roleSeen{return result,errors.New("missing assistant role")};if e:=validateTools(acc.tools,result.Summary.FinishReason);e!=nil{return result,e};result.Summary.ToolCalls=len(acc.tools)>0
 if emit==nil{choices,e:=aggregateChoices(chat,&acc,result.Summary.FinishReason);if e!=nil{return result,e};result.Choices=choices};return result,nil
}
func aggregateChoices(chat bool,acc *accumulator,finish string)(json.RawMessage,error){
 if !chat{var reasoning *string;if acc.reasoningSeen{x:=acc.reasoning.String();reasoning=&x};return json.Marshal([]dmr.CompletionChoice{{Index:0,Text:acc.text.String(),ReasoningContent:reasoning,FinishReason:&finish}})}
 var content,reasoning *string;if acc.contentSeen{x:=acc.content.String();content=&x};if acc.reasoningSeen{x:=acc.reasoning.String();reasoning=&x};calls:=make([]dmr.ToolCall,0,len(acc.tools));for i:=0;i<len(acc.tools);i++{tool:=acc.tools[i];calls=append(calls,dmr.ToolCall{ID:tool.id,Type:tool.kind,Function:dmr.Function{Name:tool.name,Arguments:tool.arguments.String()}})};message:=dmr.ChatMessage{Role:"assistant",Content:content,ReasoningContent:reasoning,ToolCalls:calls};return json.Marshal([]aggregatedChatChoice{{Index:0,Message:message,FinishReason:&finish}})
}
func decodeChatChoice(raw json.RawMessage,reasoning bool,tools map[int]*assembledTool)(chatChoice,*string,bool,error){var fields map[string]json.RawMessage;var choice chatChoice;if json.Unmarshal(raw,&fields)!=nil||json.Unmarshal(raw,&choice)!=nil||choice.Index!=0||fields["delta"]==nil{return choice,nil,false,errors.New("invalid chat choice")};if choice.Delta.Role!=nil&&*choice.Delta.Role!="assistant"{return choice,nil,false,errors.New("invalid chat role")};if !reasoning&&choice.Delta.ReasoningContent!=nil&&*choice.Delta.ReasoningContent!=""{return choice,nil,false,errors.New("reasoning disabled protocol mismatch")};token:=choice.Delta.Content!=nil&&*choice.Delta.Content!=""||choice.Delta.ReasoningContent!=nil&&*choice.Delta.ReasoningContent!="";for _,call:=range choice.Delta.ToolCalls{if call.Index==nil||*call.Index<0{return choice,nil,false,errors.New("invalid tool call index")};part:=tools[*call.Index];if part==nil{part=&assembledTool{};tools[*call.Index]=part};if e:=mergeTool(part,call);e!=nil{return choice,nil,false,e};if call.Function.Arguments!=""{token=true}};return choice,choice.FinishReason,token,nil}
func decodeCompletionChoice(raw json.RawMessage,reasoning bool)(completionChoice,*string,bool,error){var fields map[string]json.RawMessage;var choice completionChoice;if json.Unmarshal(raw,&fields)!=nil||json.Unmarshal(raw,&choice)!=nil||choice.Index!=0||fields["text"]==nil{return choice,nil,false,errors.New("invalid completion choice")};if !reasoning&&choice.ReasoningContent!=nil&&*choice.ReasoningContent!=""{return choice,nil,false,errors.New("reasoning disabled protocol mismatch")};token:=choice.Text!=nil&&*choice.Text!=""||choice.ReasoningContent!=nil&&*choice.ReasoningContent!="";return choice,choice.FinishReason,token,nil}
func mergeTool(dst *assembledTool,call dmr.ToolCall)error{if call.ID!=""{if dst.id!=""&&dst.id!=call.ID{return errors.New("tool call id changed")};dst.id=call.ID};if call.Type!=""{if call.Type!="function"||dst.kind!=""&&dst.kind!=call.Type{return errors.New("invalid tool call type")};dst.kind=call.Type};if call.Function.Name!=""{if dst.name!=""&&dst.name!=call.Function.Name{return errors.New("tool call name changed")};dst.name=call.Function.Name};dst.arguments.WriteString(call.Function.Arguments);return nil}
func validateTools(tools map[int]*assembledTool,finish string)error{if len(tools)==0{if finish=="tool_calls"{return errors.New("tool finish without calls")};return nil};if finish!="tool_calls"{return errors.New("tool calls without tool finish")};for i:=0;i<len(tools);i++{tool:=tools[i];if tool==nil||tool.id==""||tool.kind!="function"||tool.name==""{return errors.New("incomplete tool call")};raw:=[]byte(tool.arguments.String());if !json.Valid(raw){return errors.New("invalid tool arguments")};var object map[string]json.RawMessage;if json.Unmarshal(raw,&object)!=nil||object==nil{return errors.New("tool arguments must be an object")};};return nil}
func Done(w http.ResponseWriter)error{_,e:=w.Write([]byte("data: [DONE]\n\n"));if f,ok:=w.(http.Flusher);ok{f.Flush()};return e}
func Error(w http.ResponseWriter,envelope any){b,_:=json.Marshal(envelope);_,_=w.Write([]byte("data: "));_,_=w.Write(b);_,_=w.Write([]byte("\n\ndata: [DONE]\n\n"));if f,ok:=w.(http.Flusher);ok{f.Flush()}}
