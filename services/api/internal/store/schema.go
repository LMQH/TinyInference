package store
import "context"
func(s *Store)RequireSchema(ctx context.Context,expected int64)error{var version,count int64;if e:=s.pool.QueryRow(ctx,"select coalesce(max(version),0),count(*) from api_schema_version").Scan(&version,&count);e!=nil{return e};if version!=expected||count!=expected{return ErrSchemaMismatch};return nil}
