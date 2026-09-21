package main

import(
 "context"
 "errors"
 "fmt"
 "net/http"
 "os"
 "os/signal"
 "syscall"
 "time"

 "mini-inference/services/controller/internal/adapter"
 "mini-inference/services/controller/internal/authority"
 "mini-inference/services/controller/internal/config"
 controllerhttp "mini-inference/services/controller/internal/http"
)
func main(){if len(os.Args)>1{if os.Args[1]!="healthcheck"{fmt.Fprintln(os.Stderr,"unknown command");os.Exit(1)};if e:=healthcheck(os.Args[2:]);e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)};return};if e:=run();e!=nil{fmt.Fprintln(os.Stderr,"controller startup failed");os.Exit(1)}}
func run()error{cfg,e:=config.Load();if e!=nil{return e};ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);v,e:=authority.Open(ctx,cfg.DatabaseURL);cancel();if e!=nil{return e};defer v.Close();a:=adapter.New(adapter.Config{Binary:cfg.Binary,RunnerHost:cfg.RunnerHost,ModelRef:cfg.ModelRef,SourceSHA:cfg.SourceSHA});srv:=&http.Server{Addr:cfg.ListenAddr,Handler:controllerhttp.New(v,a,cfg.ModelRef,cfg.SourceSHA).Handler(),ReadHeaderTimeout:5*time.Second,IdleTimeout:30*time.Second};errs:=make(chan error,1);go func(){errs<-srv.ListenAndServe()}();signals:=make(chan os.Signal,1);signal.Notify(signals,syscall.SIGTERM,syscall.SIGINT);select{case<-signals:ctx,cancel:=context.WithTimeout(context.Background(),65*time.Second);defer cancel();return srv.Shutdown(ctx);case e:=<-errs:if errors.Is(e,http.ErrServerClosed){return nil};return e}}
func healthcheck(args []string)error{const prefix="--url=";if len(args)!=1||args[0]!=prefix+"http://127.0.0.1:9090/health/ready"{return errors.New("usage: healthcheck --url=http://127.0.0.1:9090/health/ready")};c:=http.Client{Timeout:2*time.Second};r,e:=c.Get(args[0][len(prefix):]);if e!=nil{return errors.New("healthcheck failed")};defer r.Body.Close();if r.StatusCode/100!=2{return errors.New("healthcheck not ready")};return nil}
