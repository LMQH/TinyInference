package safeerror
import("encoding/json";"net/http";"strconv";g "mini-inference/services/api/internal/contract/generated")
type Error struct{Status int;Type,Code,Message string;Param *string;Retryable bool;RetryAfter *int}
func New(status int,typ,code,message string,param *string,retry bool,after *int)*Error{return &Error{status,typ,code,message,param,retry,after}}
func(e *Error)Error()string{return e.Code}
func Write(w http.ResponseWriter,id string,e *Error){w.Header().Set("Content-Type","application/json");w.Header().Set("X-Request-ID",id);if e.Status==401{w.Header().Set("WWW-Authenticate","Bearer")};if e.RetryAfter!=nil{w.Header().Set("Retry-After",strconv.Itoa(*e.RetryAfter))};w.WriteHeader(e.Status);_ = json.NewEncoder(w).Encode(g.ErrorEnvelope{Error:g.ErrorDetail{Message:e.Message,Type:e.Type,Code:e.Code,Param:e.Param,RequestID:id,Retryable:e.Retryable,RetryAfterSeconds:e.RetryAfter}})}
func Invalid(code string,param *string)*Error{return New(400,"invalid_request_error",code,"The request is invalid.",param,false,nil)}
