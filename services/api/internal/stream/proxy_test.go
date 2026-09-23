package stream

import(
 "encoding/json"
 "strings"
 "testing"
 "net/http/httptest"
)

func TestProxyValidatesAndRewritesTypedChatStream(t *testing.T){
 input:=strings.Join([]string{
  `data: {"id":"upstream","object":"chat.completion.chunk","created":1,"model":"internal","choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":null}]}`,
  `data: {"id":"upstream","object":"chat.completion.chunk","created":2,"model":"internal","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
  `data: {"id":"upstream","object":"chat.completion.chunk","created":3,"model":"internal","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
  `data: [DONE]`,"",
 },"\n\n")
 w:=httptest.NewRecorder();summary,err:=Proxy(w,strings.NewReader(input),"public-model",false,true);if err!=nil{t.Fatal(err)}
 if summary.FinishReason!="stop"||summary.Usage.TotalTokens!=5||summary.FirstToken==nil{t.Fatalf("summary = %#v",summary)}
 body:=w.Body.String();if !strings.Contains(body,`"model":"public-model"`)||!strings.Contains(body,`"reasoning_tokens":0`){t.Fatalf("rewritten stream = %s",body)}
}
func TestProxyRequiresReasoningUsageWhenEnabled(t *testing.T){
 input:="data: {\"id\":\"x\",\"object\":\"text_completion\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"text\":\"x\",\"finish_reason\":\"stop\"}]}\n\ndata: {\"id\":\"x\",\"object\":\"text_completion\",\"created\":1,\"model\":\"m\",\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n"
 if _,err:=Proxy(httptest.NewRecorder(),strings.NewReader(input),"public",true,false);err==nil{t.Fatal("accepted missing reasoning_tokens")}
}
func TestProxyAssemblesToolArgumentsBeforeSuccess(t *testing.T){
 input:=strings.Join([]string{
  `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"x\":"}}]},"finish_reason":null}]}`,
  `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"","type":"","function":{"name":"","arguments":"1}"}}]},"finish_reason":"tool_calls"}]}`,
  `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"reasoning_tokens":0,"total_tokens":3}}`,
  `data: [DONE]`,"",
 },"\n\n")
 summary,err:=Proxy(httptest.NewRecorder(),strings.NewReader(input),"public",false,true);if err!=nil{t.Fatal(err)};if !summary.ToolCalls{t.Fatal("tool calls not reported")};collected,err:=Collect(strings.NewReader(input),"public",false,true);if err!=nil{t.Fatal(err)};var choices []aggregatedChatChoice;if err=json.Unmarshal(collected.Choices,&choices);err!=nil{t.Fatal(err)};if len(choices)!=1||len(choices[0].Message.ToolCalls)!=1||choices[0].Message.ToolCalls[0].Function.Arguments!="{\"x\":1}"{t.Fatalf("aggregated tool choices = %#v",choices)}
 bad:=strings.Replace(input,"1}","oops}",1);if _,err=Proxy(httptest.NewRecorder(),strings.NewReader(bad),"public",false,true);err==nil{t.Fatal("accepted malformed assembled tool arguments")}
}
func TestCollectAggregatesReasoningAndContentWithAuthoritativeUsage(t *testing.T){
 input:=strings.Join([]string{
  `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"think"},"finish_reason":null}]}`,
  `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"reasoning_content":"ing","content":"answer"},"finish_reason":null}]}`,
  `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
  `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":5,"reasoning_tokens":2,"total_tokens":9}}`,
  `data: [DONE]`,"",
 },"\n\n")
 result,err:=Collect(strings.NewReader(input),"public",true,true);if err!=nil{t.Fatal(err)};var choices []map[string]any;if err=json.Unmarshal(result.Choices,&choices);err!=nil{t.Fatal(err)};if _,exists:=choices[0]["delta"];exists{t.Fatalf("non-stream choice leaked delta: %#v",choices[0])};message:=choices[0]["message"].(map[string]any);if message["content"]!="answer"||message["reasoning_content"]!="thinking"{t.Fatalf("message = %#v",message)};if result.Summary.Usage.ReasoningTokens!=2||result.Summary.FinishReason!="stop"{t.Fatalf("summary = %#v",result.Summary)}
}
func TestCollectRejectsMissingTerminalUsage(t *testing.T){
 input:="data: {\"id\":\"x\",\"object\":\"text_completion\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"text\":\"answer\",\"finish_reason\":\"stop\"}]}\\n\\ndata: [DONE]\\n\\n"
 if _,err:=Collect(strings.NewReader(input),"public",false,false);err==nil{t.Fatal("accepted stream without terminal usage")}
}
func TestCompletionTerminalChoiceCarriesUsage(t *testing.T){
 input:=strings.Join([]string{
  `data: {"id":"x","object":"text_completion","created":1,"model":"internal","choices":[{"index":0,"text":"a","finish_reason":null}]}`,
  `data: {"id":"x","object":"text_completion","created":1,"model":"internal","choices":[{"index":0,"text":"b","finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":2,"reasoning_tokens":0,"total_tokens":4}}`,
  `data: [DONE]`,"",
 },"\n\n")
 collected,err:=Collect(strings.NewReader(input),"public",false,false);if err!=nil{t.Fatal(err)}
 if collected.Summary.Usage.TotalTokens!=4||collected.Summary.FinishReason!="stop"{t.Fatalf("summary = %#v",collected.Summary)}
 w:=httptest.NewRecorder();summary,err:=Proxy(w,strings.NewReader(input),"public",false,false);if err!=nil{t.Fatal(err)}
 if summary.Usage.TotalTokens!=4{t.Fatal("wrong stream usage")}
 events:=[]map[string]json.RawMessage{}
 for _,line:=range strings.Split(w.Body.String(),"\n"){if !strings.HasPrefix(line,"data: "){continue};var event map[string]json.RawMessage;if err=json.Unmarshal([]byte(strings.TrimPrefix(line,"data: ")),&event);err!=nil{t.Fatal(err)};events=append(events,event)}
 if len(events)!=3{t.Fatalf("got %d SSE events",len(events))}
 for i,event:=range events{if string(event["model"])!=`"public"`{t.Fatalf("event %d has wrong model",i)};if i<2&&string(event["usage"])!="null"{t.Fatalf("event %d has non-null usage",i)}}
 var choices []json.RawMessage;if err=json.Unmarshal(events[2]["choices"],&choices);err!=nil||len(choices)!=0{t.Fatal("terminal usage chunk has choices")}
 var usage struct{TotalTokens int `json:"total_tokens"`};if err=json.Unmarshal(events[2]["usage"],&usage);err!=nil||usage.TotalTokens!=4{t.Fatal("terminal usage chunk is invalid")}
}
