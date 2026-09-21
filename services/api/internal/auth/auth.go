package auth
import("crypto/subtle";"net/http";"strings")
func Valid(r *http.Request,key string)bool{values:=r.Header.Values("Authorization");if len(values)!=1{return false};v:=values[0];if !strings.HasPrefix(v,"Bearer ")||strings.Count(v," ")!=1{return false};got:=strings.TrimPrefix(v,"Bearer ");return len(got)==len(key)&&subtle.ConstantTimeCompare([]byte(got),[]byte(key))==1}
