package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunnerHostFailsClosed(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "dsn")
	if e := os.WriteFile(dsn, []byte("postgres://controller"), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("DATABASE_URL_FILE", dsn)
	t.Setenv("MODEL_ARTIFACT_REF", AllowedModelRef)
	t.Setenv("MODEL_SOURCE_SHA256", AllowedSHA)
	t.Setenv("DOCKER_MODEL_BIN", "/usr/local/bin/docker-model")
	for _, host := range []string{"", "http://model-runner.docker.internal:12434", "http://localhost:12435", "http://model-runner.docker.internal:12435/path", "http://evil.internal:12435"} {
		t.Setenv("MODEL_RUNNER_HOST", host)
		if _, e := Load(); e == nil {
			t.Fatalf("accepted %q", host)
		}
	}
	t.Setenv("MODEL_RUNNER_HOST", AllowedRunnerHost)
	if _, e := Load(); e != nil {
		t.Fatalf("rejected allowed origin: %v", e)
	}
}
func TestCandidateGuardFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "guard")
	expected := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if e := os.WriteFile(path, []byte(expected+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := checkCandidateGuard(path, expected); e != nil {
		t.Fatal(e)
	}
	if e := checkCandidateGuard(path, "different"); e == nil {
		t.Fatal("accepted mismatched guard")
	}
	link := filepath.Join(dir, "link")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if e := checkCandidateGuard(link, expected); e == nil {
		t.Fatal("accepted guard symlink")
	}
}
