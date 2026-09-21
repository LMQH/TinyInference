package tokenizer

import (
 "bufio"
 "bytes"
 "encoding/binary"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "os"
 "sort"
 "strings"
 "unicode"
)

const toolInstructions="# Tools\n\nYou are provided with function signatures within <tools></tools> XML tags:\n<tools>"
const toolInstructionsSuffix="\n</tools>\n\nTool usage guidelines:\n- You may call zero or more functions. If no function calls are needed, just answer normally and do not include any <function ... </function>.\n- When calling a function, return an XML object within <function ... </function> using:\n<function name=\"function-name\"><param name=\"param-name\">param-value</param></function>\n- param-value may be multi-line. If it contains <, & or newline characters, wrap it in a CDATA block: <param name=\"param-name\"><![CDATA[...multi-line value...]]></param>"
type Tokenizer struct{vocab map[string]int;ranks map[string]int;bos string;addBOS bool;control []string;userDefined []string;chatTemplate string;byteRunes [256]rune}
type ChatMessage struct{Role,Content,ToolCalls,ToolCallID string}

func LoadGGUF(path string)(*Tokenizer,error){
 f,e:=os.Open(path);if e!=nil{return nil,e};defer f.Close();r:=bufio.NewReaderSize(f,1<<20)
 magic:=make([]byte,4);if _,e=io.ReadFull(r,magic);e!=nil||string(magic)!="GGUF"{return nil,errors.New("invalid GGUF header")}
 var version uint32;if e=binary.Read(r,binary.LittleEndian,&version);e!=nil||version<2||version>3{return nil,errors.New("unsupported GGUF version")}
 var tensors,metadata uint64;if binary.Read(r,binary.LittleEndian,&tensors)!=nil||binary.Read(r,binary.LittleEndian,&metadata)!=nil{return nil,errors.New("invalid GGUF counts")};_ = tensors
 values:=map[string]any{};for range metadata{k,e:=readString(r);if e!=nil{return nil,e};var typ uint32;if e=binary.Read(r,binary.LittleEndian,&typ);e!=nil{return nil,e};v,e:=readValue(r,typ);if e!=nil{return nil,fmt.Errorf("metadata %s: %w",k,e)};values[k]=v}
 tokens,ok:=values["tokenizer.ggml.tokens"].([]string);if !ok||len(tokens)==0{return nil,errors.New("GGUF tokenizer tokens missing")}
 if values["tokenizer.ggml.model"]!="gpt2"||values["tokenizer.ggml.pre"]!="minicpm5"{return nil,errors.New("unsupported tokenizer identity")}
 template,ok:=values["tokenizer.chat_template"].(string);if !ok||!strings.Contains(template,"<|im_start|>system\\n")||!strings.Contains(template,"<tool_response>")||!strings.Contains(template,"add_generation_prompt"){return nil,errors.New("unsupported chat template")}
 t:=&Tokenizer{vocab:make(map[string]int,len(tokens)),ranks:map[string]int{},chatTemplate:template,byteRunes:gpt2ByteRunes()}
 for id,s:=range tokens{t.vocab[s]=id}
 if merges,ok:=values["tokenizer.ggml.merges"].([]string);ok{for rank,m:=range merges{t.ranks[m]=rank}}
 if id,ok:=integer(values["tokenizer.ggml.bos_token_id"]);ok&&id>=0&&int(id)<len(tokens){t.bos=tokens[id]}
 if b,ok:=values["tokenizer.ggml.add_bos_token"].(bool);ok{t.addBOS=b}
 if types,ok:=values["tokenizer.ggml.token_type"].([]any);ok&&len(types)==len(tokens){for i,v:=range types{if n,ok:=integer(v);ok{if n==3{t.control=append(t.control,tokens[i])}else if n==4{t.userDefined=append(t.userDefined,tokens[i])}}}}else{return nil,errors.New("GGUF tokenizer token types missing")}
 sort.Slice(t.control,func(i,j int)bool{return len(t.control[i])>len(t.control[j])});sort.Slice(t.userDefined,func(i,j int)bool{return len(t.userDefined[i])>len(t.userDefined[j])})
 if t.bos==""{return nil,errors.New("GGUF tokenizer BOS missing")}
 return t,nil
}

