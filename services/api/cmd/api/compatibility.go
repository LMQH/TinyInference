package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type compatibilityManifest struct {
	Release struct {
		Status string `json:"status"`
	} `json:"release"`
	Model struct {
		OCIDigest string `json:"oci_digest"`
	} `json:"model"`
	Images map[string]struct {
		Digest string `json:"digest"`
	} `json:"images"`
	Database struct {
		MigrationVersion int `json:"migration_version"`
	} `json:"database"`
	ResolvedComposeConfigSHA256 string `json:"resolved_compose_config_sha256"`
	PrivacyGate                 struct {
		Status        string  `json:"status"`
		EvidenceID    *string `json:"evidence_id"`
		ExactBuildSet *string `json:"exact_build_set"`
		Passed        bool    `json:"passed"`
	} `json:"privacy_gate"`
	Evidence struct {
		Privacy *string `json:"privacy"`
	} `json:"evidence"`
	UnresolvedFields *[]string `json:"unresolved_fields"`
}

type runtimeIdentity struct {
	DMRSHA256   string `json:"dmr_sha256"`
	LlamaSHA256 string `json:"llama_sha256"`
	ModelDigest string `json:"model_digest"`
}

func verifyCompatibilityManifest() error {
	path, wantHash, wantCompose := os.Getenv("COMPATIBILITY_MANIFEST_FILE"), os.Getenv("COMPATIBILITY_MANIFEST_SHA256"), os.Getenv("RESOLVED_COMPOSE_CONFIG_SHA256")
	if path == "" || !validSHA256(wantHash) || !validSHA256(wantCompose) {
		return errors.New("invalid compatibility identity")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 1<<20 {
		return errors.New("invalid compatibility manifest")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return errors.New("invalid compatibility manifest")
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != wantHash {
		return errors.New("compatibility hash mismatch")
	}
	var manifest compatibilityManifest
	if json.Unmarshal(raw, &manifest) != nil {
		return errors.New("invalid compatibility manifest")
	}
	if manifest.Release.Status != "verified" || manifest.Database.MigrationVersion != 6 || manifest.ResolvedComposeConfigSHA256 != wantCompose || manifest.UnresolvedFields == nil || len(*manifest.UnresolvedFields) != 0 {
		return errors.New("compatibility unresolved")
	}
	if manifest.PrivacyGate.Status != "passed" || !manifest.PrivacyGate.Passed || manifest.PrivacyGate.EvidenceID == nil || *manifest.PrivacyGate.EvidenceID == "" || manifest.PrivacyGate.ExactBuildSet == nil || manifest.Evidence.Privacy == nil || *manifest.Evidence.Privacy != *manifest.PrivacyGate.EvidenceID {
		return errors.New("privacy gate unresolved")
	}
	builds, err := parseExactBuildSet(*manifest.PrivacyGate.ExactBuildSet)
	if err != nil || builds["model"] != strings.TrimPrefix(manifest.Model.OCIDigest, "sha256:") {
		return errors.New("invalid exact build set")
	}
	for _, name := range []string{"api", "web", "controller", "jobs"} {
		image, ok := manifest.Images[name]
		if !ok || builds[name] != strings.TrimPrefix(image.Digest, "sha256:") {
			return errors.New("invalid exact build set")
		}
	}
	identity, err := fetchRuntimeIdentity(os.Getenv("AI_MODEL_URL"))
	if err != nil || identity.DMRSHA256 != builds["dmr"] || identity.LlamaSHA256 != builds["llama"] || identity.ModelDigest != builds["model"] {
		return errors.New("runtime identity mismatch")
	}
	return nil
}

func parseExactBuildSet(raw string) (map[string]string, error) {
	expected := map[string]bool{"model": true, "api": true, "web": true, "controller": true, "jobs": true, "dmr": true, "llama": true}
	result := make(map[string]string, len(expected))
	for _, item := range strings.Split(raw, ";") {
		name, digest, ok := strings.Cut(item, ":")
		if !ok || !expected[name] || !validSHA256(digest) {
			return nil, errors.New("invalid exact build set")
		}
		if _, exists := result[name]; exists {
			return nil, errors.New("duplicate exact build")
		}
		result[name] = digest
	}
	if len(result) != len(expected) {
		return nil, errors.New("incomplete exact build set")
	}
	return result, nil
}

func fetchRuntimeIdentity(rawBase string) (runtimeIdentity, error) {
	var identity runtimeIdentity
	base, err := url.Parse(rawBase)
	if err != nil || base.Scheme != "http" || base.Host == "" || base.RawQuery != "" || base.Fragment != "" {
		return identity, errors.New("invalid runtime URL")
	}
	base.Path = strings.TrimSuffix(base.Path, "/") + "/mini-inference/v1/identity"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return identity, err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect rejected") }}
	resp, err := client.Do(req)
	if err != nil {
		return identity, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return identity, errors.New("runtime identity unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (8<<10)+1))
	if err != nil || len(raw) > 8<<10 {
		return identity, errors.New("invalid runtime identity")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&identity); err != nil {
		return identity, errors.New("invalid runtime identity")
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) || !validSHA256(identity.DMRSHA256) || !validSHA256(identity.LlamaSHA256) || !validSHA256(identity.ModelDigest) {
		return identity, errors.New("invalid runtime identity")
	}
	return identity, nil
}

func validSHA256(raw string) bool {
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}
