package resources

import(
 "bufio"
 "context"
 "os"
 "strconv"
 "strings"
 "sync"
 "time"

 "github.com/google/uuid"
 g "mini-inference/services/api/internal/contract/generated"
 "mini-inference/services/api/internal/lifecycle"
)
type Collector struct{controller *lifecycle.Controller;epoch func()(int64,uuid.UUID);mu sync.Mutex;lastCPU uint64;lastAt time.Time}
func New(c *lifecycle.Controller,epoch func()(int64,uuid.UUID))*Collector{return &Collector{controller:c,epoch:epoch}}
func(c *Collector)Sample(ctx context.Context)g.ResourceSnapshot{now:=time.Now().UTC();snap:=g.ResourceSnapshot{Status:"unavailable",SampledAt:&now,CPUSource:"unavailable",UnifiedMemorySource:"unavailable",DiskSource:"unavailable",Metal:"unknown",MetalSource:"unavailable"};if pct,ok:=c.cpu(now);ok{snap.CPUPercent=&pct;snap.CPUSource="api_container"};epoch,holder:=c.epoch();if r,e:=c.controller.Call(ctx,"status",epoch,holder,uuid.NewString());e==nil{rr:=r.RuntimeResources;snap.UnifiedMemoryUsedBytes=rr.UnifiedMemoryUsedBytes;snap.UnifiedMemoryTotalBytes=rr.UnifiedMemoryTotalBytes;snap.UnifiedMemorySource=rr.UnifiedMemorySource;snap.DiskUsedBytes=rr.DiskUsedBytes;snap.DiskTotalBytes=rr.DiskTotalBytes;snap.DiskSource=rr.DiskSource;snap.Metal=rr.Metal;snap.MetalSource=rr.MetalSource;snap.SampledAt=&rr.SampledAt};available:=0;if snap.CPUPercent!=nil{available++};if snap.UnifiedMemoryUsedBytes!=nil{available++};if snap.DiskUsedBytes!=nil{available++};if snap.Metal!="unknown"{available++};if available==4{snap.Status="available"}else if available>0{snap.Status="partial"}else{snap.Status="unavailable"};if snap.SampledAt!=nil&&now.Sub(*snap.SampledAt)>15*time.Second{snap.Status="stale"};return snap}
func(c *Collector)cpu(now time.Time)(float64,bool){f,e:=os.Open("/sys/fs/cgroup/cpu.stat");if e!=nil{return 0,false};defer f.Close();var usec uint64;sc:=bufio.NewScanner(f);for sc.Scan(){p:=strings.Fields(sc.Text());if len(p)==2&&p[0]=="usage_usec"{usec,_=strconv.ParseUint(p[1],10,64)}};if usec==0{return 0,false};c.mu.Lock();defer c.mu.Unlock();if c.lastAt.IsZero(){c.lastAt=now;c.lastCPU=usec;return 0,false};elapsed:=now.Sub(c.lastAt).Microseconds();delta:=usec-c.lastCPU;c.lastAt=now;c.lastCPU=usec;if elapsed<=0{return 0,false};return float64(delta)*100/float64(elapsed),true}
