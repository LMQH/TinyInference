package lifecycle

import(
 "context"
 "errors"
 "testing"
 "time"

 "github.com/google/uuid"
 "mini-inference/services/api/internal/authority"
 "mini-inference/services/api/internal/modelstate"
 "mini-inference/services/api/internal/queue"
 "mini-inference/services/api/internal/store"
)

type failingOperationStore struct{err error}
func(f failingOperationStore)BeginOperation(context.Context,uuid.UUID,int64,uuid.UUID,string,time.Time)error{return f.err}
func(f failingOperationStore)FinishOperation(context.Context,uuid.UUID,int64,uuid.UUID,string,string,string)error{return f.err}
func(f failingOperationStore)Transition(context.Context,store.Transition)error{return f.err}

func TestFinishEntryPropagatesPersistenceFailureAndDegradesQueue(t *testing.T){
 persistErr:=errors.New("database unavailable");q:=queue.New(20);defer q.Close();now:=time.Now().UTC();_,_,_ = q.Admit(context.Background(),uuid.NewString(),now);waiting,_,_:=q.Admit(context.Background(),uuid.NewString(),now)
 model:=modelstate.New(now);if err:=model.Transition(modelstate.Starting,now,"op","");err!=nil{t.Fatal(err)};if err:=model.Transition(modelstate.Ready,now,"op","");err!=nil{t.Fatal(err)}
 m:=&Manager{fence:&authority.Fence{},store:failingOperationStore{persistErr},model:model,queue:q};if err:=m.finishEntry(context.Background(),waiting,"waiting","cancelled","model_stop",409);!errors.Is(err,persistErr){t.Fatalf("finishEntry error = %v",err)};m.degradePersistence()
 if got:=model.Snapshot().State;got!=modelstate.Unavailable{t.Fatalf("model state = %s",got)};snap:=q.Snapshot();if snap.ActiveID!=""||len(snap.Waiting)!=0{t.Fatalf("queue remained executable: %#v",snap)};if got:=waiting.Wait(context.Background());got!=queue.Interrupted{t.Fatalf("waiting result = %s",got)}
}
