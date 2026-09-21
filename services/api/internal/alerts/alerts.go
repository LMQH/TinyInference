package alerts
import("context";"github.com/google/uuid";"github.com/jackc/pgx/v5/pgxpool")
type Manager struct{db *pgxpool.Pool;holder uuid.UUID;epoch int64}
func New(db *pgxpool.Pool,holder uuid.UUID,epoch int64)*Manager{return &Manager{db,holder,epoch}}
func(m *Manager)Raise(ctx context.Context,severity,code string)error{id,_:=uuid.NewV7();_,e:=m.db.Exec(ctx,"select upsert_alert($1,$2,$3,$4,$5)",m.holder,m.epoch,id,severity,code);return e}
func(m *Manager)Resolve(ctx context.Context,code string)error{_,e:=m.db.Exec(ctx,"select resolve_alert($1,$2,$3)",m.holder,m.epoch,code);return e}
