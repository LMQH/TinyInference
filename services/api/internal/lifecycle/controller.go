package lifecycle

import(
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strconv"
 "strings"
 "time"

 "github.com/google/uuid"
 g "mini-inference/services/api/internal/contract/generated"
)
type Controller struct{base *url.URL;http *http.Client}
func NewController(raw string)(*Controller,error){u,e:=url.Parse(raw);if e!=nil||u.String()!="http://controller:9090"{return nil,errors.New("invalid controller URL")};return &Controller{u,&http.Client{Transport:&http.Transport{Proxy:nil}}},nil}
func(c *Controller)Call(ctx context.Context,op string,epoch int64,holder uuid.UUID,requestID string)(g.ControllerResult,error){method:=http.MethodPost;timeout:=60*time.Second;if op=="status"{method=http.MethodGet;timeout=5*time.Second}else if op=="load"{timeout=10*time.Minute}else if op!="unload"{return g.ControllerResult{},errors.New("invalid controller operation")};u:=*c.base;u.Path="/internal/v1/model/"+op;ctx,cancel:=context.WithTimeout(ctx,timeout);defer cancel();req,e:=http.NewRequestWithContext(ctx,method,u.String(),nil);if e!=nil{return g.ControllerResult{},e};req.Header.Set("X-Request-ID",requestID);req.Header.Set("X-Authority-Epoch",strconv.FormatInt(epoch,10));req.Header.Set("X-Authority-Holder",holder.String());resp,e:=c.http.Do(req);if e!=nil{return g.ControllerResult{},e};defer resp.Body.Close();if resp.StatusCode!=200{io.Copy(io.Discard,io.LimitReader(resp.Body,64<<10));return g.ControllerResult{},fmt.Errorf("controller status %d",resp.StatusCode)};var out g.ControllerResult;d:=json.NewDecoder(io.LimitReader(resp.Body,1<<20));d.DisallowUnknownFields();if e=d.Decode(&out);e!=nil{return out,errors.New("invalid controller response")};if out.Operation!=op||out.AuthorityEpoch!=epoch||out.AuthorityHolder!=holder.String()||out.ModelRef!=g.InternalModelRef||out.SourceSHA256!=g.ModelSourceSHA256||out.Runner.Engine!="llama.cpp"||out.ObservedState == "loaded" && out.Runner.KeepAlive != -1||!validObserved(out.ObservedState)||!validOutcome(out.Outcome){return out,errors.New("controller identity mismatch")};return out,nil}
func validObserved(s string)bool{return s=="loaded"||s=="unloaded"||s=="unknown"}
func validOutcome(s string)bool{return s=="succeeded"||s=="failed"||s=="indeterminate"}
var _ = strings.Builder{}
