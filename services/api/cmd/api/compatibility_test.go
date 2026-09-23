package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyCompatibilityManifestRequiresBoundVerifiedEvidence(t *testing.T) {
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mini-inference/v1/identity" || r.URL.RawQuery != "" {
			t.Fatalf("identity request = %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"dmr_sha256":"` + strings64("b") + `","llama_sha256":"` + strings64("c") + `","model_digest":"` + strings64("d") + `"}`))
	}))
	defer identity.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "compatibility.json")
	compose := strings64("a")
	raw := validManifest(compose, "privacy-1")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	t.Setenv("COMPATIBILITY_MANIFEST_FILE", path)
	t.Setenv("COMPATIBILITY_MANIFEST_SHA256", hex.EncodeToString(sum[:]))
	t.Setenv("RESOLVED_COMPOSE_CONFIG_SHA256", compose)
	t.Setenv("AI_MODEL_URL", identity.URL)
	if err := verifyCompatibilityManifest(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RESOLVED_COMPOSE_CONFIG_SHA256", strings64("9"))
	if err := verifyCompatibilityManifest(); err == nil {
		t.Fatal("accepted mismatched resolved Compose hash")
	}
}

func TestVerifyCompatibilityManifestRejectsLiveRuntimeMismatch(t *testing.T) {
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"dmr_sha256":"` + strings64("9") + `","llama_sha256":"` + strings64("c") + `","model_digest":"` + strings64("d") + `"}`))
	}))
	defer identity.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "compatibility.json")
	compose := strings64("a")
	raw := validManifest(compose, "privacy-1")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	t.Setenv("COMPATIBILITY_MANIFEST_FILE", path)
	t.Setenv("COMPATIBILITY_MANIFEST_SHA256", hex.EncodeToString(sum[:]))
	t.Setenv("RESOLVED_COMPOSE_CONFIG_SHA256", compose)
	t.Setenv("AI_MODEL_URL", identity.URL)
	if err := verifyCompatibilityManifest(); err == nil {
		t.Fatal("accepted mismatched live DMR identity")
	}
}

func TestVerifyCompatibilityManifestRejectsUnresolvedPrivacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compatibility.json")
	compose := strings64("a")
	raw := validManifest(compose, "different")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	t.Setenv("COMPATIBILITY_MANIFEST_FILE", path)
	t.Setenv("COMPATIBILITY_MANIFEST_SHA256", hex.EncodeToString(sum[:]))
	t.Setenv("RESOLVED_COMPOSE_CONFIG_SHA256", compose)
	if err := verifyCompatibilityManifest(); err == nil {
		t.Fatal("accepted unbound privacy evidence")
	}
}

func TestVerifyCompatibilityManifestCandidateRequiresExplicitMode(t *testing.T) {
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"dmr_sha256":"` + strings64("b") + `","llama_sha256":"` + strings64("c") + `","model_digest":"` + strings64("d") + `"}`))
	}))
	defer identity.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "compatibility.json")
	compose := strings64("a")
	raw := candidateManifest(compose)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	t.Setenv("COMPATIBILITY_MANIFEST_FILE", path)
	t.Setenv("COMPATIBILITY_MANIFEST_SHA256", hex.EncodeToString(sum[:]))
	t.Setenv("RESOLVED_COMPOSE_CONFIG_SHA256", compose)
	t.Setenv("AI_MODEL_URL", identity.URL)
	if err := verifyCompatibilityManifest(); err == nil {
		t.Fatal("candidate started without explicit mode")
	}
	t.Setenv("VERIFICATION_MODE", "candidate")
	if err := verifyCompatibilityManifest(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VERIFICATION_MODE", "unexpected")
	if err := verifyCompatibilityManifest(); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

func validManifest(compose, evidence string) []byte {
	builds := "model:" + strings64("d") + ";api:" + strings64("e") + ";web:" + strings64("f") + ";controller:" + strings64("1") + ";jobs:" + strings64("2") + ";dmr:" + strings64("b") + ";llama:" + strings64("c")
	return []byte(`{"release":{"status":"verified","target_environment":"apple-silicon-mac-private-lan"},"model":{"oci_digest":"sha256:` + strings64("d") + `"},"images":{"api":{"digest":"sha256:` + strings64("e") + `"},"web":{"digest":"sha256:` + strings64("f") + `"},"controller":{"digest":"sha256:` + strings64("1") + `"},"jobs":{"digest":"sha256:` + strings64("2") + `"}},"database":{"migration_version":7},"resolved_compose_config_sha256":"` + compose + `","privacy_gate":{"status":"passed","evidence_id":"privacy-1","exact_build_set":"` + builds + `","passed":true},"evidence":{"privacy":"` + evidence + `"},"unresolved_fields":[]}`)
}

func candidateManifest(compose string) []byte {
	builds := "model:" + strings64("d") + ";api:" + strings64("e") + ";web:" + strings64("f") + ";controller:" + strings64("1") + ";jobs:" + strings64("2") + ";dmr:" + strings64("b") + ";llama:" + strings64("c")
	return []byte(`{"release":{"status":"candidate","target_environment":"apple-silicon-mac-local-candidate"},"model":{"oci_digest":"sha256:` + strings64("d") + `"},"images":{"api":{"digest":"sha256:` + strings64("e") + `"},"web":{"digest":"sha256:` + strings64("f") + `"},"controller":{"digest":"sha256:` + strings64("1") + `"},"jobs":{"digest":"sha256:` + strings64("2") + `"}},"database":{"migration_version":7},"resolved_compose_config_sha256":"` + compose + `","privacy_gate":{"status":"unresolved","evidence_id":null,"exact_build_set":"` + builds + `","passed":false},"evidence":{"privacy":null},"unresolved_fields":["privacy_gate"]}`)
}

func strings64(char string) string { return strings.Repeat(char, 64) }
