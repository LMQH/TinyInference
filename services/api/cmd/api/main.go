package main

import (
 "context"
 "errors"
 "fmt"
 "net/http"
 "os"
 "os/signal"
 "strings"
 "syscall"
 "time"

 "github.com/google/uuid"
 "mini-inference/services/api/internal/authority"
 "mini-inference/services/api/internal/dmr"
 adminhttp "mini-inference/services/api/internal/http/admin"
 publichttp "mini-inference/services/api/internal/http/public"
 "mini-inference/services/api/internal/lifecycle"
 "mini-inference/services/api/internal/modelstate"
 "mini-inference/services/api/internal/queue"
 "mini-inference/services/api/internal/resources"
 "mini-inference/services/api/internal/retention"
 "mini-inference/services/api/internal/store"
 "mini-inference/services/api/internal/tokenizer"
)

const tokenizerPath="/app/tokenizer.gguf"

func main(){
 if len(os.Args)>1{
  if os.Args[1]!="healthcheck"{fmt.Fprintln(os.Stderr,"unknown command");os.Exit(1)}
  if e:=healthcheck(os.Args[2:]);e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}
  return
 }
 if e:=run();e!=nil{fmt.Fprintln(os.Stderr,"api startup failed:",e);os.Exit(1)}
}

func run()error{
 publicAddr:=required("PUBLIC_ADDR");adminAddr:=required("ADMIN_ADDR")
 if publicAddr!=":8888"||adminAddr!=":8889"{return errors.New("config_secret")}
 key,e:=readSecret("API_KEY_FILE");if e!=nil||key!="888888"{return errors.New("config_secret")}
 dsn,e:=readSecret("DATABASE_URL_FILE");if e!=nil{return errors.New("config_secret")}
 retentionDSN,e:=readSecret("RETENTION_DATABASE_URL_FILE");if e!=nil{return errors.New("config_secret")}
 if required("AI_MODEL_NAME")!="local/minicpm5-2b:q4_k_m-ec2d58016400"||required("AI_MODEL_URL")!="http://model-runner.docker.internal:12435"{return errors.New("config_secret")}
 if e=verifyCompatibilityManifest();e!=nil{return errors.New("compatibility_manifest")}

 ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second);defer cancel()
 st,e:=store.Open(ctx,dsn);if e!=nil{return errors.New("db_open")};defer st.Close()
 if e=st.RequireSchema(ctx,7);e!=nil{return errors.New("db_schema")}
 fence,e:=authority.Acquire(ctx,dsn);if e!=nil{return errors.New("authority_acquire")};defer fence.Close(context.Background())
 if e=st.Reconcile(ctx,fence.Holder(),fence.Epoch());e!=nil{return errors.New("authority_reconcile")}
 controller,e:=lifecycle.NewController(required("CONTROLLER_BASE_URL"));if e!=nil{return errors.New("controller_init")}
 inference,e:=dmr.New(required("AI_MODEL_URL"),required("AI_MODEL_NAME"));if e!=nil{return errors.New("dmr_init")}
 tok,e:=tokenizer.LoadGGUF(tokenizerPath);if e!=nil{return errors.New("tokenizer_init")}
 q:=queue.New(20);defer q.Close()
 model:=modelstate.New(time.Now().UTC())
 manager:=lifecycle.NewManager(fence,st,controller,inference,tok,model,q)
 if e=manager.ReconcileStartup(ctx);e!=nil{return errors.New("startup_lifecycle_reconcile")}
 scheduler,e:=retention.Open(ctx,retentionDSN);if e!=nil{return errors.New("retention_init")};defer scheduler.Close()

 rootCtx,rootCancel:=context.WithCancel(context.Background());defer rootCancel()
 go fence.RunHeartbeat(rootCtx);go manager.Poll(rootCtx);go scheduler.Run(rootCtx)
 collector:=resources.New(controller,func()(int64,uuid.UUID){return fence.Epoch(),fence.Holder()})
 publicServer:=&http.Server{Addr:publicAddr,Handler:publichttp.New(key,fence,st,q,model,manager.LoadedObserved,inference,tok).Handler(),ReadHeaderTimeout:5*time.Second,IdleTimeout:60*time.Second}
 adminServer:=&http.Server{Addr:adminAddr,Handler:adminhttp.New(st,fence,model,q,manager,collector).Handler(),ReadHeaderTimeout:5*time.Second,IdleTimeout:60*time.Second}
 errs:=make(chan error,2)
 go func(){errs<-publicServer.ListenAndServe()}();go func(){errs<-adminServer.ListenAndServe()}()
 signals:=make(chan os.Signal,1);signal.Notify(signals,syscall.SIGTERM,syscall.SIGINT);defer signal.Stop(signals)
 select{
 case<-fence.Lost():rootCancel();return errors.New("authority_lost")
 case e:=<-errs:if !errors.Is(e,http.ErrServerClosed){return errors.New("listener")};return nil
 case<-signals:
  rootCancel()
  shutdownCtx,shutdownCancel:=context.WithTimeout(context.Background(),60*time.Second);defer shutdownCancel()
  _=adminServer.Shutdown(shutdownCtx)
  unloadErr:=manager.Shutdown(shutdownCtx)
  _=publicServer.Shutdown(shutdownCtx)
  if unloadErr!=nil{return errors.New("shutdown_lifecycle")};return nil
 }
}
func required(name string)string{return os.Getenv(name)}
func readSecret(env string)(string,error){p:=os.Getenv(env);if p==""{return "",errors.New("secret file not configured")};info,e:=os.Lstat(p);if e!=nil||!info.Mode().IsRegular()||info.Mode().Perm()&0077!=0{return "",errors.New("invalid secret file")};b,e:=os.ReadFile(p);if e!=nil{return "",e};return strings.TrimSpace(string(b)),nil}
func healthcheck(args []string)error{const prefix="--url=";if len(args)!=1||args[0]!=prefix+"http://127.0.0.1:8889/health/ready"{return errors.New("usage: healthcheck --url=http://127.0.0.1:8889/health/ready")};c:=http.Client{Timeout:2*time.Second};r,e:=c.Get(args[0][len(prefix):]);if e!=nil{return errors.New("healthcheck failed")};defer r.Body.Close();if r.StatusCode/100!=2{return errors.New("healthcheck not ready")};return nil}
