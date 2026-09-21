package controllerhttp

import(
 "context"
 "encoding/json"
 "net/http"
 "strconv"
 "strings"
 "sync"
 "time"

 "github.com/google/uuid"
 g "mini-inference/services/controller/internal/contract/generated"
 "mini-inference/services/controller/internal/safestatus"
)
type Validator interface{Check(context.Context,int64,uuid.UUID)error;Healthy(context.Context)error}
type Runner interface{Run(context.Context,string)g.ControllerResult}
type Server struct{validator Validator;adapter Runner;model,sha string;mu sync.Mutex}
func New(v Validator,a Runner,model,sha string)*Server{return &Server{validator:v,adapter:a,model:model,sha:sha}}
func(s *Server)Handler()http.Handler{m:=http.NewServeMux();m.HandleFunc("/internal/v1/model/status",s.operation("status",http.MethodGet,5*time.Second));m.HandleFunc("/internal/v1/model/load",s.operation("load",http.MethodPost,10*time.Minute));m.HandleFunc("/internal/v1/model/unload",s.operation("unload",http.MethodPost,60*time.Second));m.HandleFunc("/health/live",func(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodGet{w.WriteHeader(405);return};w.WriteHeader(200)});m.HandleFunc("/health/ready",func(w http.ResponseWriter,r *http.Request){if r.Method!=http.MethodGet{w.WriteHeader(405);return};ctx,cancel:=context.WithTimeout(r.Context(),time.Second);defer cancel();if s.validator.Healthy(ctx)!=nil{w.WriteHeader(503);return};w.WriteHeader(200)});return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){h,p:=m.Handler(r);if p==""{w.WriteHeader(404);return};h.ServeHTTP(w,r)})}
func(s *Server)operation(op,method string,deadline time.Duration)http.HandlerFunc{return func(w http.ResponseWriter,r *http.Request){if r.Method!=method{safeErr(w,405,"invalid_request","Method not allowed.",false);return};if r.URL.RawQuery!=""||hasBody(r)||hasUnknownArgumentHeader(r){safeErr(w,400,"invalid_request","Request must not include body, query, or argument headers.",false);return};epoch,e:=strconv.ParseInt(r.Header.Get("X-Authority-Epoch"),10,64);holder,e2:=uuid.Parse(r.Header.Get("X-Authority-Holder"));if e!=nil||e2!=nil||epoch<1{safeErr(w,400,"invalid_request","Authority headers are required.",false);return};if s.validator.Check(r.Context(),epoch,holder)!=nil{safeErr(w,409,"state_mismatch","Authority is stale.",false);return};s.mu.Lock();defer s.mu.Unlock();if s.validator.Check(r.Context(),epoch,holder)!=nil{safeErr(w,409,"state_mismatch","Authority is stale.",false);return};ctx,cancel:=context.WithTimeout(r.Context(),deadline);defer cancel();res:=s.adapter.Run(ctx,op);if s.validator.Check(context.WithoutCancel(r.Context()),epoch,holder)!=nil{res=safestatus.Indeterminate(op,epoch,holder.String(),s.model,s.sha)}else{res.AuthorityEpoch=epoch;res.AuthorityHolder=holder.String()};writeJSON(w,200,res)}}
func hasBody(r *http.Request)bool{if r.ContentLength>0{return true};if r.Body==nil{return false};var b [1]byte;n,_:=r.Body.Read(b[:]);return n>0}
func hasUnknownArgumentHeader(r *http.Request)bool{for k:=range r.Header{lk:=strings.ToLower(k);if strings.HasPrefix(lk,"x-")&&lk!="x-request-id"&&lk!="x-authority-epoch"&&lk!="x-authority-holder"{return true}};return false}
func safeErr(w http.ResponseWriter,status int,code,message string,retry bool){writeJSON(w,status,g.ErrorEnvelope{Error:g.SafeError{Code:code,Message:message,Retryable:retry}})}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
