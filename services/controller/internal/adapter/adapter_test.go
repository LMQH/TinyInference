package adapter

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestParseStatusPinnedSchema(t *testing.T) {
	raw := []byte(`{"running":true,"backends":{"llama.cpp":"Not Installed","vllm":"Not Installed"},"kind":"docker-desktop","endpoint":"http://model-runner.docker.internal","endpointHost":"model-runner.docker.internal"}`)
	st, err := parseStatus(raw)
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}
	if got := st.Backends["llama.cpp"]; got != "Not Installed" {
		t.Fatalf("engine version = %q", got)
	}
}
func TestParseStatusRejectsUnknownOrStopped(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"running":true,"backends":{"llama.cpp":"v1"},"kind":"desktop","endpoint":"x","endpointHost":"x","version":"invented"}`),
		[]byte(`{"running":false,"backends":{"llama.cpp":"v1"},"kind":"desktop","endpoint":"x","endpointHost":"x"}`),
		[]byte(`{"running":true,"backends":{},"kind":"desktop","endpoint":"x","endpointHost":"x"}`),
	}
	for _, raw := range cases {
		if _, err := parseStatus(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestParsePSPinnedTable(t *testing.T) {
	const header = "MODEL NAME  BACKEND  MODE  UNTIL\n"
	loaded, keepAlive, err := parsePS([]byte(header), "local/minicpm5-2b:q4")
	if err != nil || loaded || keepAlive != 0 {
		t.Fatalf("empty table: loaded=%v keep_alive=%d err=%v", loaded, keepAlive, err)
	}
	for _, name := range []string{"local/minicpm5-2b:q4", "docker.io/local/minicpm5-2b:q4"} {
		loaded, keepAlive, err = parsePS([]byte(header+name+"  llama.cpp  detached  Forever\n"), "local/minicpm5-2b:q4")
		if err != nil || !loaded || keepAlive != -1 {
			t.Fatalf("approved row %q: loaded=%v keep_alive=%d err=%v", name, loaded, keepAlive, err)
		}
	}
}
func TestParsePSTimedRowsStayFailClosed(t *testing.T) {
	const header = "MODEL NAME  BACKEND  MODE  UNTIL\n"
	for _, until := range []string{"4 minutes", "2026-06-09T12:00:00Z", "forever"} {
		loaded, keepAlive, err := parsePS([]byte(header+"local/minicpm5-2b:q4 llama.cpp detached "+until+"\n"), "local/minicpm5-2b:q4")
		if err != nil || !loaded || keepAlive != 0 {
			t.Fatalf("UNTIL %q: loaded=%v keep_alive=%d err=%v", until, loaded, keepAlive, err)
		}
	}
}
func TestParsePSRejectsUnexpectedModelAndMalformedRows(t *testing.T) {
	const header = "MODEL NAME  BACKEND  MODE  UNTIL\n"
	cases := []string{
		"NAME BACKEND MODE UNTIL\n",
		header + "other/model:q4 llama.cpp detached Forever\n",
		header + "local/minicpm5-2b:q4 vllm detached Forever\n",
		header + "local/minicpm5-2b:q4 llama.cpp\n",
		header + "local/minicpm5-2b:q4 llama.cpp detached Forever\nlocal/minicpm5-2b:q4 llama.cpp detached Forever\n",
	}
	for _, raw := range cases {
		if _, _, err := parsePS([]byte(raw), "local/minicpm5-2b:q4"); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
func TestParseServerVersion(t *testing.T) {
	raw := []byte("Client:\n Version: v1.2.6\nServer:\n Version: v1.2.8\n")
	got, err := parseServerVersion(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1.2.8" {
		t.Fatalf("version = %q", got)
	}
	if _, err = parseServerVersion([]byte("Client:\n Version: v1.2.6\n")); err == nil {
		t.Fatal("accepted output without server version")
	}
}
func TestCommandEnvPreservesInheritedAndUniquelyOverrides(t *testing.T) {
	base := []string{"Z=value", "PATH=/bin", "HOME=/home/controller", "MODEL_RUNNER_HOST=old", "A=1", "PATH=/sbin", "MODEL_RUNNER_HOST=duplicate", "malformed"}
	got := commandEnv(base, "http://model-runner.docker.internal:12435")
	want := []string{"A=1", "HOME=/home/controller", "Z=value", "MODEL_RUNNER_HOST=http://model-runner.docker.internal:12435", "PATH=/usr/local/bin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %#v, want %#v", got, want)
	}
}
func TestSelectCLIOutputAcceptsExactlyOneStream(t *testing.T) {
	cases := []struct {
		name                 string
		stdout, stderr, want []byte
	}{
		{name: "stdout", stdout: []byte("  value\n"), want: []byte("value")},
		{name: "stderr", stderr: []byte("\nvalue  "), want: []byte("value")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectCLIOutput(tc.stdout, tc.stderr, false)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
		})
	}
}
func TestSelectCLIOutputRejectsAmbiguousAndEmpty(t *testing.T) {
	cases := []struct {
		name           string
		stdout, stderr []byte
	}{
		{name: "both", stdout: []byte("out"), stderr: []byte("err")},
		{name: "empty"},
		{name: "whitespace", stdout: []byte(" \n"), stderr: []byte("\t")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := selectCLIOutput(tc.stdout, tc.stderr, false); err == nil {
				t.Fatal("accepted invalid stream combination")
			}
		})
	}
}
func TestSelectCLIOutputActionAllowsEmptyButRejectsAmbiguous(t *testing.T) {
	got, err := selectCLIOutput(nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("output = %q", got)
	}
	if _, err = selectCLIOutput([]byte("out"), []byte("err"), true); err == nil {
		t.Fatal("accepted ambiguous action output")
	}
}
func TestResourcesStrictPinnedEndpoint(t *testing.T) {
	sampled := "2026-06-09T12:00:00Z"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/mini-inference/v1/resources" || r.URL.RawQuery != "" || r.ContentLength > 0 {
			t.Fatalf("request = %s %s content_length=%d", r.Method, r.URL.String(), r.ContentLength)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sampled_at":"` + sampled + `","unified_memory_used_bytes":10,"unified_memory_total_bytes":null,"unified_memory_source":"dmr_process","disk_used_bytes":30,"disk_total_bytes":null,"disk_source":"project_storage","metal":"enabled","metal_source":"dmr_process"}`))
	}))
	defer srv.Close()
	got, err := New(Config{RunnerHost: srv.URL}).resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantTime, _ := time.Parse(time.RFC3339, sampled)
	if !got.SampledAt.Equal(wantTime) || got.UnifiedMemoryUsedBytes == nil || *got.UnifiedMemoryUsedBytes != 10 || got.UnifiedMemoryTotalBytes != nil || got.DiskUsedBytes == nil || *got.DiskUsedBytes != 30 || got.DiskTotalBytes != nil || got.Metal != "enabled" || got.MetalSource != "dmr_process" {
		t.Fatalf("resources = %#v", got)
	}
}
func TestResourcesInvalidPayloadStaysUnavailable(t *testing.T) {
	cases := []string{
		`{"sampled_at":"bad","unified_memory_used_bytes":10,"unified_memory_total_bytes":20,"unified_memory_source":"dmr_process","disk_used_bytes":30,"disk_total_bytes":40,"disk_source":"project_storage","metal":"enabled","metal_source":"dmr_process"}`,
		`{"sampled_at":"2026-06-09T12:00:00Z","unified_memory_used_bytes":-1,"unified_memory_total_bytes":20,"unified_memory_source":"dmr_process","disk_used_bytes":30,"disk_total_bytes":40,"disk_source":"project_storage","metal":"enabled","metal_source":"dmr_process"}`,
		`{"sampled_at":"2026-06-09T12:00:00Z","unified_memory_used_bytes":10,"unified_memory_total_bytes":20,"unified_memory_source":"dmr_process","disk_used_bytes":30,"disk_total_bytes":40,"disk_source":"project_storage","metal":"enabled","metal_source":"dmr_process","extra":true}`,
		`{"sampled_at":"2026-06-09T12:00:00Z","sampled_at":"2026-06-09T12:00:01Z","unified_memory_used_bytes":10,"unified_memory_total_bytes":20,"unified_memory_source":"dmr_process","disk_used_bytes":30,"disk_total_bytes":40,"disk_source":"project_storage","metal":"enabled","metal_source":"dmr_process"}`,
	}
	for _, body := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		got, err := New(Config{RunnerHost: srv.URL}).resources(context.Background())
		srv.Close()
		if err == nil || got.UnifiedMemorySource != "unavailable" || got.DiskSource != "unavailable" || got.Metal != "unknown" || got.UnifiedMemoryUsedBytes != nil {
			t.Fatalf("body %s accepted as %#v err=%v", body, got, err)
		}
	}
}
