package metrics
import("strconv";"strings";"sync")
type Counters struct{mu sync.RWMutex;values map[string]uint64}
func New()*Counters{return &Counters{values:map[string]uint64{}}}
var routes=map[string]bool{"models":true,"chat.completions":true,"completions":true,"admin":true}
func(c *Counters)Inc(route,method,statusClass string){if !routes[route]||method!="GET"&&method!="POST"||statusClass!="2xx"&&statusClass!="4xx"&&statusClass!="5xx"{return};k:=route+"|"+method+"|"+statusClass;c.mu.Lock();c.values[k]++;c.mu.Unlock()}
func(c *Counters)Prometheus()string{c.mu.RLock();defer c.mu.RUnlock();var b strings.Builder;for k,v:=range c.values{p:=strings.Split(k,"|");b.WriteString("mini_inference_http_requests_total{route=\"");b.WriteString(p[0]);b.WriteString("\",method=\"");b.WriteString(p[1]);b.WriteString("\",status_class=\"");b.WriteString(p[2]);b.WriteString("\"} ");b.WriteString(strconv.FormatUint(v,10));b.WriteByte('\n')};return b.String()}
