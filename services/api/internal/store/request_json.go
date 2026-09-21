package store
import "encoding/json"
func(r RequestRecord)MarshalJSON()([]byte,error){type wire RequestRecord;w:=wire(r);if r.Generation!=nil&&*r.Generation>0&&r.Output!=nil&&r.ReasoningTokens!=nil{v:=float64(*r.Output+*r.ReasoningTokens)/(float64(*r.Generation)/1000);w.Throughput=&v};return json.Marshal(w)}
