package queue

import (
 "container/list"
 "context"
 "errors"
 "sync"
 "time"
)
var ErrFull=errors.New("queue full")
var ErrNotWaiting=errors.New("request not waiting")
type Result string
const ( Promoted Result="promoted"; Cancelled Result="cancelled"; CancelledPersisted Result="cancelled_persisted"; OperatorCancelled Result="operator_cancelled"; TimedOut Result="queue_timeout"; Interrupted Result="interrupted"; Stopped Result="stopped"; PersistenceFailed Result="persistence_failed" )
type Entry struct { ID string; ArrivalSeq uint64; EnqueuedAt time.Time; Deadline time.Time; Context context.Context; ready chan Result; cancel context.CancelFunc; persistTimeout func(*Entry)error; persistCancel func(*Entry)error }
type Snapshot struct { ActiveID string; Waiting []Entry }
type Queue struct { mu sync.Mutex; waiting *list.List; byID map[string]*list.Element; active *Entry; next uint64; capacity int; changed chan struct{}; closed bool }
func New(capacity int)*Queue{q:=&Queue{waiting:list.New(),byID:map[string]*list.Element{},capacity:capacity,changed:make(chan struct{},1)};go q.expireLoop();return q}
func (q *Queue) signal(){select{case q.changed<-struct{}{}:default:}}
func (q *Queue) Admit(ctx context.Context,id string,now time.Time)(*Entry,bool,error){return q.AdmitWith(ctx,id,now,func(*Entry,bool)error{return nil})}
// AdmitWith serializes the durable admission callback before making the entry visible.
func(q *Queue)AdmitWith(ctx context.Context,id string,now time.Time,persist func(*Entry,bool)error)(*Entry,bool,error){return q.AdmitDurable(ctx,id,now,persist,nil,nil)}
func(q *Queue)AdmitDurable(ctx context.Context,id string,now time.Time,persist func(*Entry,bool)error,persistTimeout,persistCancel func(*Entry)error)(*Entry,bool,error){q.mu.Lock();defer q.mu.Unlock();if q.closed{return nil,false,ErrFull};active:=q.active==nil;if !active&&q.waiting.Len()>=q.capacity{return nil,false,ErrFull};seq:=q.next+1;e:=&Entry{ID:id,ArrivalSeq:seq,EnqueuedAt:now,Deadline:now.Add(30*time.Minute),Context:ctx,ready:make(chan Result,1),persistTimeout:persistTimeout,persistCancel:persistCancel};if err:=persist(e,active);err!=nil{return nil,false,err};q.next=seq;if active{q.active=e;return e,true,nil};q.byID[id]=q.waiting.PushBack(e);q.signal();return e,false,nil}
func (e *Entry) Wait(ctx context.Context)Result{select{case r:=<-e.ready:return r;case<-ctx.Done():return Cancelled}}
func(q *Queue)Cancel(id string,r Result)error{return q.CancelWith(id,r,func(*Entry)error{return nil})}
func(q *Queue)CancelWith(id string,r Result,persist func(*Entry)error)error{q.mu.Lock();defer q.mu.Unlock();el:=q.byID[id];if el==nil{return ErrNotWaiting};e:=el.Value.(*Entry);if err:=persist(e);err!=nil{q.interruptWaitingLocked();q.signal();return err};delete(q.byID,id);q.waiting.Remove(el);e.ready<-r;q.signal();return nil}
func(q *Queue)SetActiveCancel(id string,cancel context.CancelFunc)bool{q.mu.Lock();defer q.mu.Unlock();if q.active==nil||q.active.ID!=id{return false};q.active.cancel=cancel;return true}
func(q *Queue)CompleteActive(now time.Time)*Entry{e,_:=q.CompleteActiveWith(now,func(*Entry)error{return nil});return e}
func(q *Queue)CompleteActiveWith(now time.Time,persist func(*Entry)error)(*Entry,error){q.mu.Lock();defer q.mu.Unlock();q.active=nil;if err:=q.expireLocked(now);err!=nil{q.signal();return nil,err};for q.waiting.Len()>0{el:=q.waiting.Front();e:=el.Value.(*Entry);q.waiting.Remove(el);delete(q.byID,e.ID);if e.Context.Err()!=nil{if e.persistCancel!=nil{if err:=e.persistCancel(e);err!=nil{e.ready<-PersistenceFailed;q.interruptWaitingLocked();q.signal();return nil,err};e.ready<-CancelledPersisted}else{e.ready<-Cancelled};continue};if err:=persist(e);err!=nil{e.ready<-Interrupted;q.interruptWaitingLocked();q.signal();return nil,err};q.active=e;e.ready<-Promoted;q.signal();return e,nil};q.signal();return nil,nil}
func(q *Queue)expireLocked(now time.Time)error{for el:=q.waiting.Front();el!=nil;{next:=el.Next();e:=el.Value.(*Entry);if !e.Deadline.After(now){if e.persistTimeout!=nil{if err:=e.persistTimeout(e);err!=nil{q.waiting.Remove(el);delete(q.byID,e.ID);e.ready<-PersistenceFailed;q.interruptWaitingLocked();return err}};q.waiting.Remove(el);delete(q.byID,e.ID);e.ready<-TimedOut};el=next};return nil}
func(q *Queue)expireLoop(){var timer *time.Timer;for{q.mu.Lock();if q.closed{q.mu.Unlock();return};var wait time.Duration=time.Hour;if f:=q.waiting.Front();f!=nil{earliest:=f.Value.(*Entry).Deadline;for el:=f.Next();el!=nil;el=el.Next(){if d:=el.Value.(*Entry).Deadline;d.Before(earliest){earliest=d}};wait=time.Until(earliest);if wait<0{wait=0}};q.mu.Unlock();if timer==nil{timer=time.NewTimer(wait)}else{if !timer.Stop(){select{case<-timer.C:default:}};timer.Reset(wait)};select{case now:=<-timer.C:q.mu.Lock();_=q.expireLocked(now);q.mu.Unlock();case<-q.changed:}}}
func(q *Queue)Drain(r Result)(active *Entry,waiting []*Entry){q.mu.Lock();defer q.mu.Unlock();active=q.active;q.active=nil;if active!=nil&&active.cancel!=nil{active.cancel()};for el:=q.waiting.Front();el!=nil;el=el.Next(){e:=el.Value.(*Entry);waiting=append(waiting,e);e.ready<-r};q.waiting.Init();q.byID=map[string]*list.Element{};q.signal();return}
func(q *Queue)DrainWith(r Result,persist func(*Entry,bool)error)(active *Entry,waiting []*Entry,err error){q.mu.Lock();defer q.mu.Unlock();active=q.active;if active!=nil&&active.cancel!=nil{active.cancel()};for el:=q.waiting.Front();el!=nil;el=el.Next(){waiting=append(waiting,el.Value.(*Entry))};if persist!=nil{for _,e:=range waiting{if err=persist(e,false);err!=nil{break}};if err==nil&&active!=nil{err=persist(active,true)}};q.active=nil;signal:=r;if err!=nil{signal=Interrupted};for _,e:=range waiting{e.ready<-signal};q.waiting.Init();q.byID=map[string]*list.Element{};q.signal();return}
func(q *Queue)interruptWaitingLocked(){for el:=q.waiting.Front();el!=nil;el=el.Next(){el.Value.(*Entry).ready<-Interrupted};q.waiting.Init();q.byID=map[string]*list.Element{}}
func(q *Queue)Snapshot()Snapshot{q.mu.Lock();defer q.mu.Unlock();s:=Snapshot{};if q.active!=nil{s.ActiveID=q.active.ID};for el:=q.waiting.Front();el!=nil;el=el.Next(){e:=*(el.Value.(*Entry));e.ready=nil;s.Waiting=append(s.Waiting,e)};return s}
func(q *Queue)Close(){q.mu.Lock();q.closed=true;q.mu.Unlock();q.signal()}
