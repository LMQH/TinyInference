package requeststate
import "testing"
func TestTerminalStatesCannotChange(t *testing.T){for _,from:=range []State{Succeeded,Failed,Cancelled,QueueTimeout,Interrupted,Rejected}{for _,to:=range []State{Waiting,Active,Succeeded,Failed,Cancelled,QueueTimeout,Interrupted,Rejected}{if CanTransition(from,to){t.Fatalf("terminal %s transitioned to %s",from,to)}}}}
func TestOnlyLegalTransitions(t *testing.T){legal:=[][2]State{{Waiting,Active},{Waiting,Cancelled},{Waiting,QueueTimeout},{Waiting,Interrupted},{Active,Succeeded},{Active,Failed},{Active,Cancelled},{Active,Interrupted}};for _,p:=range legal{if !CanTransition(p[0],p[1]){t.Fatalf("rejected %v",p)}}}
