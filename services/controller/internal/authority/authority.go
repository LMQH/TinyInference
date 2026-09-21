package authority
import("context";"errors";"time";"github.com/google/uuid";"github.com/jackc/pgx/v5/pgxpool")
type Validator struct{pool *pgxpool.Pool}
func Open(ctx context.Context,dsn string)(*Validator,error){p,e:=pgxpool.New(ctx,dsn);if e!=nil{return nil,e};if e=p.Ping(ctx);e!=nil{p.Close();return nil,e};return &Validator{p},nil}
func(v *Validator)Close(){v.pool.Close()}
func(v *Validator)Check(ctx context.Context,epoch int64,holder uuid.UUID)error{var e int64;var h uuid.UUID;var heartbeat time.Time;if err:=v.pool.QueryRow(ctx,"select epoch,holder_id,heartbeat_at from controller_authority_epoch").Scan(&e,&h,&heartbeat);err!=nil{return errors.New("authority unavailable")};age:=time.Since(heartbeat);if e!=epoch||h!=holder||age>6*time.Second||age<(-time.Second){return errors.New("authority mismatch")};return nil}
func(v *Validator)Healthy(ctx context.Context)error{var one int;return v.pool.QueryRow(ctx,"select 1").Scan(&one)}
