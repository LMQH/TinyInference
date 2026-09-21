package operations
import("context";"mini-inference/services/api/internal/store")
type Snapshot struct{Runs map[string]store.Run;ModelOperations []store.ModelOperation}
type Source interface{LatestRuns(context.Context)(map[string]store.Run,error);ModelOperations(context.Context,int)([]store.ModelOperation,error)}
func Read(ctx context.Context,s Source)(Snapshot,error){runs,e:=s.LatestRuns(ctx);if e!=nil{return Snapshot{},e};ops,e:=s.ModelOperations(ctx,50);if e!=nil{return Snapshot{},e};return Snapshot{runs,ops},nil}
