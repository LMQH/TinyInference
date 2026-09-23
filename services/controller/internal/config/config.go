package config

import (
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
)

const AllowedRunnerHost = "http://model-runner.docker.internal:12435"
const AllowedModelRef = "local/minicpm5-2b:q4_k_m-ec2d58016400"
const AllowedSHA = "ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd"

type Config struct{ ListenAddr, DatabaseURL, RunnerHost, ModelRef, SourceSHA, Binary string }

func Load() (Config, error) {
	c := Config{ListenAddr: os.Getenv("LISTEN_ADDR"), RunnerHost: os.Getenv("MODEL_RUNNER_HOST"), ModelRef: os.Getenv("MODEL_ARTIFACT_REF"), SourceSHA: os.Getenv("MODEL_SOURCE_SHA256"), Binary: os.Getenv("DOCKER_MODEL_BIN")}
	if c.ListenAddr == "" {
		c.ListenAddr = ":9090"
	}
	if c.Binary == "" {
		c.Binary = "/usr/local/bin/docker-model"
	}
	if e := candidateGuard(); e != nil {
		return c, e
	}
	b, e := os.ReadFile(os.Getenv("DATABASE_URL_FILE"))
	if e != nil {
		return c, errors.New("controller database credential unavailable")
	}
	c.DatabaseURL = strings.TrimSpace(string(b))
	u, e := url.Parse(c.RunnerHost)
	if e != nil || u.String() != AllowedRunnerHost || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return c, errors.New("invalid MODEL_RUNNER_HOST")
	}
	if c.ModelRef != AllowedModelRef || c.SourceSHA != AllowedSHA {
		return c, errors.New("invalid model identity")
	}
	if c.Binary != "/usr/local/bin/docker-model" {
		return c, errors.New("invalid docker-model binary")
	}
	return c, nil
}
func candidateGuard() error {
	switch os.Getenv("VERIFICATION_MODE") {
	case "":
		return nil
	case "candidate":
		expected := os.Getenv("CANDIDATE_MANIFEST_SHA256")
		if len(expected) != 64 {
			return errors.New("invalid candidate identity")
		}
		if _, e := hex.DecodeString(expected); e != nil {
			return errors.New("invalid candidate identity")
		}
		path := os.Getenv("CANDIDATE_GUARD_FILE")
		if path != "/run/candidate.guard" {
			return errors.New("invalid candidate guard path")
		}
		return checkCandidateGuard(path, expected)
	default:
		return errors.New("invalid verification mode")
	}
}
func checkCandidateGuard(path, expected string) error {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() {
		return errors.New("candidate guard unavailable")
	}
	b, e := os.ReadFile(path)
	if e != nil || strings.TrimSpace(string(b)) != expected {
		return errors.New("candidate guard mismatch")
	}
	return nil
}
