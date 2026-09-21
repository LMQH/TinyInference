package tokenizer

import (
 "reflect"
 "strings"
 "testing"
)

func TestRenderReadinessAndConversationFixtures(t *testing.T){
 tok:=&Tokenizer{bos:"<s>"}
 ready:=tok.RenderChat([]ChatMessage{{Role:"user",Content:"Mini-Inference readiness probe."}},"","",false)
 wantReady:="<s><|im_start|>user\nMini-Inference readiness probe.<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"
 if ready!=wantReady{t.Fatalf("disabled-readiness render = %q",ready)}
 conversation:=tok.RenderChat([]ChatMessage{{Role:"system",Content:"Be concise."},{Role:"user",Content:"Hello, world!"}},"","",true)
 wantConversation:="<s><|im_start|>system\nBe concise.<|im_end|>\n<|im_start|>user\nHello, world!<|im_end|>\n<|im_start|>assistant\n<think>\n"
 if conversation!=wantConversation{t.Fatalf("enabled-conversation render = %q",conversation)}
}

func TestRenderAssistantAndGroupedToolResponses(t *testing.T){
 tok:=&Tokenizer{bos:"<s>"}
 got:=tok.RenderChat([]ChatMessage{{Role:"assistant",Content:"Four."},{Role:"tool",Content:`{"temperature":21}`,ToolCallID:"call_1"},{Role:"tool",Content:`{"humidity":40}`,ToolCallID:"call_2"}},"","",false)
 want:="<s><|im_start|>assistant\n<think>\n\n</think>\n\nFour.<|im_end|>\n<|im_start|>user\n<tool_response>\n{\"temperature\":21}\n</tool_response>\n<tool_response>\n{\"humidity\":40}\n</tool_response><|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"
 if got!=want{t.Fatalf("tool response render = %q",got)}
}

func TestMiniCPM5PretokenizerAndByteEncoding(t *testing.T){
 got:=pretokenizeMiniCPM5("Hello, world! 1234\n")
 want:=[]string{"Hello",","," world","!"," ","123","4","\n"}
 if !reflect.DeepEqual(got,want){t.Fatalf("pieces = %#v, want %#v",got,want)}
 tok:=&Tokenizer{byteRunes:gpt2ByteRunes()}
 if got:=tok.byteEncode(" Hello");got!="ĠHello"{t.Fatalf("byte encoding = %q",got)}
}

func TestRenderToolDefinitionsUsesEmbeddedTemplateShape(t *testing.T){
 tok:=&Tokenizer{bos:"<s>"}
 tools:=`[{"type":"function","function":{"name":"weather","description":"Weather","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}}]`
 got:=tok.RenderChat([]ChatMessage{{Role:"user",Content:"Weather?"}},tools,`{"type":"function","function":{"name":"weather"}}`,false)
 for _,part:=range []string{"<s><|im_start|>system\n# Tools",`<tools>\n{"function":`,"Tool usage guidelines:","<|im_start|>user\nWeather?<|im_end|>","<|im_start|>assistant\n"}{if !strings.Contains(got,strings.ReplaceAll(part,`\n`,"\n")){t.Fatalf("render missing %q: %q",part,got)}}
}
func TestDisabledGenerationPromptExtractsExactlyTwoAddedTokens(t *testing.T){
 tok:=&Tokenizer{control:[]string{"<|im_start|>"},userDefined:[]string{"<think>","</think>"}}
 suffix:="<think>\n\n</think>\n\n";added:=0
 for len(suffix)>0{if token:=tok.partitionPrefix(suffix,true);token!=""{added++;suffix=suffix[len(token):]}else{suffix=suffix[1:]}}
 if added!=2{t.Fatalf("added-token count = %d",added)}
 if tok.partitionPrefix("<|im_start|>",false)!=""{t.Fatal("control token matched with parseSpecial disabled")}
 if tok.partitionPrefix("<think>",false)!="<think>"{t.Fatal("user-defined token was not extracted before BPE")}
}
