package store

import (
 "context"
 "encoding/base64"
 "errors"
 "fmt"
 "strconv"
 "strings"
 "time"

 "github.com/google/uuid"
 "github.com/jackc/pgx/v5/pgxpool"
)
var ErrSchemaMismatch=errors.New("schema version mismatch")

type Store struct{pool *pgxpool.Pool}
func Open(ctx context.Context,dsn string)(*Store,error){p,e:=pgxpool.New(ctx,dsn);if e!=nil{return nil,e};if e=p.Ping(ctx);e!=nil{p.Close();return nil,e};return &Store{pool:p},nil}
func(s *Store)Close(){s.pool.Close()}
type Admit struct{Holder uuid.UUID;Epoch int64;ID uuid.UUID;Endpoint,Model string;Stream,Reasoning bool;Status string;Arrival int64;Created,Enqueued time.Time;Started *time.Time}
func(s *Store)Admit(ctx context.Context,a Admit)error{tx,e:=s.pool.Begin(ctx);if e!=nil{return e};defer tx.Rollback(ctx);if _,e=tx.Exec(ctx,"select admit_request($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)",a.Holder,a.Epoch,a.ID,a.Endpoint,a.Model,a.Stream,a.Reasoning,a.Status,a.Arrival,a.Created,a.Enqueued,a.Started);e!=nil{return e};if e=tx.QueryRow(ctx,"select publish_admin_event($1,$2,'queue_changed',$3::jsonb)",a.Holder,a.Epoch,[]byte(`{"changed":["queue","requests"]}`)).Scan(new(int64));e!=nil{return e};return tx.Commit(ctx)}
type Transition struct{Holder uuid.UUID;Epoch int64;ID uuid.UUID;From,To,Reason string;HTTPStatus *int16;Started,FirstToken,Completed *time.Time;Input,Output,Reasoning,QueueWait,TTFT,Duration,Generation *int64;ToolCalls bool}
func(s *Store)Transition(ctx context.Context,x Transition)error{tx,e:=s.pool.Begin(ctx);if e!=nil{return e};defer tx.Rollback(ctx);if _,e=tx.Exec(ctx,"select transition_request($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)",x.Holder,x.Epoch,x.ID,x.From,x.To,x.Reason,x.HTTPStatus,x.Started,x.FirstToken,x.Completed,x.Input,x.Output,x.Reasoning,x.QueueWait,x.TTFT,x.Duration,x.Generation,x.ToolCalls);e!=nil{return e};if e=tx.QueryRow(ctx,"select publish_admin_event($1,$2,'queue_changed',$3::jsonb)",x.Holder,x.Epoch,[]byte(`{"changed":["queue","requests","metrics"]}`)).Scan(new(int64));e!=nil{return e};return tx.Commit(ctx)}
func(s *Store)Reconcile(ctx context.Context,h uuid.UUID,epoch int64)error{_,e:=s.pool.Exec(ctx,"select reconcile_prior_epoch($1,$2)",h,epoch);return e}
func(s *Store)BeginOperation(ctx context.Context,h uuid.UUID,epoch int64,id uuid.UUID,op string,at time.Time)error{_,e:=s.pool.Exec(ctx,"select begin_model_operation($1,$2,$3,$4,$5)",h,epoch,id,op,at);return e}
func(s *Store)FinishOperation(ctx context.Context,h uuid.UUID,epoch int64,id uuid.UUID,status,code,observed string)error{_,e:=s.pool.Exec(ctx,"select finish_model_operation($1,$2,$3,$4,$5,$6)",h,epoch,id,status,code,observed);return e}
func(s *Store)SnapshotVersion(ctx context.Context)(int64,error){var v int64;e:=s.pool.QueryRow(ctx,"select snapshot_version from api_snapshot_state").Scan(&v);return v,e}
func(s *Store)Publish(ctx context.Context,h uuid.UUID,epoch int64,typ string,data []byte)(int64,error){var v int64;e:=s.pool.QueryRow(ctx,"select publish_admin_event($1,$2,$3,$4::jsonb)",h,epoch,typ,data).Scan(&v);return v,e}
type RequestRecord struct{ID uuid.UUID `json:"id"`;Endpoint string `json:"endpoint"`;Model string `json:"public_model_id"`;Stream bool `json:"stream"`;Reasoning bool `json:"reasoning_enabled"`;ToolCalls bool `json:"tool_calls_returned"`;Status string `json:"status"`;Terminal *string `json:"terminal_code"`;HTTP *int16 `json:"http_status"`;Created time.Time `json:"created_at"`;Enqueued *time.Time `json:"enqueued_at"`;Started *time.Time `json:"started_at"`;FirstToken *time.Time `json:"first_token_at"`;Completed *time.Time `json:"completed_at"`;Input *int64 `json:"input_tokens"`;Output *int64 `json:"output_tokens"`;ReasoningTokens *int64 `json:"reasoning_tokens"`;QueueWait *int64 `json:"queue_wait_ms"`;TTFT *int64 `json:"ttft_ms"`;Duration *int64 `json:"duration_ms"`;Generation *int64 `json:"-"`;Throughput *float64 `json:"throughput_tokens_per_second"`}
type Cursor struct{At time.Time;ID uuid.UUID}
func EncodeCursor(c Cursor)string{return base64.RawURLEncoding.EncodeToString([]byte(c.At.UTC().Format(time.RFC3339Nano)+"|"+c.ID.String()))}
func DecodeCursor(v string)(Cursor,error){b,e:=base64.RawURLEncoding.DecodeString(v);if e!=nil{return Cursor{},errors.New("invalid cursor")};p:=strings.Split(string(b),"|");if len(p)!=2{return Cursor{},errors.New("invalid cursor")};at,e:=time.Parse(time.RFC3339Nano,p[0]);if e!=nil{return Cursor{},errors.New("invalid cursor")};id,e:=uuid.Parse(p[1]);if e!=nil{return Cursor{},errors.New("invalid cursor")};return Cursor{at,id},nil}
func(s *Store)Requests(ctx context.Context,c *Cursor,limit int)([]RequestRecord,*Cursor,error){if limit<1||limit>100{return nil,nil,errors.New("invalid limit")};q:="select id,endpoint,public_model_id,stream,reasoning_enabled,tool_calls_returned,status,terminal_code,http_status,created_at,enqueued_at,started_at,first_token_at,completed_at,input_tokens,output_tokens,reasoning_tokens,queue_wait_ms,ttft_ms,duration_ms,generation_ms from api_request_history";args:=[]any{};if c!=nil{q+=" where (created_at,id)<($1,$2)";args=append(args,c.At,c.ID)};q+=fmt.Sprintf(" order by created_at desc,id desc limit %d",limit+1);rows,e:=s.pool.Query(ctx,q,args...);if e!=nil{return nil,nil,e};defer rows.Close();out:=[]RequestRecord{};for rows.Next(){var r RequestRecord;if e=rows.Scan(&r.ID,&r.Endpoint,&r.Model,&r.Stream,&r.Reasoning,&r.ToolCalls,&r.Status,&r.Terminal,&r.HTTP,&r.Created,&r.Enqueued,&r.Started,&r.FirstToken,&r.Completed,&r.Input,&r.Output,&r.ReasoningTokens,&r.QueueWait,&r.TTFT,&r.Duration,&r.Generation);e!=nil{return nil,nil,e};out=append(out,r)};if e=rows.Err();e!=nil{return nil,nil,e};var next *Cursor;if len(out)>limit{x:=out[limit-1];out=out[:limit];next=&Cursor{x.Created,x.ID}};return out,next,nil}
type Event struct{ID,Version int64;At time.Time;Type string;Data []byte}
func(s *Store)EventsAfter(ctx context.Context,id int64,limit int)([]Event,bool,error){var oldest *int64;_ = s.pool.QueryRow(ctx,"select min(id) from api_admin_events where occurred_at>=transaction_timestamp()-interval '30 days'").Scan(&oldest);if id>0&&(oldest==nil||id<*oldest-1){return nil,true,nil};rows,e:=s.pool.Query(ctx,"select id,snapshot_version,occurred_at,event_type,data from api_admin_events where id>$1 order by id limit $2",id,limit+1);if e!=nil{return nil,false,e};defer rows.Close();var out []Event;for rows.Next(){var v Event;if e=rows.Scan(&v.ID,&v.Version,&v.At,&v.Type,&v.Data);e!=nil{return nil,false,e};out=append(out,v)};if len(out)>limit{return nil,true,nil};return out,false,rows.Err()}
func(s *Store)Health(ctx context.Context)error{var one int;return s.pool.QueryRow(ctx,"select 1").Scan(&one)}
func ParseLimit(v string)(int,error){if v==""{return 50,nil};n,e:=strconv.Atoi(v);if e!=nil||n<1||n>100{return 0,errors.New("invalid limit")};return n,nil}