func integer(v any)(int64,bool){switch n:=v.(type){case uint8:return int64(n),true;case int8:return int64(n),true;case uint16:return int64(n),true;case int16:return int64(n),true;case uint32:return int64(n),true;case int32:return int64(n),true;case uint64:if n<=1<<63-1{return int64(n),true};case int64:return n,true};return 0,false}
func readString(r io.Reader)(string,error){var n uint64;if e:=binary.Read(r,binary.LittleEndian,&n);e!=nil{return "",e};if n>64<<20{return "",errors.New("oversize GGUF string")};b:=make([]byte,int(n));_,e:=io.ReadFull(r,b);return string(b),e}
func readValue(r io.Reader,typ uint32)(any,error){switch typ{case 0:var v uint8;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 1:var v int8;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 2:var v uint16;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 3:var v int16;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 4:var v uint32;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 5:var v int32;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 6:var v float32;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 7:var v uint8;e:=binary.Read(r,binary.LittleEndian,&v);return v!=0,e;case 8:return readString(r);case 9:var elem uint32;var n uint64;if e:=binary.Read(r,binary.LittleEndian,&elem);e!=nil{return nil,e};if e:=binary.Read(r,binary.LittleEndian,&n);e!=nil{return nil,e};if n>20_000_000{return nil,errors.New("oversize GGUF array")};if elem==8{a:=make([]string,int(n));for i:=range a{s,e:=readString(r);if e!=nil{return nil,e};a[i]=s};return a,nil};a:=make([]any,int(n));for i:=range a{v,e:=readValue(r,elem);if e!=nil{return nil,e};a[i]=v};return a,nil;case 10:var v uint64;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 11:var v int64;e:=binary.Read(r,binary.LittleEndian,&v);return v,e;case 12:var v float64;e:=binary.Read(r,binary.LittleEndian,&v);return v,e};return nil,fmt.Errorf("unknown GGUF type %d",typ)}

func(t *Tokenizer)CountCompletion(prompt string)int{return t.count(prompt,t.addBOS,true)}
func(t *Tokenizer)CountChat(messages []ChatMessage,toolsJSON,toolChoiceJSON string,reasoning bool)int{return t.count(t.RenderChat(messages,toolsJSON,toolChoiceJSON,reasoning),false,true)}

