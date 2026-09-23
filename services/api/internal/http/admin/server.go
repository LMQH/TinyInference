package admin

import(
 "context"
 "encoding/base64"
 "encoding/json"
 "errors"
 "net/http"
 "strconv"
 "strings"
 "time"

 "github.com/google/uuid"
 "mini-inference/services/api/internal/adminstream"
 "mini-inference/services/api/internal/authority"
 g "mini-inference/services/api/internal/contract/generated"
 "mini-inference/services/api/internal/lifecycle"
 "mini-inference/services/api/internal/modelstate"
 "mini-inference/services/api/internal/queue"
 "mini-inference/services/api/internal/resources"
 "mini-inference/services/api/internal/safeerror"
 "mini-inference/services/api/internal/store"
)
type Server struct{store *store.Store;fence *authority.Fence;model *modelstate.Machine;queue *queue.Queue;lifecycle *lifecycle.Manager;resources *resources.Collector;events *adminstream.Handler}
func New(st *store.Store,f *authority.Fence,m *modelstate.Machine,q *queue.Queue,l *lifecycle.Manager,r *resources.Collector)*Server{return &Server{st,f,m,q,l,r,adminstream.New(st)}}
func(s *Server)Handler()http.Handler{
 m:=http.NewServeMux()
 m.HandleFunc("/admin/v1/snapshot",s.snapshot)
 m.HandleFunc("/admin/v1/requests",s.requests)
 m.HandleFunc("/admin/v1/metrics",s.metrics)
 m.HandleFunc("/admin/v1/alerts",s.alerts)
 m.HandleFunc("/admin/v1/logs",s.logs)
 m.HandleFunc("/admin/v1/operations",s.operations)
 m.Handle("/admin/v1/events",s.events)
 m.HandleFunc("/admin/v1/model/start",s.action(true))
 m.HandleFunc("/admin/v1/model/stop",s.action(false))
 m.HandleFunc("/admin/v1/model/name",s.modelName)
 m.HandleFunc("/admin/v1/queue/",s.cancel)
 m.HandleFunc("/health/live",func(w http.ResponseWriter,r *http.Request){w.WriteHeader(200)})
 m.HandleFunc("/health/ready",s.ready)
 m.HandleFunc("/metrics",s.prometheus)
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  h,p:=m.Handler(r)
  if p==""||strings.HasPrefix(r.URL.Path,"/internal/")||strings.HasPrefix(r.URL.Path,"/v1/"){s.err(w,404,"request_not_found");return}
  expected:="GET"
  if r.URL.Path=="/admin/v1/model/start"||r.URL.Path=="/admin/v1/model/stop"||r.URL.Path=="/admin/v1/model/name"||strings.HasPrefix(r.URL.Path,"/admin/v1/queue/"){expected="POST"}
  if r.Method!=expected{s.err(w,405,"method_not_allowed");return}
  h.ServeHTTP(w,r)
 })
}
func(s *Server)etag(ctx context.Context,w http.ResponseWriter,r *http.Request)(int64,bool){v,e:=s.store.SnapshotVersion(ctx);if e!=nil{s.err(w,503,"database_unavailable");return 0,false};tag:=`"`+strconv.FormatInt(v,10)+`"`;w.Header().Set("ETag",tag);if r.Header.Get("If-None-Match")==tag{w.WriteHeader(304);return v,false};return v,true}
func(s *Server)snapshot(w http.ResponseWriter,r *http.Request){if !getOnly(w,r){return};v,ok:=s.etag(r.Context(),w,r);if !ok{return};publicModelID,e:=s.store.PublicModelID(r.Context());if e!=nil{s.err(w,503,"database_unavailable");return};active,waiting,e:=s.store.QueueRecords(r.Context());if e!=nil{s.err(w,503,"database_unavailable");return};qs:=g.QueueSnapshot{Capacity:20,Depth:len(waiting),Waiting:make([]g.WaitingRequest,0,len(waiting))};if active!=nil&&active.Started!=nil{qs.Active=&g.ActiveRequest{ID:active.ID.String(),Status:"active",Endpoint:active.Endpoint,Stream:active.Stream,ReasoningEnabled:active.Reasoning,EnqueuedAt:active.Enqueued,StartedAt:*active.Started}};for i,x:=range waiting{qs.Waiting=append(qs.Waiting,g.WaitingRequest{ID:x.ID.String(),Status:"waiting",Endpoint:x.Endpoint,Stream:x.Stream,ReasoningEnabled:x.Reasoning,Position:i+1,EnqueuedAt:x.Enqueued,DeadlineAt:x.Enqueued.Add(30*time.Minute),CanCancel:true})};ms:=s.model.Snapshot();var op *string;if ms.OperationID!=""{x:=ms.OperationID;op=&x};var failure *g.SafeFailure;if ms.FailureCode!=""{failure=&g.SafeFailure{Code:ms.FailureCode,Message:"The model is unavailable.",Retryable:true}};ready:=s.fence.Alive()&&(ms.State==modelstate.Unloaded||ms.State==modelstate.Ready);reason:=(*string)(nil);if !ready{x:="service_not_ready";reason=&x};count,e:=s.store.ActiveAlertCount(r.Context());if e!=nil{s.err(w,503,"database_unavailable");return};resource:=s.resources.Sample(r.Context());writeJSON(w,200,g.AdminSnapshot{SnapshotVersion:v,GeneratedAt:time.Now().UTC(),Service:g.ServiceSnapshot{State:serviceState(ms.State,ready),Ready:ready,AuthorityEpoch:s.fence.Epoch(),ReasonCode:reason},Model:g.ModelSnapshot{State:string(ms.State),TransitionStartedAt:ms.TransitionStartedAt,OperationID:op,Failure:failure,PublicModelID:publicModelID,DefaultPublicModelID:g.PublicModelID},Queue:qs,Resources:resource,ActiveAlertCount:count})}
func(s *Server)requests(w http.ResponseWriter,r *http.Request){if !getOnly(w,r)||!onlyQuery(r,"cursor","limit"){if w.Header().Get("Content-Type")==""{s.err(w,400,"invalid_parameter")};return};_,ok:=s.etag(r.Context(),w,r);if !ok{return};limit,e:=store.ParseLimit(r.URL.Query().Get("limit"));if e!=nil{s.err(w,400,"invalid_parameter");return};var cursor *store.Cursor;if v:=r.URL.Query().Get("cursor");v!=""{x,e:=store.DecodeCursor(v);if e!=nil{s.err(w,400,"invalid_parameter");return};cursor=&x};items,next,e:=s.store.Requests(r.Context(),cursor,limit);if e!=nil{s.err(w,503,"database_unavailable");return};var n any=nil;if next!=nil{n=store.EncodeCursor(*next)};writeJSON(w,200,map[string]any{"items":items,"next_cursor":n})}
func(s *Server)metrics(w http.ResponseWriter,r *http.Request){if !getOnly(w,r)||!onlyQuery(r,"window","granularity"){s.err(w,400,"invalid_parameter");return};_,ok:=s.etag(r.Context(),w,r);if !ok{return};window,gran:=r.URL.Query().Get("window"),r.URL.Query().Get("granularity");dur,step,valid:=metricRange(window,gran);if !valid{s.err(w,400,"invalid_parameter");return};to:=time.Now().UTC();from:=to.Add(-dur);samples,e:=s.store.MetricSamples(r.Context(),from,to);if e!=nil{s.err(w,503,"database_unavailable");return};type bucket struct{BucketStart time.Time `json:"bucket_start"`;RequestCount int64 `json:"request_count"`;Outcomes map[string]int64 `json:"outcomes"`;InputTokens int64 `json:"input_tokens"`;OutputTokens int64 `json:"output_tokens"`;ReasoningTokens int64 `json:"reasoning_tokens"`;Throughput *float64 `json:"throughput_tokens_per_second"`;TTFT *float64 `json:"ttft_ms_avg"`;Duration *float64 `json:"duration_ms_avg"`;ttftN,durationN,generation int64}
 buckets:=map[int64]*bucket{};for _,x:=range samples{k:=x.Created.Unix()/int64(step.Seconds())*int64(step.Seconds());b:=buckets[k];if b==nil{outcomes:=map[string]int64{};for _,s:=range []string{"waiting","active","succeeded","failed","cancelled","queue_timeout","interrupted","rejected"}{outcomes[s]=0};b=&bucket{BucketStart:time.Unix(k,0).UTC(),Outcomes:outcomes};buckets[k]=b};b.RequestCount++;b.Outcomes[x.Status]++;if x.Input!=nil{b.InputTokens+=*x.Input};if x.Output!=nil{b.OutputTokens+=*x.Output};if x.Reasoning!=nil{b.ReasoningTokens+=*x.Reasoning};if x.TTFT!=nil{v:=float64(*x.TTFT);if b.TTFT==nil{b.TTFT=&v}else{*b.TTFT+=v};b.ttftN++};if x.Duration!=nil{v:=float64(*x.Duration);if b.Duration==nil{b.Duration=&v}else{*b.Duration+=v};b.durationN++};if x.Generation!=nil{b.generation+=*x.Generation}}
 series:=[]*bucket{};for at:=from.Truncate(step);at.Before(to);at=at.Add(step){b:=buckets[at.Unix()];if b==nil{continue};if b.ttftN>0{*b.TTFT/=float64(b.ttftN)};if b.durationN>0{*b.Duration/=float64(b.durationN)};if b.generation>0{v:=float64(b.OutputTokens+b.ReasoningTokens)/(float64(b.generation)/1000);b.Throughput=&v};series=append(series,b)};writeJSON(w,200,map[string]any{"from":from,"to":to,"granularity":gran,"series":series})}
