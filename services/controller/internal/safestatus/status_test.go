package safestatus

import("testing";"github.com/google/uuid")

func TestIndeterminateDoesNotInventRuntimeFacts(t *testing.T){
 result:=Indeterminate("status",7,uuid.NewString(),"local/model:q4","aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
 if result.Runner.KeepAlive==-1{t.Fatal("indeterminate status claimed verified infinite retention")}
 if result.RuntimeResources.Metal!="unknown"||result.RuntimeResources.MetalSource!="unavailable"{t.Fatalf("metal provenance = %q/%q",result.RuntimeResources.Metal,result.RuntimeResources.MetalSource)}
}
