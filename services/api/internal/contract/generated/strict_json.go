package generated

import (
 "bytes"
 "encoding/json"
 "errors"
)

func strictDecode(data []byte,dst any)error{d:=json.NewDecoder(bytes.NewReader(data));d.DisallowUnknownFields();return d.Decode(dst)}
func(t *ToolFunctionCall)UnmarshalJSON(data []byte)error{type plain ToolFunctionCall;var v plain;if e:=strictDecode(data,&v);e!=nil{return e};var args map[string]json.RawMessage;if json.Unmarshal([]byte(v.Arguments),&args)!=nil||args==nil{return errors.New("tool arguments must be a JSON object")};*t=ToolFunctionCall(v);return nil}
func(t *ToolCall)UnmarshalJSON(data []byte)error{type plain ToolCall;var v plain;if e:=strictDecode(data,&v);e!=nil{return e};*t=ToolCall(v);return nil}
func(t *ToolFunction)UnmarshalJSON(data []byte)error{type plain ToolFunction;var v plain;if e:=strictDecode(data,&v);e!=nil{return e};*t=ToolFunction(v);return nil}
func(t *Tool)UnmarshalJSON(data []byte)error{type plain Tool;var v plain;if e:=strictDecode(data,&v);e!=nil{return e};*t=Tool(v);return nil}
func(t *ToolChoiceFunction)UnmarshalJSON(data []byte)error{type plain ToolChoiceFunction;var v plain;if e:=strictDecode(data,&v);e!=nil{return e};*t=ToolChoiceFunction(v);return nil}
func(t *ToolChoiceObject)UnmarshalJSON(data []byte)error{type plain ToolChoiceObject;var v plain;if e:=strictDecode(data,&v);e!=nil{return e};*t=ToolChoiceObject(v);return nil}