func(t *Tokenizer)RenderChat(messages []ChatMessage,toolsJSON,toolChoiceJSON string,reasoning bool)string{
 _=toolChoiceJSON
 var b strings.Builder;b.WriteString(t.bos)
 hasTools:=toolsJSON!=""
 start:=0
 if hasTools{
  defs:=renderToolDefinitions(toolsJSON)
  b.WriteString("<|im_start|>system\n")
  if len(messages)>0&&messages[0].Role=="system"{if strings.Contains(messages[0].Content,"<tool_def_sep>"){b.WriteString(strings.ReplaceAll(messages[0].Content,"<tool_def_sep>",defs))}else{b.WriteString(messages[0].Content);b.WriteString("\n\n");b.WriteString(defs)};start=1}else{b.WriteString(defs)}
  b.WriteString("<|im_end|>\n")
 }else if len(messages)>0&&messages[0].Role=="system"{writeTurn(&b,"system",messages[0].Content);start=1}
 for i:=start;i<len(messages);i++{
  m:=messages[i]
  switch m.Role{
  case "user","system":writeTurn(&b,m.Role,m.Content)
  case "assistant":renderAssistant(&b,m)
  case "tool":
   b.WriteString("<|im_start|>user")
   for ;i<len(messages)&&messages[i].Role=="tool";i++{b.WriteString("\n<tool_response>\n");b.WriteString(messages[i].Content);b.WriteString("\n</tool_response>")}
   b.WriteString("<|im_end|>\n");i--
  }
 }
 b.WriteString("<|im_start|>assistant\n");if reasoning{b.WriteString("<think>\n")}else{b.WriteString("<think>\n\n</think>\n\n")}
 return b.String()
}
func writeTurn(b *strings.Builder,role,content string){b.WriteString("<|im_start|>");b.WriteString(role);b.WriteByte('\n');b.WriteString(content);b.WriteString("<|im_end|>\n")}
func renderToolDefinitions(raw string)string{var items []json.RawMessage;if json.Unmarshal([]byte(raw),&items)!=nil{return ""};var b strings.Builder;b.WriteString(toolInstructions);for _,item:=range items{var value any;if json.Unmarshal(item,&value)!=nil{return ""};var encoded bytes.Buffer;encoder:=json.NewEncoder(&encoded);encoder.SetEscapeHTML(false);if encoder.Encode(value)!=nil{return ""};b.WriteByte('\n');b.Write(bytes.TrimSpace(encoded.Bytes()))};b.WriteString(toolInstructionsSuffix);return b.String()}
type renderedToolCall struct{Function struct{Name string `json:"name"`;Arguments string `json:"arguments"`} `json:"function"`}
func renderAssistant(b *strings.Builder,m ChatMessage){
 var calls []renderedToolCall;if m.ToolCalls!=""{_ = json.Unmarshal([]byte(m.ToolCalls),&calls)}
 content:=m.Content;reasoning:=""
 if at:=strings.Index(content,"</think>");at>=0{before:=strings.TrimRight(content[:at],"\n");if open:=strings.LastIndex(before,"<think>");open>=0{reasoning=strings.TrimLeft(before[open+len("<think>"):],"\n")};content=strings.TrimLeft(content[at+len("</think>"):],"\n")}
 if len(calls)>0{parts:=strings.Split(content,"<tool_sep>");var processed strings.Builder;processed.WriteString(parts[0]);for i:=1;i<len(parts);i++{if i-1<len(calls){renderToolCall(&processed,calls[i-1].Function.Name,calls[i-1].Function.Arguments)};processed.WriteString(parts[i])};for i:=len(parts)-1;i<len(calls);i++{renderToolCall(&processed,calls[i].Function.Name,calls[i].Function.Arguments)};content=processed.String()}
 b.WriteString("<|im_start|>assistant\n")
 if reasoning!=""{b.WriteString("<think>\n");b.WriteString(strings.Trim(reasoning,"\n"));b.WriteString("\n</think>\n\n");b.WriteString(strings.TrimLeft(content,"\n"))}else if !strings.Contains(content,"<think>")&&!strings.Contains(content,"</think>"){b.WriteString("<think>\n\n</think>\n\n");b.WriteString(strings.TrimLeft(content,"\n"))}else{b.WriteString(content)}
 for i,c:=range calls{if (i==0&&content!="")||i>0{b.WriteByte('\n')};renderToolCall(b,c.Function.Name,c.Function.Arguments)}
 b.WriteString("<|im_end|>\n")
}
func renderToolCall(b *strings.Builder,name,args string){b.WriteString("<function name=\"");b.WriteString(name);b.WriteString("\">");var values map[string]json.RawMessage;if json.Unmarshal([]byte(args),&values)==nil{keys:=make([]string,0,len(values));for k:=range values{keys=append(keys,k)};sort.Strings(keys);for _,k:=range keys{b.WriteString("<param name=\"");b.WriteString(k);b.WriteString("\">");var s string;if json.Unmarshal(values[k],&s)==nil{if strings.ContainsAny(s,"<&\n"){b.WriteString("<![CDATA[");b.WriteString(s);b.WriteString("]]>")}else{b.WriteString(s)}}else{b.Write(bytes.TrimSpace(values[k]))};b.WriteString("</param>")}};b.WriteString("</function>")}

