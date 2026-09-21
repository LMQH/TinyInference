package dmr

import("encoding/json";"testing")

func TestUsageTracksReasoningTokenFieldPresence(t *testing.T){
 var without Usage;if err:=json.Unmarshal([]byte(`{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}`),&without);err!=nil{t.Fatal(err)};if without.ReasoningPresent{t.Fatal("missing reasoning_tokens marked present")}
 var withZero Usage;if err:=json.Unmarshal([]byte(`{"prompt_tokens":1,"completion_tokens":2,"reasoning_tokens":0,"total_tokens":3}`),&withZero);err!=nil{t.Fatal(err)};if !withZero.ReasoningPresent{t.Fatal("explicit zero reasoning_tokens not marked present")}
}
