package auth
import("net/http/httptest";"testing")
func TestBearerShapeAndValue(t *testing.T){for _,tc:=range []struct{headers []string;want bool}{{[]string{"Bearer 888888"},true},{nil,false},{[]string{"bearer 888888"},false},{[]string{"Bearer wrong"},false},{[]string{"Bearer 888888","Bearer 888888"},false},{[]string{"Bearer  888888"},false}}{r:=httptest.NewRequest("GET","/v1/models",nil);for _,v:=range tc.headers{r.Header.Add("Authorization",v)};if got:=Valid(r,"888888");got!=tc.want{t.Fatalf("%v got %v",tc.headers,got)}}}
