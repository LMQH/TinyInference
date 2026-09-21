package controllerhttp
import("context";"net/http";"net/http/httptest";"strings";"testing";"github.com/google/uuid";g "mini-inference/services/controller/internal/contract/generated")
type validAuthority struct{}
func(validAuthority)Check(context.Context,int64,uuid.UUID)error{return nil}
func(validAuthority)Healthy(context.Context)error{return nil}
type spyRunner struct{calls int}
func(s *spyRunner)Run(context.Context,string)g.ControllerResult{s.calls++;return g.ControllerResult{Outcome:"succeeded"}}
func TestRejectsInjectionBeforeExecution(t *testing.T){cases:=[]struct{method,path,body string;header bool}{{"POST","/internal/v1/model/load?model=other","",false},{"POST","/internal/v1/model/load","{}",false},{"GET","/internal/v1/model/load","",false},{"POST","/internal/v1/model/load","",true}};for _,tc:=range cases{spy:=&spyRunner{};h:=New(validAuthority{},spy,"model","sha").Handler();r:=httptest.NewRequest(tc.method,tc.path,strings.NewReader(tc.body));r.Header.Set("X-Authority-Epoch","1");r.Header.Set("X-Authority-Holder",uuid.NewString());if tc.header{r.Header.Set("X-Model","other")};w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code<400||spy.calls!=0{t.Fatalf("%s %s: status=%d calls=%d",tc.method,tc.path,w.Code,spy.calls)}}}
func TestAuthorityCheckedThreeTimes(t *testing.T){v:=&countAuthority{};spy:=&spyRunner{};h:=New(v,spy,"model","sha").Handler();r:=httptest.NewRequest("POST","/internal/v1/model/load",nil);r.Header.Set("X-Authority-Epoch","1");r.Header.Set("X-Authority-Holder",uuid.NewString());w:=httptest.NewRecorder();h.ServeHTTP(w,r);if v.n!=3||spy.calls!=1{t.Fatalf("checks=%d calls=%d",v.n,spy.calls)}}
type countAuthority struct{n int}
func(v *countAuthority)Check(context.Context,int64,uuid.UUID)error{v.n++;return nil}
func(v *countAuthority)Healthy(context.Context)error{return nil}
var _ = http.MethodGet
