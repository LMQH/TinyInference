package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	g "mini-inference/services/controller/internal/contract/generated"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

var (
	errStateMismatch = errors.New("state_mismatch")
	errStatusExec    = errors.New("status_exec")
	errStatusParse   = errors.New("status_parse")
	errVersionExec   = errors.New("version_exec")
	errVersionParse  = errors.New("version_parse")
	errPSExec        = errors.New("ps_exec")
	errPSParse       = errors.New("ps_parse")
)

type Config struct{ Binary, RunnerHost, ModelRef, SourceSHA string }
type Adapter struct {
	cfg    Config
	client *http.Client
}

func New(c Config) *Adapter {
	return &Adapter{cfg: c, client: &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect rejected") }}}
}

type statusJSON struct {
	Running      *bool             `json:"running"`
	Backends     map[string]string `json:"backends"`
	Kind         string            `json:"kind"`
	Endpoint     string            `json:"endpoint"`
	EndpointHost string            `json:"endpointHost"`
}
type resourceJSON struct {
	SampledAt               string `json:"sampled_at"`
	UnifiedMemoryUsedBytes  *int64 `json:"unified_memory_used_bytes"`
	UnifiedMemoryTotalBytes *int64 `json:"unified_memory_total_bytes"`
	UnifiedMemorySource     string `json:"unified_memory_source"`
	DiskUsedBytes           *int64 `json:"disk_used_bytes"`
	DiskTotalBytes          *int64 `json:"disk_total_bytes"`
	DiskSource              string `json:"disk_source"`
	Metal                   string `json:"metal"`
	MetalSource             string `json:"metal_source"`
}

func (a *Adapter) Run(ctx context.Context, operation string) g.ControllerResult {
	res := g.ControllerResult{Operation: operation, Outcome: "failed", ObservedState: "unknown", ModelRef: a.cfg.ModelRef, SourceSHA256: a.cfg.SourceSHA, ObservedAt: time.Now().UTC(), Runner: g.Runner{Engine: "llama.cpp", KeepAlive: 0}, RuntimeResources: unavailableResources()}
	var err error
	switch operation {
	case "status":
		err = a.observe(ctx, &res)
	case "load":
		if _, err = a.execAction(ctx, "run", "--detach", a.cfg.ModelRef); err == nil {
			err = a.observe(ctx, &res)
			if err == nil && res.ObservedState != "loaded" {
				err = errStateMismatch
			}
		}
	case "unload":
		if _, err = a.execAction(ctx, "unload", a.cfg.ModelRef); err == nil {
			err = a.observe(ctx, &res)
			if err == nil && res.ObservedState != "unloaded" {
				err = errStateMismatch
			}
		}
	default:
		err = errors.New("invalid operation")
	}
	if err != nil {
		code := "cli_failed"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = "timeout"
			res.Outcome = "indeterminate"
		} else if errors.Is(err, errStateMismatch) {
			code = "state_mismatch"
			res.Outcome = "indeterminate"
		} else {
			switch {
			case errors.Is(err, errStatusExec):
				code = "status_exec"
			case errors.Is(err, errStatusParse):
				code = "status_parse"
			case errors.Is(err, errVersionExec):
				code = "version_exec"
			case errors.Is(err, errVersionParse):
				code = "version_parse"
			case errors.Is(err, errPSExec):
				code = "ps_exec"
			case errors.Is(err, errPSParse):
				code = "ps_parse"
			}
		}
		res.Error = &g.SafeError{Code: code, Message: safeMessage(code), Retryable: code == "timeout" || code == "cli_failed" || strings.HasSuffix(code, "_exec")}
		return res
	}
	res.Outcome = "succeeded"
	return res
}
func (a *Adapter) observe(ctx context.Context, res *g.ControllerResult) error {
	statusRaw, e := a.exec(ctx, "status", "--json")
	if e != nil {
		return errStatusExec
	}
	st, e := parseStatus(statusRaw)
	if e != nil {
		return errStatusParse
	}
	versionRaw, e := a.exec(ctx, "version")
	if e != nil {
		return errVersionExec
	}
	serverVersion, e := parseServerVersion(versionRaw)
	if e != nil {
		return errVersionParse
	}
	psRaw, e := a.exec(ctx, "ps")
	if e != nil {
		return errPSExec
	}
	loaded, keepAlive, e := parsePS(psRaw, a.cfg.ModelRef)
	if e != nil {
		if errors.Is(e, errStateMismatch) {
			return e
		}
		return errPSParse
	}
	engineVersion := st.Backends["llama.cpp"]
	if loaded && strings.EqualFold(strings.TrimSpace(engineVersion), "Not Installed") {
		return errStateMismatch
	}
	res.Runner = g.Runner{Available: true, DMRVersion: serverVersion, Engine: "llama.cpp", EngineVersion: engineVersion, KeepAlive: keepAlive}
	if loaded {
		res.ObservedState = "loaded"
	} else {
		res.ObservedState = "unloaded"
	}
	if resources, e := a.resources(ctx); e == nil {
		res.RuntimeResources = resources
	}
	return nil
}
func parseStatus(raw []byte) (statusJSON, error) {
	var st statusJSON
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&st); e != nil {
		return st, errors.New("invalid status output")
	}
	var extra any
	if e := d.Decode(&extra); !errors.Is(e, io.EOF) {
		return st, errors.New("invalid status output")
	}
	if st.Running == nil || !*st.Running || st.Backends == nil {
		return st, errors.New("invalid status output")
	}
	engine, ok := st.Backends["llama.cpp"]
	if !ok || strings.TrimSpace(engine) == "" {
		return st, errors.New("invalid status output")
	}
	return st, nil
}
func parsePS(raw []byte, modelRef string) (bool, int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 1024), 1<<20)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(scanner.Text(), "\r"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	if scanner.Err() != nil || len(lines) == 0 {
		return false, 0, errors.New("invalid ps output")
	}
	header := strings.Fields(lines[0])
	expected := []string{"MODEL", "NAME", "BACKEND", "MODE", "UNTIL"}
	if len(header) != len(expected) {
		return false, 0, errors.New("invalid ps output")
	}
	for i := range expected {
		if header[i] != expected[i] {
			return false, 0, errors.New("invalid ps output")
		}
	}
	canonical := "docker.io/" + modelRef
	loaded, keepAlive := false, 0
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return false, 0, errors.New("invalid ps output")
		}
		name := fields[0]
		if name != modelRef && name != canonical {
			return false, 0, errStateMismatch
		}
		if fields[1] != "llama.cpp" || loaded {
			return false, 0, errStateMismatch
		}
		loaded = true
		if len(fields) == 4 && fields[3] == "Forever" {
			keepAlive = -1
		}
	}
	return loaded, keepAlive, nil
}
func parseServerVersion(raw []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 1024), 1<<20)
	inServer := false
	version := ""
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(scanner.Text(), "\r"))
		if line == "" {
			continue
		}
		if line == "Server:" {
			inServer = true
			continue
		}
		if strings.HasSuffix(line, ":") {
			inServer = false
			continue
		}
		if inServer && strings.HasPrefix(line, "Version:") {
			fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "Version:")))
			if len(fields) != 1 || version != "" {
				return "", errors.New("invalid version output")
			}
			version = fields[0]
		}
	}
	if scanner.Err() != nil || version == "" || strings.ContainsAny(version, "/\\") {
		return "", errors.New("invalid version output")
	}
	return version, nil
}
func (a *Adapter) resources(ctx context.Context) (g.RuntimeResources, error) {
	fallback := unavailableResources()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.RunnerHost+"/mini-inference/v1/resources", nil)
	if e != nil {
		return fallback, e
	}
	resp, e := a.client.Do(req)
	if e != nil {
		return fallback, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fallback, errors.New("resource status")
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if e != nil || len(raw) > 64<<10 || duplicateJSONKey(raw) {
		return fallback, errors.New("invalid resource body")
	}
	var wire resourceJSON
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&wire); e != nil {
		return fallback, e
	}
	var extra any
	if e = d.Decode(&extra); !errors.Is(e, io.EOF) {
		return fallback, errors.New("resource trailing data")
	}
	sampled, e := time.Parse(time.RFC3339, wire.SampledAt)
	if e != nil || wire.UnifiedMemoryUsedBytes == nil || wire.DiskUsedBytes == nil ||
		*wire.UnifiedMemoryUsedBytes < 0 || *wire.DiskUsedBytes < 0 ||
		wire.UnifiedMemorySource != "dmr_process" || wire.DiskSource != "project_storage" ||
		wire.Metal != "enabled" || wire.MetalSource != "dmr_process" {
		return fallback, errors.New("invalid resources")
	}
	return g.RuntimeResources{
		SampledAt:               sampled,
		UnifiedMemoryUsedBytes:  wire.UnifiedMemoryUsedBytes,
		UnifiedMemoryTotalBytes: wire.UnifiedMemoryTotalBytes,
		UnifiedMemorySource:     wire.UnifiedMemorySource,
		DiskUsedBytes:           wire.DiskUsedBytes,
		DiskTotalBytes:          wire.DiskTotalBytes,
		DiskSource:              wire.DiskSource,
		Metal:                   wire.Metal,
		MetalSource:             wire.MetalSource,
	}, nil
}
func duplicateJSONKey(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() bool
	walk = func() bool {
		tok, e := d.Token()
		if e != nil {
			return true
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return false
		}
		if delim == '{' {
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return true
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return true
				}
				seen[name] = true
				if walk() {
					return true
				}
			}
			_, e = d.Token()
			return e != nil
		}
		if delim == '[' {
			for d.More() {
				if walk() {
					return true
				}
			}
			_, e = d.Token()
			return e != nil
		}
		return false
	}
	return walk()
}
func (a *Adapter) exec(ctx context.Context, args ...string) ([]byte, error) {
	return a.execMode(ctx, false, args...)
}
func (a *Adapter) execAction(ctx context.Context, args ...string) ([]byte, error) {
	return a.execMode(ctx, true, args...)
}
func (a *Adapter) execMode(ctx context.Context, allowEmpty bool, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, a.cfg.Binary, args...)
	cmd.Env = commandEnv(os.Environ(), a.cfg.RunnerHost)
	var out limitedBuffer
	var stderr limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	e := cmd.Run()
	if out.overflow || stderr.overflow {
		return nil, errors.New("cli output exceeded limit")
	}
	if e != nil {
		return nil, errors.New("cli failed")
	}
	return selectCLIOutput(out.Bytes(), stderr.Bytes(), allowEmpty)
}
func selectCLIOutput(stdout, stderr []byte, allowEmpty bool) ([]byte, error) {
	out := bytes.TrimSpace(stdout)
	errOut := bytes.TrimSpace(stderr)
	if len(out) > 0 && len(errOut) > 0 {
		return nil, errors.New("ambiguous cli output")
	}
	if len(out) > 0 {
		return bytes.Clone(out), nil
	}
	if len(errOut) > 0 {
		return bytes.Clone(errOut), nil
	}
	if allowEmpty {
		return []byte{}, nil
	}
	return nil, errors.New("empty cli output")
}
func commandEnv(base []string, runnerHost string) []string {
	env := make([]string, 0, len(base)+2)
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "MODEL_RUNNER_HOST" || key == "PATH" {
			continue
		}
		env = append(env, entry)
	}
	sort.Strings(env)
	env = append(env, "MODEL_RUNNER_HOST="+runnerHost, "PATH=/usr/local/bin")
	return env
}

type limitedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		b.overflow = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
func unavailableResources() g.RuntimeResources {
	now := time.Now().UTC()
	return g.RuntimeResources{SampledAt: now, UnifiedMemorySource: "unavailable", DiskSource: "unavailable", Metal: "unknown", MetalSource: "unavailable"}
}
func safeMessage(code string) string {
	switch code {
	case "timeout":
		return "Controller operation timed out."
	case "state_mismatch":
		return "Observed model state did not match the operation."
	case "status_exec":
		return "Controller status command failed."
	case "status_parse":
		return "Controller status result was invalid."
	case "version_exec":
		return "Controller version command failed."
	case "version_parse":
		return "Controller version result was invalid."
	case "ps_exec":
		return "Controller inventory command failed."
	case "ps_parse":
		return "Controller inventory result was invalid."
	default:
		return "Controller operation failed."
	}
}
