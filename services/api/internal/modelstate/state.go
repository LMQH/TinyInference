package modelstate

import (
 "fmt"
 "sync"
 "time"
)
type State string
const ( Unloaded State="unloaded"; Starting State="starting"; Ready State="ready"; Stopping State="stopping"; Unavailable State="unavailable" )
type Snapshot struct { State State; TransitionStartedAt time.Time; OperationID string; FailureCode string }
type Machine struct { mu sync.RWMutex; snapshot Snapshot }
func New(now time.Time)*Machine{return &Machine{snapshot:Snapshot{State:Unloaded,TransitionStartedAt:now}}}
func (m *Machine) Snapshot()Snapshot{m.mu.RLock();defer m.mu.RUnlock();return m.snapshot}
func (m *Machine) Transition(to State,now time.Time,operationID,failure string)error{m.mu.Lock();defer m.mu.Unlock();from:=m.snapshot.State;ok:=(from==Unloaded&&to==Starting)||(from==Starting&&(to==Ready||to==Unavailable))||(from==Ready&&to==Stopping)||(from==Stopping&&(to==Unloaded||to==Unavailable))||(from==Unavailable&&(to==Starting||to==Stopping||to==Unavailable))||(from==Ready&&to==Unavailable);if !ok{return fmt.Errorf("illegal model transition %s -> %s",from,to)};m.snapshot=Snapshot{State:to,TransitionStartedAt:now,OperationID:operationID,FailureCode:failure};return nil}
func (m *Machine) Admissible(now time.Time,loadedObservedAt time.Time)bool{m.mu.RLock();defer m.mu.RUnlock();return m.snapshot.State==Ready&&!loadedObservedAt.IsZero()&&now.Sub(loadedObservedAt)<=10*time.Second}
