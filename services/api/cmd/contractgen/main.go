package main

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "go/format"
 "os"
 "path/filepath"
 "sort"
)

type manifest struct { SchemaVersion int `json:"schema_version"`; Sources map[string]string `json:"sources"` }
var allowed = map[string]bool{"public.openapi.yaml":true,"admin.openapi.yaml":true,"admin-events.schema.json":true,"controller.openapi.yaml":true,"config.schema.json":true}

func main() {
 contracts:=flag.String("contracts","contracts","sealed contract directory")
 apiOut:=flag.String("api-out","internal/contract/generated","API generated directory")
 controllerOut:=flag.String("controller-out","../controller/internal/contract/generated","controller generated directory")
 flag.Parse()
 if err:=run(*contracts,*apiOut,*controllerOut); err!=nil { fmt.Fprintln(os.Stderr,"contractgen:",err); os.Exit(1) }
}
func run(contracts,apiOut,controllerOut string) error {
 raw,err:=os.ReadFile(filepath.Join(contracts,"contract-manifest.json")); if err!=nil{return err}
 var m manifest; if json.Unmarshal(raw,&m)!=nil||m.SchemaVersion!=1{return errors.New("invalid contract manifest")}
 if len(m.Sources)!=len(allowed){return errors.New("manifest source set is not sealed")}
 names:=make([]string,0,len(m.Sources)); for n:=range m.Sources{names=append(names,n)}; sort.Strings(names)
 for _,n:=range names { if !allowed[n]{return fmt.Errorf("unapproved source %q",n)}; b,e:=os.ReadFile(filepath.Join(contracts,n));if e!=nil{return e};h:=sha256.Sum256(b);if hex.EncodeToString(h[:])!=m.Sources[n]{return fmt.Errorf("hash mismatch for %s",n)} }
 for _,target:=range []string{filepath.Join(apiOut,"types.go"),filepath.Join(controllerOut,"types.go")} { b,e:=os.ReadFile(target);if e!=nil{return e}; formatted,e:=format.Source(b);if e!=nil{return fmt.Errorf("format %s: %w",target,e)};if e=os.WriteFile(target,formatted,0644);e!=nil{return e} }
 return nil
}
