package lifecycle

import(
 "context"
 "errors"
 "io"
 "sync"
 "time"

 "github.com/google/uuid"
 "mini-inference/services/api/internal/authority"
 "mini-inference/services/api/internal/dmr"
 "mini-inference/services/api/internal/modelstate"
 "mini-inference/services/api/internal/queue"
 "mini-inference/services/api/internal/store"
 "mini-inference/services/api/internal/tokenizer"
 "mini-inference/services/api/internal/stream"
)
var ErrOperationInProgress=errors.New("operation_in_progress")
type operationStore interface{BeginOperation(context.Context,uuid.UUID,int64,uuid.UUID,string,time.Time)error;FinishOperation(context.Context,uuid.UUID,int64,uuid.UUID,string,string,string)error;Transition(context.Context,store.Transition)error}
type Manager struct{fence *authority.Fence;store operationStore;controller *Controller;dmr *dmr.Client;tokenizer *tokenizer.Tokenizer;model *modelstate.Machine;queue *queue.Queue;opMu sync.Mutex;mu sync.RWMutex;loadedAt time.Time}
func NewManager(f *authority.Fence,s operationStore,c *Controller,d *dmr.Client,t *tokenizer.Tokenizer,m *modelstate.Machine,q *queue.Queue)*Manager{return &Manager{fence:f,store:s,controller:c,dmr:d,tokenizer:t,model:m,queue:q}}
func(m *Manager)LoadedObserved()time.Time{m.mu.RLock();defer m.mu.RUnlock();return m.loadedAt}
func(m *Manager)setLoaded(t time.Time){m.mu.Lock();m.loadedAt=t;m.mu.Unlock()}
func(m *Manager)ReconcileStartup(ctx context.Context)error{status,e:=m.controller.Call(ctx,"status",m.fence.Epoch(),m.fence.Holder(),uuid.NewString());if e!=nil{return e};if status.ObservedState!="unloaded"{unload,e:=m.controller.Call(ctx,"unload",m.fence.Epoch(),m.fence.Holder(),uuid.NewString());if e!=nil||unload.Outcome!="succeeded"||unload.ObservedState!="unloaded"{return errors.New("startup unload could not be proven")}};return nil}
func(m *Manager)Start(ctx context.Context)(uuid.UUID,error){m.opMu.Lock();defer m.opMu.Unlock();s:=m.model.Snapshot().State;if s!=modelstate.Unloaded&&s!=modelstate.Unavailable{return uuid.Nil,ErrOperationInProgress};id,_:=uuid.NewV7();now:=time.Now().UTC();if e:=m.store.BeginOperation(ctx,m.fence.Holder(),m.fence.Epoch(),id,"start",now);e!=nil{return uuid.Nil,e};if e:=m.model.Transition(modelstate.Starting,now,id.String(),"");e!=nil{return uuid.Nil,e};go m.runStart(id);return id,nil}
func(m *Manager)runStart(id uuid.UUID){ctx:=context.Background();result,e:=m.controller.Call(ctx,"load",m.fence.Epoch(),m.fence.Holder(),id.String());code:="controller_failure";if e==nil&&result.Outcome=="succeeded"&&result.ObservedState=="loaded"&&result.Runner.KeepAlive==-1{payload:=map[string]any{"model":m.dmr.Model(),"messages":[]any{map[string]any{"role":"user","content":"Mini-Inference readiness probe."}},"stream":true,"stream_options":map[string]any{"include_usage":true},"max_tokens":1,"temperature":0.8,"reasoning_budget":0,"chat_template_kwargs":map[string]any{"enable_thinking":false}};resp,x:=m.dmr.Do(ctx,"chat/completions",payload);if x==nil{decoded,x2:=stream.Collect(resp.Body,m.dmr.Model(),false,true);io.Copy(io.Discard,resp.Body);resp.Body.Close();expected:=m.tokenizer.CountChat([]tokenizer.ChatMessage{{Role:"user",Content:"Mini-Inference readiness probe."}},"","",false);if x2==nil&&decoded.Summary.Usage.PromptTokens==int64(expected){now:=time.Now().UTC();m.setLoaded(now);_ = m.model.Transition(modelstate.Ready,now,id.String(),"");_ = m.store.FinishOperation(ctx,m.fence.Holder(),m.fence.Epoch(),id,"succeeded","","loaded");return};code="tokenizer_mismatch"}else{code="warm_probe_failed"}};now:=time.Now().UTC();_ = m.model.Transition(modelstate.Unavailable,now,id.String(),code);_ = m.store.FinishOperation(ctx,m.fence.Holder(),m.fence.Epoch(),id,"failed",code,"unknown")}
func(m *Manager)Stop(ctx context.Context)(uuid.UUID,error){m.opMu.Lock();defer m.opMu.Unlock();s:=m.model.Snapshot().State;if s!=modelstate.Ready&&s!=modelstate.Unavailable{return uuid.Nil,ErrOperationInProgress};id,_:=uuid.NewV7();now:=time.Now().UTC();if e:=m.store.BeginOperation(ctx,m.fence.Holder(),m.fence.Epoch(),id,"stop",now);e!=nil{return uuid.Nil,e};if e:=m.model.Transition(modelstate.Stopping,now,id.String(),"");e!=nil{return uuid.Nil,e};_,_,persistErr:=m.queue.DrainWith(queue.Stopped,func(e *queue.Entry,active bool)error{from:="waiting";if active{from="active"};return m.finishEntry(ctx,e,from,"cancelled","model_stop",409)});if persistErr!=nil{m.degradePersistence();go m.runStop(id);return id,persistErr};go m.runStop(id);return id,nil}
func(m *Manager)runStop(id uuid.UUID){ctx:=context.Background();r,e:=m.controller.Call(ctx,"unload",m.fence.Epoch(),m.fence.Holder(),id.String());now:=time.Now().UTC();if e==nil&&r.Outcome=="succeeded"&&r.ObservedState=="unloaded"{m.setLoaded(time.Time{});_ = m.model.Transition(modelstate.Unloaded,now,id.String(),"");_ = m.store.FinishOperation(ctx,m.fence.Holder(),m.fence.Epoch(),id,"succeeded","","unloaded");return};_ = m.model.Transition(modelstate.Unavailable,now,id.String(),"controller_failure");_ = m.store.FinishOperation(ctx,m.fence.Holder(),m.fence.Epoch(),id,"indeterminate","controller_failure","unknown")}
func(m *Manager)Shutdown(ctx context.Context)error{
 _,_,persistErr:=m.queue.DrainWith(queue.Interrupted,func(e *queue.Entry,active bool)error{from:="waiting";if active{from="active"};return m.finishEntry(ctx,e,from,"interrupted","service_shutdown",503)})
 if persistErr!=nil{m.degradePersistence()}
 r,e:=m.controller.Call(ctx,"unload",m.fence.Epoch(),m.fence.Holder(),uuid.NewString())
 if e!=nil{return e}
 if r.Outcome!="succeeded"||r.ObservedState!="unloaded"{return errors.New("shutdown unload could not be proven")}
 return persistErr
}
func(m *Manager)finishEntry(ctx context.Context,e *queue.Entry,from,to,code string,httpStatus int)error{id,x:=uuid.Parse(e.ID);if x!=nil{return x};now:=time.Now().UTC();h:=int16(httpStatus);return m.store.Transition(ctx,store.Transition{Holder:m.fence.Holder(),Epoch:m.fence.Epoch(),ID:id,From:from,To:to,Reason:code,HTTPStatus:&h,Completed:&now})}
func(m *Manager)degradePersistence(){now:=time.Now().UTC();_ = m.model.Transition(modelstate.Unavailable,now,"","metadata_persistence_failed");m.queue.Drain(queue.Interrupted)}
func(m *Manager)Poll(ctx context.Context){ticker:=time.NewTicker(5*time.Second);defer ticker.Stop();for{select{case<-ctx.Done():return;case<-ticker.C:if m.model.Snapshot().State!=modelstate.Ready{continue};r,e:=m.controller.Call(ctx,"status",m.fence.Epoch(),m.fence.Holder(),uuid.NewString());if e!=nil||r.Outcome!="succeeded"||r.ObservedState!="loaded"||r.Runner.KeepAlive!=-1{now:=time.Now().UTC();_ = m.model.Transition(modelstate.Unavailable,now,"","model_unavailable");_,_,persistErr:=m.queue.DrainWith(queue.Interrupted,func(entry *queue.Entry,active bool)error{from:="waiting";if active{from="active"};return m.finishEntry(context.Background(),entry,from,"failed","model_unavailable",503)});if persistErr!=nil{m.degradePersistence()};continue};m.setLoaded(time.Now().UTC())}}}
