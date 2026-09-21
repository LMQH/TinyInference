package requeststate

import "fmt"

type State string
const (
 Waiting State="waiting"; Active State="active"; Succeeded State="succeeded"; Failed State="failed"; Cancelled State="cancelled"; QueueTimeout State="queue_timeout"; Interrupted State="interrupted"; Rejected State="rejected"
)
func (s State) Terminal() bool { return s==Succeeded||s==Failed||s==Cancelled||s==QueueTimeout||s==Interrupted||s==Rejected }
func CanTransition(from,to State) bool {
 switch from { case Waiting: return to==Active||to==Failed||to==Cancelled||to==QueueTimeout||to==Interrupted; case Active: return to==Succeeded||to==Failed||to==Cancelled||to==Interrupted }
 return false
}
func Transition(from,to State) error { if !CanTransition(from,to){return fmt.Errorf("illegal request transition %s -> %s",from,to)};return nil }