func(s *Server)alerts(w http.ResponseWriter,r *http.Request){if !getOnly(w,r)||!onlyQuery(r,"cursor","limit"){s.err(w,400,"invalid_parameter");return};_,ok:=s.etag(r.Context(),w,r);if !ok{return};limit,e:=store.ParseLimit(r.URL.Query().Get("limit"));if e!=nil{s.err(w,400,"invalid_parameter");return};var c *store.Cursor;if v:=r.URL.Query().Get("cursor");v!=""{x,e:=store.DecodeCursor(v);if e!=nil{s.err(w,400,"invalid_parameter");return};c=&x};items,next,e:=s.store.Alerts(r.Context(),c,limit);if e!=nil{s.err(w,503,"database_unavailable");return};var n any;if next!=nil{n=store.EncodeCursor(*next)};writeJSON(w,200,map[string]any{"items":items,"next_cursor":n})}
func(s *Server)logs(w http.ResponseWriter,r *http.Request){if !getOnly(w,r)||!onlyQuery(r,"cursor","limit","level"){s.err(w,400,"invalid_parameter");return};_,ok:=s.etag(r.Context(),w,r);if !ok{return};level:=r.URL.Query().Get("level");if level!=""&&level!="info"&&level!="warning"&&level!="error"{s.err(w,400,"invalid_parameter");return};limit,e:=store.ParseLimit(r.URL.Query().Get("limit"));if e!=nil{s.err(w,400,"invalid_parameter");return};var c *int64;if v:=r.URL.Query().Get("cursor");v!=""{b,e:=base64.RawURLEncoding.DecodeString(v);if e!=nil{s.err(w,400,"invalid_parameter");return};x,e:=strconv.ParseInt(string(b),10,64);if e!=nil{s.err(w,400,"invalid_parameter");return};c=&x};items,next,e:=s.store.Logs(r.Context(),c,limit,level);if e!=nil{s.err(w,503,"database_unavailable");return};var n any;if next!=nil{n=base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(*next,10)))};writeJSON(w,200,map[string]any{"items":items,"next_cursor":n})}
func(s *Server)operations(w http.ResponseWriter,r *http.Request){
 if !getOnly(w,r)||!onlyQuery(r){s.err(w,400,"invalid_parameter");return}
 if _,ok:=s.etag(r.Context(),w,r);!ok{return}
 runs,e:=s.store.LatestRuns(r.Context());if e!=nil{s.err(w,503,"database_unavailable");return}
 ops,e:=s.store.ModelOperations(r.Context(),50);if e!=nil{s.err(w,503,"database_unavailable");return}
 outcome:=func(kind string)string{x,ok:=runs[kind];if !ok{return"unknown"};if x.Status=="succeeded"{return"succeeded"};return"failed"}
 rtn:=runs["retention"];backup:=runs["backup"];restore,hasRestore:=runs["restore_proof"]
 var totals any
 if hasRestore&&restore.Input!=nil&&restore.Output!=nil&&restore.Reasoning!=nil{totals=map[string]any{"input":*restore.Input,"output":*restore.Output,"reasoning":*restore.Reasoning}}
 writeJSON(w,200,map[string]any{
  "retention":map[string]any{"cutoff_at":rtn.Cutoff,"last_run_at":rtn.Completed,"outcome":outcome("retention"),"deleted_rows":rtn.Affected},
  "backup":map[string]any{"last_run_at":backup.Completed,"outcome":outcome("backup"),"artifact_name":backup.Artifact,"retained_count":backup.Affected,"next_expected_at":nextBackup(backup.Completed)},
  "restore_proof":map[string]any{"last_run_at":restore.Completed,"outcome":outcome("restore_proof"),"restored_request_count":restore.RequestCount,"restored_token_totals":totals},
  "model_operations":ops,
 })
}
func(s *Server)action(start bool)http.HandlerFunc{return func(w http.ResponseWriter,r *http.Request){if r.Method!="POST"{s.err(w,405,"method_not_allowed");return};if r.URL.RawQuery!=""||hasBody(r){s.err(w,400,"invalid_parameter");return};now:=time.Now().UTC();var id uuid.UUID;var e error;if start{id,e=s.lifecycle.Start(r.Context())}else{id,e=s.lifecycle.Stop(r.Context())};if e!=nil{if errors.Is(e,lifecycle.ErrOperationInProgress){s.err(w,409,"operation_in_progress")}else if errors.Is(e,context.Canceled)||errors.Is(e,context.DeadlineExceeded){s.err(w,503,"authority_unavailable")}else{s.err(w,503,"database_unavailable")};return};writeJSON(w,202,map[string]any{"operation_id":id,"status":"running","accepted_at":now})}}
func(s *Server)cancel(w http.ResponseWriter,r *http.Request){
 if r.Method!="POST"{s.err(w,405,"method_not_allowed");return}
 if r.URL.RawQuery!=""||hasBody(r){s.err(w,400,"invalid_parameter");return}
 prefix,suffix:="/admin/v1/queue/","/cancel";p:=r.URL.Path
 if len(p)<=len(prefix)+len(suffix)||p[len(p)-len(suffix):]!=suffix{s.err(w,404,"request_not_found");return}
 idText:=p[len(prefix):len(p)-len(suffix)];id,e:=uuid.Parse(idText);if e!=nil{s.err(w,400,"invalid_parameter");return}
 now:=time.Now().UTC();h:=int16(409)
 e=s.queue.CancelWith(idText,queue.OperatorCancelled,func(*queue.Entry)error{return s.store.Transition(r.Context(),store.Transition{Holder:s.fence.Holder(),Epoch:s.fence.Epoch(),ID:id,From:"waiting",To:"cancelled",Reason:"operator_cancelled",HTTPStatus:&h,Completed:&now})})
 if e!=nil{if errors.Is(e,queue.ErrNotWaiting){s.err(w,409,"request_not_waiting")}else{s.degradePersistence();s.err(w,503,"database_unavailable")};return}
 version,e:=s.store.SnapshotVersion(r.Context());if e!=nil{s.err(w,503,"database_unavailable");return}
 writeJSON(w,200,map[string]any{"request_id":id,"status":"cancelled","cancelled_at":now,"snapshot_version":version})
}
func(s *Server)degradePersistence(){now:=time.Now().UTC();_ = s.model.Transition(modelstate.Unavailable,now,"","metadata_persistence_failed");s.queue.Drain(queue.Interrupted)}
func(s *Server)ready(w http.ResponseWriter,r *http.Request){if r.Method!="GET"{w.WriteHeader(405);return};ctx,cancel:=context.WithTimeout(r.Context(),time.Second);defer cancel();ms:=s.model.Snapshot().State;staleLoaded:=ms==modelstate.Ready&&(s.lifecycle.LoadedObserved().IsZero()||time.Since(s.lifecycle.LoadedObserved())>10*time.Second);if !s.fence.Alive()||s.store.Health(ctx)!=nil||staleLoaded||(ms!=modelstate.Unloaded&&ms!=modelstate.Ready){w.WriteHeader(503);return};w.WriteHeader(200)}
func(s *Server)prometheus(w http.ResponseWriter,r *http.Request){if r.Method!="GET"{w.WriteHeader(405);return};w.Header().Set("Content-Type","text/plain; version=0.0.4");snap:=s.queue.Snapshot();write:=func(name string,v int){_,_=w.Write([]byte(name+" "+strconv.Itoa(v)+"\n"))};write("mini_inference_queue_depth",len(snap.Waiting));if snap.ActiveID!=""{write("mini_inference_active_requests",1)}else{write("mini_inference_active_requests",0)}}
func(s *Server)err(w http.ResponseWriter,status int,code string){typ:="invalid_request_error";retry:=false;if status==503{typ="service_unavailable_error";retry=true}else if status==409{typ="conflict_error"};safeerror.Write(w,uuid.NewString(),safeerror.New(status,typ,code,"The administrative request could not be completed.",nil,retry,nil))}
func getOnly(w http.ResponseWriter,r *http.Request)bool{if r.Method!="GET"{w.WriteHeader(405);return false};return true}
func onlyQuery(r *http.Request,names ...string)bool{allowed:=map[string]bool{};for _,n:=range names{allowed[n]=true};for k,v:=range r.URL.Query(){if !allowed[k]||len(v)!=1{return false}};return true}
func hasBody(r *http.Request)bool{return r.ContentLength!=0||len(r.TransferEncoding)>0}
func serviceState(s modelstate.State,ready bool)string{if s==modelstate.Starting{return"starting"};if s==modelstate.Stopping{return"stopping"};if ready{return"ready"};return"degraded"}
func metricRange(w,g string)(time.Duration,time.Duration,bool){switch{case w=="1h"&&g=="1m":return time.Hour,time.Minute,true;case w=="24h"&&g=="1h":return 24*time.Hour,time.Hour,true;case w=="7d"&&g=="1h":return 7*24*time.Hour,time.Hour,true;case w=="30d"&&g=="1d":return 30*24*time.Hour,24*time.Hour,true};return 0,0,false}
func nextBackup(last *time.Time)*time.Time{if last==nil{return nil};x:=last.Add(24*time.Hour);return &x}
func zero(v *int64)int64{if v==nil{return 0};return *v}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