func(t *Tokenizer)count(text string,bos,parseSpecial bool)int{n:=0;if bos&&t.bos!=""{n++};for len(text)>0{if token:=t.partitionPrefix(text,parseSpecial);token!=""{n++;text=text[len(token):];continue};end:=t.nextPartition(text,parseSpecial);if end==0{end=1};for _,piece:=range pretokenizeMiniCPM5(text[:end]){n+=t.countBPE(t.byteEncode(piece))};text=text[end:]};return n}
func(t *Tokenizer)partitionPrefix(s string,parseSpecial bool)string{best:="";for _,token:=range t.userDefined{if len(token)>len(best)&&strings.HasPrefix(s,token){best=token}};if parseSpecial{for _,token:=range t.control{if len(token)>len(best)&&strings.HasPrefix(s,token){best=token}}};return best}
func(t *Tokenizer)nextPartition(s string,parseSpecial bool)int{end:=len(s);for _,token:=range t.userDefined{if i:=strings.Index(s,token);i>=0&&i<end{end=i}};if parseSpecial{for _,token:=range t.control{if i:=strings.Index(s,token);i>=0&&i<end{end=i}}};return end}
func(t *Tokenizer)byteEncode(s string)string{var b strings.Builder;for _,v:=range []byte(s){b.WriteRune(t.byteRunes[v])};return b.String()}
func gpt2ByteRunes()[256]rune{var out [256]rune;used:=[256]bool{};for b:=33;b<=126;b++{out[b]=rune(b);used[b]=true};for b:=161;b<=172;b++{out[b]=rune(b);used[b]=true};for b:=174;b<=255;b++{out[b]=rune(b);used[b]=true};extra:=0;for b:=0;b<256;b++{if !used[b]{out[b]=rune(256+extra);extra++}};return out}
func pretokenizeMiniCPM5(s string)[]string{r:=[]rune(s);out:=make([]string,0,len(r));for i:=0;i<len(r);{start:=i;if unicode.IsDigit(r[i]){for i<len(r)&&unicode.IsDigit(r[i])&&i-start<3{i++}}else if n:=contractionLen(r[i:]);n>0{i+=n}else if unicode.IsLetter(r[i]){for i<len(r)&&unicode.IsLetter(r[i]){i++}}else if r[i]!='\r'&&r[i]!='\n'&&!unicode.IsLetter(r[i])&&!unicode.IsDigit(r[i])&&i+1<len(r)&&unicode.IsLetter(r[i+1]){i+=2;for i<len(r)&&unicode.IsLetter(r[i]){i++}}else if punctuationStart(r,i){if r[i]==' '{i++};for i<len(r)&&!unicode.IsSpace(r[i])&&!unicode.IsLetter(r[i])&&!unicode.IsDigit(r[i]){i++};for i<len(r)&&(r[i]=='\r'||r[i]=='\n'){i++}}else if unicode.IsSpace(r[i]){for i<len(r)&&unicode.IsSpace(r[i]){i++}}else{i++};out=append(out,string(r[start:i]))};return out}
func contractionLen(r []rune)int{if len(r)<2||r[0]!='\''{return 0};lower:=strings.ToLower(string(r));for _,x:=range []string{"'re","'ve","'ll","'s","'t","'m","'d"}{if strings.HasPrefix(lower,x){return len([]rune(x))}};return 0}
func punctuationStart(r []rune,i int)bool{j:=i;if r[j]==' '{j++;if j>=len(r){return false}};return !unicode.IsSpace(r[j])&&!unicode.IsLetter(r[j])&&!unicode.IsDigit(r[j])}
func(t *Tokenizer)countBPE(s string)int{if _,ok:=t.vocab[s];ok{return 1};parts:=make([]string,0,len(s));for _,r:=range s{parts=append(parts,string(r))};for{best:=-1;rank:=int(^uint(0)>>1);for i:=0;i+1<len(parts);i++{if r,ok:=t.ranks[parts[i]+" "+parts[i+1]];ok&&r<rank{best=i;rank=r}};if best<0{break};parts[best]=parts[best]+parts[best+1];parts=append(parts[:best+1],parts[best+2:]...)};return len(parts)}
func(t *Tokenizer)ChatTemplate()string{return t.chatTemplate}
