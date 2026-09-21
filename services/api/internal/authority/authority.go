package authority

import (
 "context"
 "errors"
 "sync/atomic"
 "time"

 "github.com/google/uuid"
 "github.com/jackc/pgx/v5"
)
const AdvisoryLock int64=741291551
type Fence struct{conn *pgx.Conn;holder uuid.UUID;epoch int64;alive atomic.Bool;lost chan struct{}}
func Acquire(ctx context.Context,dsn string)(*Fence,error){c,e:=pgx.Connect(ctx,dsn);if e!=nil{return nil,e};var ok bool;if e=c.QueryRow(ctx,"select pg_try_advisory_lock($1)",AdvisoryLock).Scan(&ok);e!=nil||!ok{c.Close(ctx);if e!=nil{return nil,e};return nil,errors.New("authority unavailable")};h,e:=uuid.NewV7();if e!=nil{c.Close(ctx);return nil,e};var epoch int64;if e=c.QueryRow(ctx,"select acquire_backend_authority($1)",h).Scan(&epoch);e!=nil{c.Close(ctx);return nil,e};f:=&Fence{conn:c,holder:h,epoch:epoch,lost:make(chan struct{})};f.alive.Store(true);return f,nil}
func(f *Fence)Holder()uuid.UUID{return f.holder}
func(f *Fence)Epoch()int64{return f.epoch}
func(f *Fence)Alive()bool{return f.alive.Load()}
func(f *Fence)Lost()<-chan struct{}{return f.lost}
func(f *Fence)RunHeartbeat(ctx context.Context){t:=time.NewTicker(2*time.Second);defer t.Stop();for{select{case<-ctx.Done():return;case<-t.C:if _,e:=f.conn.Exec(ctx,"select heartbeat_backend_authority($1,$2)",f.holder,f.epoch);e!=nil{if f.alive.Swap(false){close(f.lost)};return}}}}
func(f *Fence)Close(ctx context.Context)error{if f.alive.Swap(false){_,_ = f.conn.Exec(ctx,"select release_backend_authority($1,$2)",f.holder,f.epoch);close(f.lost)};return f.conn.Close(ctx)}
