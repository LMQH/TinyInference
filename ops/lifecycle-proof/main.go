package main

import (
 "bytes"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "os"
 "strings"
 "time"
)

type response struct {
 Operation string `json:"operation"`
 Outcome string `json:"outcome"`
 ObservedState string `json:"observed_state"`
 AuthorityEpoch int64 `json:"authority_epoch"`
 AuthorityHolder string `json:"authority_holder"`
 ModelRef string `json:"model_ref"`
 SourceSHA256 string `json:"source_sha256"`
 Runner struct { KeepAlive int `json:"keep_alive"` } `json:"runner"`
}

func request(client *http.Client, base, method, path, epoch, holder string, body io.Reader) (*http.Response, []byte, error) {
 req, err := http.NewRequest(method, base+path, body)
 if err != nil { return nil,nil,err }
 req.Header.Set("X-Request-ID", "00000000-0000-7000-8000-000000000001")
 req.Header.Set("X-Authority-Epoch", epoch)
 req.Header.Set("X-Authority-Holder", holder)
 resp, err := client.Do(req)
 if err != nil { return nil,nil,err }
 defer resp.Body.Close()
 data, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
 if err != nil { return nil,nil,err }
 if len(data)>65536 { return nil,nil,errors.New("controller response exceeded bound") }
 return resp,data,nil
}

func positive(client *http.Client, base, epoch, holder, model, sha string) error {
 steps:=[]struct{method,path,state string}{{"GET","/internal/v1/model/status",""},{"POST","/internal/v1/model/load","loaded"},{"GET","/internal/v1/model/status","loaded"},{"POST","/internal/v1/model/unload","unloaded"},{"GET","/internal/v1/model/status","unloaded"}}
 for _,step:=range steps {
  resp,data,err:=request(client,base,step.method,step.path,epoch,holder,nil); if err!=nil{return err}
  if resp.StatusCode!=http.StatusOK{return fmt.Errorf("fixed lifecycle operation returned status %d",resp.StatusCode)}
  var got response
  if err=json.Unmarshal(data,&got);err!=nil{return errors.New("invalid bounded controller JSON")}
  if got.Outcome!="succeeded" || got.AuthorityHolder!=holder || fmt.Sprint(got.AuthorityEpoch)!=epoch || got.ModelRef!=model || got.SourceSHA256!=sha || (got.ObservedState=="loaded" && got.Runner.KeepAlive != -1) { return errors.New("controller identity, authority, outcome, or loaded keep-alive mismatch") }
  if step.state!="" && got.ObservedState!=step.state{return errors.New("controller observed state mismatch")}
 }
 return nil
}

func negative(client *http.Client, base, epoch, holder string) error {
 cases:=[]struct{method,path string; body io.Reader}{{"GET","/internal/v1/model/status?model=other",nil},{"POST","/internal/v1/model/load",bytes.NewBufferString("{}")},{"POST","/internal/v1/model/unknown",nil},{"DELETE","/internal/v1/model/status",nil}}
 for _,test:=range cases {
  resp,_,err:=request(client,base,test.method,test.path,epoch,holder,test.body); if err!=nil{return err}
  if resp.StatusCode<400 || resp.StatusCode>499{return errors.New("negative lifecycle case did not fail closed")}
 }
 resp,_,err:=request(client,base,"GET","/internal/v1/model/status","0",holder,nil)
 if err!=nil{return err}; if resp.StatusCode<400||resp.StatusCode>499{return errors.New("stale authority was not rejected")}
 req,err:=http.NewRequest("GET",base+"/internal/v1/model/status",nil); if err!=nil{return err}
 req.Header.Set("X-Request-ID","00000000-0000-7000-8000-000000000002")
 req.Header.Set("X-Authority-Epoch",epoch); req.Header.Set("X-Authority-Holder",holder); req.Header.Set("X-Model-Override","other")
 extra,err:=client.Do(req); if err!=nil{return err}; extra.Body.Close()
 if extra.StatusCode<400||extra.StatusCode>499{return errors.New("argument-like header was not rejected")}
 return nil
}

func main(){
 if len(os.Args)!=2 || (os.Args[1]!="positive"&&os.Args[1]!="negative"){fmt.Fprintln(os.Stderr,"fixed action must be positive or negative");os.Exit(2)}
 base:=os.Getenv("CONTROLLER_BASE_URL"); epoch:=os.Getenv("AUTHORITY_EPOCH"); holder:=os.Getenv("AUTHORITY_HOLDER")
 if base!="http://controller:9090" || epoch=="" || holder=="" || strings.ContainsAny(epoch+holder,"\r\n"){fmt.Fprintln(os.Stderr,"invalid fixed proof configuration");os.Exit(1)}
 client:=&http.Client{Timeout:11*time.Minute}
 var err error
 if os.Args[1]=="positive" {err=positive(client,base,epoch,holder,os.Getenv("EXPECTED_MODEL_REF"),os.Getenv("EXPECTED_SOURCE_SHA256"))} else {err=negative(client,base,epoch,holder)}
 if err!=nil {fmt.Fprintln(os.Stderr,err);os.Exit(1)}
 fmt.Println("lifecycle proof phase passed")
}
