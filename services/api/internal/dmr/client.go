package dmr

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strings"
 "time"
)

type Client struct{base *url.URL;model string;http *http.Client}
func New(raw,model string)(*Client,error){u,e:=url.Parse(raw);if e!=nil||u.Scheme!="http"||u.Host==""||u.RawQuery!=""||u.Fragment!=""{return nil,errors.New("invalid AI_MODEL_URL")};u.Path=strings.TrimSuffix(u.Path,"/");return &Client{base:u,model:model,http:&http.Client{Transport:&http.Transport{Proxy:nil,MaxIdleConnsPerHost:2,ResponseHeaderTimeout:30*time.Second}}},nil}
func(c *Client)Model()string{return c.model}
func(c *Client)Do(ctx context.Context,endpoint string,payload map[string]any)(*http.Response,error){if endpoint!="chat/completions"&&endpoint!="completions"{return nil,errors.New("invalid DMR endpoint")};payload["model"]=c.model;b,e:=json.Marshal(payload);if e!=nil{return nil,e};u:=*c.base;u.Path=strings.TrimSuffix(u.Path,"/")+"/engines/v1/"+endpoint;req,e:=http.NewRequestWithContext(ctx,http.MethodPost,u.String(),bytes.NewReader(b));if e!=nil{return nil,e};req.Header.Set("Content-Type","application/json");resp,e:=c.http.Do(req);if e!=nil{return nil,e};if resp.StatusCode<200||resp.StatusCode>=300{io.Copy(io.Discard,io.LimitReader(resp.Body,64<<10));resp.Body.Close();return nil,fmt.Errorf("DMR status %d",resp.StatusCode)};return resp,nil}
type Usage struct{PromptTokens int64 `json:"prompt_tokens"`;CompletionTokens int64 `json:"completion_tokens"`;ReasoningTokens int64 `json:"reasoning_tokens"`;TotalTokens int64 `json:"total_tokens"`;ReasoningPresent bool `json:"-"`}
func(u *Usage)UnmarshalJSON(data []byte)error{var wire struct{PromptTokens int64 `json:"prompt_tokens"`;CompletionTokens int64 `json:"completion_tokens"`;ReasoningTokens *int64 `json:"reasoning_tokens"`;TotalTokens int64 `json:"total_tokens"`};if e:=json.Unmarshal(data,&wire);e!=nil{return e};u.PromptTokens=wire.PromptTokens;u.CompletionTokens=wire.CompletionTokens;u.TotalTokens=wire.TotalTokens;u.ReasoningPresent=wire.ReasoningTokens!=nil;if wire.ReasoningTokens!=nil{u.ReasoningTokens=*wire.ReasoningTokens};return nil}
type Function struct{Name string `json:"name"`;Arguments string `json:"arguments"`}
type ToolCall struct{Index *int `json:"index,omitempty"`;ID string `json:"id"`;Type string `json:"type"`;Function Function `json:"function"`}
type ChatMessage struct{Role string `json:"role"`;Content *string `json:"content"`;ReasoningContent *string `json:"reasoning_content"`;ToolCalls []ToolCall `json:"tool_calls"`}
type ChatChoice struct{Index int `json:"index"`;Message ChatMessage `json:"message"`;Delta ChatMessage `json:"delta"`;FinishReason *string `json:"finish_reason"`}
type CompletionChoice struct{Index int `json:"index"`;Text string `json:"text"`;ReasoningContent *string `json:"reasoning_content"`;FinishReason *string `json:"finish_reason"`}
