package public

import(
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "strconv"
 "time"

 "github.com/google/uuid"
 "mini-inference/services/api/internal/auth"
 "mini-inference/services/api/internal/authority"
 g "mini-inference/services/api/internal/contract/generated"
 "mini-inference/services/api/internal/dmr"
 "mini-inference/services/api/internal/modelstate"
 "mini-inference/services/api/internal/queue"
 "mini-inference/services/api/internal/safeerror"
 "mini-inference/services/api/internal/store"
 "mini-inference/services/api/internal/stream"
 "mini-inference/services/api/internal/tokenizer"
)
type Repository interface{Admit(context.Context,store.Admit)error;Transition(context.Context,store.Transition)error}
type Server struct{key string;fence *authority.Fence;repo Repository;q *queue.Queue;model *modelstate.Machine;loadedObserved func()time.Time;dmr *dmr.Client;tokenizer *tokenizer.Tokenizer}
func New(key string,f *authority.Fence,r Repository,q *queue.Queue,m *modelstate.Machine,observed func()time.Time,d *dmr.Client,t *tokenizer.Tokenizer)*Server{return &Server{key:key,fence:f,repo:r,q:q,model:m,loadedObserved:observed,dmr:d,tokenizer:t}}
func(s *Server)Handler()http.Handler{m:=http.NewServeMux();m.HandleFunc("/v1/models",s.models);m.HandleFunc("/v1/chat/completions",s.completion(true));m.HandleFunc("/v1/completions",s.completion(false));return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){id:=newID();w.Header().Set("X-Request-ID",id);h,p:=m.Handler(r);if p==""{w.WriteHeader(404);return};if !auth.Valid(r,s.key){safeerror.Write(w,id,safeerror.New(401,"authentication_error","invalid_api_key","Invalid API key.",nil,false,nil));return};r.Header.Set("X-Internal-Request-ID",id);h.ServeHTTP(w,r)})}
func(s *Server)models(w http.ResponseWriter,r *http.Request){id:=r.Header.Get("X-Internal-Request-ID");if r.Method!=http.MethodGet{safeerror.Write(w,id,safeerror.New(405,"invalid_request_error","method_not_allowed","Method not allowed.",nil,false,nil));return};if r.URL.RawQuery!=""||r.ContentLength>0{safeerror.Write(w,id,safeerror.Invalid("invalid_parameter",nil));return};writeJSON(w,200,map[string]any{"object":"list","data":[]any{map[string]any{"id":g.PublicModelID,"object":"model","created":0,"owned_by":"openbmb"}}})}
func(s *Server)completion(chat bool)http.HandlerFunc{
 return func(w http.ResponseWriter,r *http.Request){
  idText:=r.Header.Get("X-Internal-Request-ID")
  if r.Method!=http.MethodPost{safeerror.Write(w,idText,safeerror.New(405,"invalid_request_error","method_not_allowed","Method not allowed.",nil,false,nil));return}
  parsed,se:=parseRequest(r,chat)
  if se!=nil{safeerror.Write(w,idText,se);return}
  applyChatTemplateMode(&parsed)
  applyStreamUsage(&parsed)
  id,e:=uuid.Parse(idText)
  if e!=nil{safeerror.Write(w,idText,safeerror.New(500,"internal_error","internal_error","Internal error.",nil,false,nil));return}
  input,max:=s.inputTokens(parsed),parsedMax(parsed)
  if input+max>131072{p:="max_tokens";safeerror.Write(w,idText,safeerror.New(400,"invalid_request_error","context_length_exceeded","The requested context exceeds 131072 tokens.",&p,false,nil));return}
  if !s.fence.Alive(){after:=5;safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","authority_unavailable","Inference authority is unavailable.",nil,true,&after));return}
  if !s.model.Admissible(time.Now(),s.loadedObserved()){after:=5;safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","model_unavailable","The model is unavailable.",nil,true,&after));return}
  endpoint,wireEndpoint:="completions","completions"
  isStream,reasoning:=false,true
  if chat{endpoint,wireEndpoint="chat/completions","chat.completions";isStream=parsed.Chat.Stream;reasoning=parsed.Chat.Reasoning}else{isStream=parsed.Completion.Stream;reasoning=parsed.Completion.Reasoning}
  requestCtx,requestCancel:=context.WithCancel(r.Context())
  defer requestCancel()
  now:=time.Now().UTC()
  entry,active,e:=s.q.AdmitDurable(requestCtx,idText,now,func(qe *queue.Entry,a bool)error{
   status:="waiting"
   var started *time.Time
   if a{status="active";x:=now;started=&x}
   return s.repo.Admit(requestCtx,store.Admit{Holder:s.fence.Holder(),Epoch:s.fence.Epoch(),ID:id,Endpoint:wireEndpoint,Model:g.PublicModelID,Stream:isStream,Reasoning:reasoning,Status:status,Arrival:int64(qe.ArrivalSeq),Created:now,Enqueued:now,Started:started})
  },func(*queue.Entry)error{return s.finishWaiting(context.Background(),id,"queue_timeout",504)},func(*queue.Entry)error{return s.finishWaiting(context.Background(),id,"request_cancelled",409)})
  if errors.Is(e,queue.ErrFull){after:=1;safeerror.Write(w,idText,safeerror.New(429,"rate_limit_error","queue_full","The inference queue is full.",nil,true,&after));return}
  if e!=nil{after:=5;safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","database_unavailable","Metadata storage is unavailable.",nil,true,&after));return}
  if !active{
   result:=entry.Wait(requestCtx)
   switch result{
   case queue.Promoted:
   case queue.TimedOut:safeerror.Write(w,idText,safeerror.New(504,"timeout_error","queue_timeout","The queue wait timed out.",nil,true,nil));return
   case queue.Cancelled:
    now:=time.Now().UTC();h:=int16(409)
    if e=s.q.CancelWith(idText,queue.Cancelled,func(*queue.Entry)error{return s.repo.Transition(context.WithoutCancel(requestCtx),store.Transition{Holder:s.fence.Holder(),Epoch:s.fence.Epoch(),ID:id,From:"waiting",To:"cancelled",Reason:"request_cancelled",HTTPStatus:&h,Completed:&now})});e!=nil{s.degradePersistence();safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","database_unavailable","Metadata storage is unavailable.",nil,true,nil));return}
    safeerror.Write(w,idText,safeerror.New(409,"conflict_error","request_cancelled","Request cancelled.",nil,false,nil));return
   case queue.CancelledPersisted:safeerror.Write(w,idText,safeerror.New(409,"conflict_error","request_cancelled","Request cancelled.",nil,false,nil));return
   case queue.OperatorCancelled:safeerror.Write(w,idText,safeerror.New(409,"conflict_error","operator_cancelled","Request was cancelled by an operator.",nil,false,nil));return
   case queue.Stopped:safeerror.Write(w,idText,safeerror.New(409,"conflict_error","stopped","Model stopped.",nil,false,nil));return
   case queue.PersistenceFailed:s.degradePersistence();safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","database_unavailable","Metadata storage is unavailable.",nil,true,nil));return
   case queue.Interrupted:
    code,message:="service_shutdown","Service is shutting down."
    if s.model.Snapshot().State==modelstate.Unavailable{code,message="model_unavailable","Model is unavailable."}
    safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error",code,message,nil,true,nil));return
   default:safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","authority_unavailable","Inference authority is unavailable.",nil,true,nil));return
   }
  }
  started:=time.Now().UTC()
  defer s.promoteNext()
  activeCtx,activeCancel:=context.WithTimeout(requestCtx,30*time.Minute)
  defer activeCancel()
  if !s.q.SetActiveCancel(idText,activeCancel){s.finishActive(context.WithoutCancel(requestCtx),id,"interrupted","authority_unavailable",503,started,nil,nil,false);safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","authority_unavailable","Inference authority is unavailable.",nil,true,nil));return}
  resp,e:=s.dmr.Do(activeCtx,endpoint,parsed.Payload)
  if e!=nil{
   state:=s.model.Snapshot().State
   if state==modelstate.Stopping{safeerror.Write(w,idText,safeerror.New(409,"conflict_error","stopped","Model stopped.",nil,false,nil));return}
   if errors.Is(activeCtx.Err(),context.Canceled){
    if requestCtx.Err()==nil{
     code,message:="service_shutdown","Service is shutting down."
     if state==modelstate.Unavailable{code,message="model_unavailable","Model is unavailable."}
     safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error",code,message,nil,true,nil));return
    }
    s.finishActive(context.WithoutCancel(requestCtx),id,"cancelled","request_cancelled",409,started,nil,nil,false)
    safeerror.Write(w,idText,safeerror.New(409,"conflict_error","request_cancelled","Request cancelled.",nil,false,nil));return
   }
   s.finishActive(context.WithoutCancel(requestCtx),id,"failed","upstream_failure",502,started,nil,nil,false)
   safeerror.Write(w,idText,safeerror.New(502,"upstream_error","upstream_failure","Inference runtime failed.",nil,true,nil))
   return
  }
  defer resp.Body.Close()
  if isStream{s.streamResponse(w,r,id,idText,resp,started,reasoning,chat);return}
  s.nonStreamResponse(w,r,id,idText,resp,started,chat,reasoning)
 }
}
func(s *Server)inputTokens(p Parsed)int{if p.Chat!=nil{ms:=make([]tokenizer.ChatMessage,0,len(p.Chat.Messages));for _,m:=range p.Chat.Messages{content:="";if m.Content!=nil{content=*m.Content};calls:="";if len(m.ToolCalls)>0{b,_:=json.Marshal(m.ToolCalls);calls=string(b)};ms=append(ms,tokenizer.ChatMessage{Role:m.Role,Content:content,ToolCalls:calls,ToolCallID:m.ToolCallID})};tools:="";if len(p.Chat.Tools)>0{b,_:=json.Marshal(p.Chat.Tools);tools=string(b)};choice:="";if p.Chat.ToolChoice!=nil{b,_:=json.Marshal(p.Chat.ToolChoice);choice=string(b)};return s.tokenizer.CountChat(ms,tools,choice,p.Chat.Reasoning)};return s.tokenizer.CountCompletion(p.Completion.Prompt)}
func parsedMax(p Parsed)int{if p.Chat!=nil{return p.Chat.MaxTokens};return p.Completion.MaxTokens}
func(s *Server)nonStreamResponse(w http.ResponseWriter,r *http.Request,id uuid.UUID,idText string,resp *http.Response,started time.Time,chat,reasoning bool){
 collected,e:=stream.Collect(resp.Body,g.PublicModelID,reasoning,chat)
 if e!=nil{if persistErr:=s.finishActive(context.WithoutCancel(r.Context()),id,"failed","upstream_protocol_error",502,started,collected.Summary.FirstToken,nil,false);persistErr!=nil{s.degradePersistence();safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","database_unavailable","Metadata storage is unavailable.",nil,true,nil));return};safeerror.Write(w,idText,safeerror.New(502,"upstream_error","upstream_protocol_error","Inference runtime returned an invalid response.",nil,false,nil));return}
 usage:=dmr.Usage{PromptTokens:collected.Summary.Usage.PromptTokens,CompletionTokens:collected.Summary.Usage.CompletionTokens,ReasoningTokens:collected.Summary.Usage.ReasoningTokens,TotalTokens:collected.Summary.Usage.TotalTokens,ReasoningPresent:true}
 if e=s.finishActive(context.WithoutCancel(r.Context()),id,"succeeded","",200,started,collected.Summary.FirstToken,&usage,collected.Summary.ToolCalls);e!=nil{s.degradePersistence();safeerror.Write(w,idText,safeerror.New(503,"service_unavailable_error","database_unavailable","Metadata storage is unavailable.",nil,true,nil));return}
 prefix,object:="cmpl_","text_completion"
 if chat{prefix,object="chatcmpl_","chat.completion"}
 writeJSON(w,200,map[string]any{"id":prefix+idText,"object":object,"created":started.Unix(),"model":g.PublicModelID,"choices":collected.Choices,"usage":collected.Summary.Usage})
}
func(s *Server)streamResponse(w http.ResponseWriter,r *http.Request,id uuid.UUID,idText string,resp *http.Response,started time.Time,reasoning,chat bool){
 summary,e:=stream.Proxy(w,resp.Body,g.PublicModelID,reasoning,chat)
 if e!=nil{
  status,code,httpStatus:="failed","upstream_protocol_error",502
  if r.Context().Err()!=nil{status,code,httpStatus="cancelled","request_cancelled",409}
  if persistErr:=s.finishActive(context.WithoutCancel(r.Context()),id,status,code,httpStatus,started,summary.FirstToken,nil,false);persistErr!=nil{s.degradePersistence();stream.Error(w,g.ErrorEnvelope{Error:g.ErrorDetail{Message:"Metadata storage is unavailable.",Type:"service_unavailable_error",Code:"database_unavailable",RequestID:idText,Retryable:true}});return}
  stream.Error(w,g.ErrorEnvelope{Error:g.ErrorDetail{Message:"Inference stream failed.",Type:"upstream_error",Code:code,RequestID:idText}})
  return
 }
 usage:=dmr.Usage{PromptTokens:summary.Usage.PromptTokens,CompletionTokens:summary.Usage.CompletionTokens,ReasoningTokens:summary.Usage.ReasoningTokens,TotalTokens:summary.Usage.TotalTokens,ReasoningPresent:true}
 if e=s.finishActive(context.WithoutCancel(r.Context()),id,"succeeded","",200,started,summary.FirstToken,&usage,summary.ToolCalls);e!=nil{s.degradePersistence();stream.Error(w,g.ErrorEnvelope{Error:g.ErrorDetail{Message:"Metadata storage is unavailable.",Type:"service_unavailable_error",Code:"database_unavailable",RequestID:idText,Retryable:true}});return}
 _ = stream.Done(w)
}
func(s *Server)finishWaiting(ctx context.Context,id uuid.UUID,code string,httpStatus int)error{now:=time.Now().UTC();status:="cancelled";if code=="queue_timeout"{status="queue_timeout"}else if code=="service_shutdown"{status="interrupted"};h:=int16(httpStatus);return s.repo.Transition(ctx,store.Transition{Holder:s.fence.Holder(),Epoch:s.fence.Epoch(),ID:id,From:"waiting",To:status,Reason:code,HTTPStatus:&h,Completed:&now})}
func(s *Server)finishActive(ctx context.Context,id uuid.UUID,status,code string,httpStatus int,started time.Time,first *time.Time,u *dmr.Usage,tools bool)error{
 now:=time.Now().UTC();h:=int16(httpStatus);duration:=now.Sub(started).Milliseconds()
 var input,output,reasoning,ttft,generation *int64
 if u!=nil{a,b,c:=u.PromptTokens,u.CompletionTokens-u.ReasoningTokens,u.ReasoningTokens;input,output,reasoning=&a,&b,&c}
 if first!=nil{x:=first.Sub(started).Milliseconds();if x<0{x=0};ttft=&x;y:=now.Sub(*first).Milliseconds();if y<0{y=0};generation=&y}
 return s.repo.Transition(ctx,store.Transition{Holder:s.fence.Holder(),Epoch:s.fence.Epoch(),ID:id,From:"active",To:status,Reason:code,HTTPStatus:&h,FirstToken:first,Completed:&now,Input:input,Output:output,Reasoning:reasoning,TTFT:ttft,Duration:&duration,Generation:generation,ToolCalls:tools})
}
func(s *Server)degradePersistence(){now:=time.Now().UTC();_ = s.model.Transition(modelstate.Unavailable,now,"","metadata_persistence_failed");s.q.Drain(queue.Interrupted)}
func(s *Server)promoteNext(){now:=time.Now().UTC();if _,e:=s.q.CompleteActiveWith(now,func(e *queue.Entry)error{id,err:=uuid.Parse(e.ID);if err!=nil{return err};started:=now;wait:=now.Sub(e.EnqueuedAt).Milliseconds();return s.repo.Transition(context.Background(),store.Transition{Holder:s.fence.Holder(),Epoch:s.fence.Epoch(),ID:id,From:"waiting",To:"active",Started:&started,QueueWait:&wait})});e!=nil{s.degradePersistence()}}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func newID()string{id,e:=uuid.NewV7();if e!=nil{return uuid.NewString()};return id.String()}
var _ = strconv.Itoa
