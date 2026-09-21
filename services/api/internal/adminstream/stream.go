package adminstream
import("context";"encoding/json";"fmt";"net/http";"strconv";"time";g "mini-inference/services/api/internal/contract/generated";"mini-inference/services/api/internal/store")
type Source interface{EventsAfter(context.Context,int64,int)([]store.Event,bool,error);SnapshotVersion(context.Context)(int64,error)}
type Handler struct{source Source}
func New(s Source)*Handler{return &Handler{s}}
func(h *Handler)ServeHTTP(w http.ResponseWriter,r *http.Request){
 if r.Method!="GET"{w.WriteHeader(405);return}
 if r.URL.RawQuery!=""{w.WriteHeader(400);return}
 last:=int64(0)
 if v:=r.Header.Get("Last-Event-ID");v!=""{n,e:=strconv.ParseInt(v,10,64);if e!=nil||n<0{w.WriteHeader(400);return};last=n}
 f,ok:=w.(http.Flusher);if !ok{w.WriteHeader(500);return}
 w.Header().Set("Content-Type","text/event-stream; charset=utf-8")
 w.Header().Set("Cache-Control","no-cache, no-transform")
 w.Header().Set("X-Accel-Buffering","no")
 w.WriteHeader(200)
 events,resync,e:=h.source.EventsAfter(r.Context(),last,1000);if e!=nil{return}
 if resync{
  version,_:=h.source.SnapshotVersion(r.Context())
  h.write(w,g.AdminEvent{EventID:last,SnapshotVersion:version,OccurredAt:time.Now().UTC(),Type:"resync_required",Data:g.AdminEventData{Changed:[]string{"service","model","queue","resources","requests","metrics","alerts","logs","operations"}}})
  f.Flush()
 }else if last==0{
  version,_:=h.source.SnapshotVersion(r.Context())
  h.write(w,g.AdminEvent{EventID:0,SnapshotVersion:version,OccurredAt:time.Now().UTC(),Type:"snapshot_changed",Data:g.AdminEventData{Changed:[]string{"service","model","queue","resources"}}})
  f.Flush()
 }else{
  for _,e:=range events{var data g.AdminEventData;if json.Unmarshal(e.Data,&data)!=nil{return};h.write(w,g.AdminEvent{EventID:e.ID,SnapshotVersion:e.Version,OccurredAt:e.At,Type:e.Type,Data:data});last=e.ID;f.Flush()}
 }
 heartbeat:=time.NewTicker(15*time.Second);poll:=time.NewTicker(time.Second)
 defer heartbeat.Stop();defer poll.Stop()
 for{select{
 case<-r.Context().Done():return
 case<-heartbeat.C:fmt.Fprint(w,": heartbeat\n\n");f.Flush()
 case<-poll.C:
  events,resync,e:=h.source.EventsAfter(r.Context(),last,1000);if e!=nil{return}
  if resync{version,_:=h.source.SnapshotVersion(r.Context());h.write(w,g.AdminEvent{EventID:last,SnapshotVersion:version,OccurredAt:time.Now().UTC(),Type:"resync_required",Data:g.AdminEventData{Changed:[]string{"service","model","queue","resources","requests","metrics","alerts","logs","operations"}}});f.Flush();return}
  for _,e:=range events{var data g.AdminEventData;if json.Unmarshal(e.Data,&data)!=nil{return};h.write(w,g.AdminEvent{EventID:e.ID,SnapshotVersion:e.Version,OccurredAt:e.At,Type:e.Type,Data:data});last=e.ID;f.Flush()}
 }}
}
func(h *Handler)write(w http.ResponseWriter,e g.AdminEvent){b,_:=json.Marshal(e);fmt.Fprintf(w,"id: %d\ndata: %s\n\n",e.EventID,b)}
