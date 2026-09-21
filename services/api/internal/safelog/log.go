package safelog
import("context";"encoding/json";"io";"time";"github.com/google/uuid";"github.com/jackc/pgx/v5/pgxpool")
type Logger struct{out io.Writer;db *pgxpool.Pool;holder uuid.UUID;epoch int64}
func New(out io.Writer,db *pgxpool.Pool,holder uuid.UUID,epoch int64)*Logger{return &Logger{out,db,holder,epoch}}
func(l *Logger)Event(ctx context.Context,level,code string,requestID,operationID *uuid.UUID){record:=map[string]any{"timestamp":time.Now().UTC(),"level":level,"event_code":code,"request_id":requestID,"operation_id":operationID,"authority_epoch":l.epoch};_ = json.NewEncoder(l.out).Encode(record);if l.db!=nil{_,_=l.db.Exec(ctx,"select record_safe_log($1,$2,$3,$4,$5,$6)",l.holder,l.epoch,level,code,requestID,operationID)}}
