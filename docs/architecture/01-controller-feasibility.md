# Controller Feasibility Research

Status: Evidence
Date: 2026-09-21

## Verdict

FEASIBLE — but only with an explicit documented host override (MODEL_RUNNER_HOST), not via the bare mounted Engine socket. A linux/arm64 docker-model plugin build exists and is containerizable, and its four lifecycle verbs (run --detach, unload, ps, status) transport via direct HTTP to the Docker Model Runner daemon's own endpoint paths once the daemon is reachable. A plain /var/run/docker.sock mount does NOT provide that transport: Moby does not proxy DMR's /engines/*, /logs, /models paths, and the plugin's default Moby kind points at a local controller port (http://localhost:<DefaultControllerPortMoby>) that does not exist inside the sidecar. Setting MODEL_RUNNER_HOST makes the plugin use the MobyManual kind with http.DefaultClient and issue the exact lifecycle calls directly to the daemon (e.g. http://host.docker.internal:12434 on Docker Desktop with host TCP enabled, or model-runner.docker.internal). No undocumented HTTP endpoints are required: Preload/Unload/PS/Status/Logs are the plugin's own client calls to DMR's server routes.

## Evidence

[
  "Binary target: cmd/cli/Makefile release target builds 'CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/linux-arm64/docker-model .' and GOOS=linux GOARCH=amd64 — linux/arm64 is an official release artifact; base image golang:1.25-alpine in cmd/cli/Dockerfile.",
  "Transport base + client selection: cmd/cli/desktop/context.go DetectContext() returns ModelRunnerContext{urlPrefix, client, openaiPathPrefix}. Desktop kind (macOS/Win host or WSL) -> urlPrefix 'http://localhost' + inference.ExperimentalEndpointsPrefix and a *http.Client whose Transport.DialContext dials via dockerClient.Dialer() (the Docker engine connection). Moby kind -> 'http://localhost:' + standalone.DefaultControllerPortMoby with http.DefaultClient (direct TCP). MobyManual kind -> normalized MODEL_RUNNER_HOST with http.DefaultClient (direct TCP).",
  "Path constants: pkg/inference defines ExperimentalEndpointsPrefix='/exp/vDD4.40', InferencePrefix='/engines', ModelsPrefix='/models' (confirmed via pkg.go.dev/github.com/docker/model-runner/pkg/inference and Docker docs api-reference; ExperimentalEndpointsPrefix does not apply to model-runner.docker.internal).",
  "Lifecycle verb -> HTTP mapping in cmd/cli/desktop/desktop.go (Client methods, all routed through c.modelRunner.URL(path) + c.modelRunner.Client()): Preload (= docker model run --detach) POSTs openaiPathPrefix('/engines/v1')+'/chat/completions' with header X-Preload-Only=true; Unload() POSTs '/engines/unload' (UnloadRequest{All,Backend,Models}); PS() GETs '/engines/ps' -> []BackendStatus; Status() GETs '/engines/status' (via InferencePrefix+... ). Logs() GETs '/logs'.",
  "Command wiring: commands/run.go (uses Preload via desktopClient), commands/ps.go (desktopClient.PS()), commands/unload.go (desktopClient.Unload), commands/status.go (desktopClient.Status()). All are DMR-daemon HTTP calls, not Docker Engine API calls through /var/run/docker.sock.",
  "Documented in-container path: Docker docs api-reference states containers reach DMR at http://model-runner.docker.internal (newer /v1 or older /engines/v1 base); host TCP on localhost:12434 requires 'docker desktop enable model-runner --tcp'; the /exp/vDD4.40 path is the Docker Desktop host Unix-socket bridge (~/.docker/run/docker.sock), an implementation detail not available from an Engine socket mounted inside a Linux container."
]

## Proof experiment

[
  "After Docker starts, run inside the linux/arm64 sidecar that has /var/run/docker.sock mounted AND DMR host TCP enabled: 'docker-model run --detach ai/smollm2 && docker-model ps && docker-model unload ai/smollm2 && docker-model status'.",
  "Precondition 1 (export to force direct-daemon transport): export MODEL_RUNNER_HOST=http://host.docker.internal:12434 (Docker Desktop with host TCP) or export MODEL_RUNNER_HOST=http://model-runner.docker.internal:12434. Pass. Otherwise, expect the plugin to auto-detect kind=Moby/Desktop and either dial a dead localhost:<DefaultControllerPortMoby> or fail to find the Desktop bridge -> 'connection refused' / 503 ErrServiceUnavailable, proving the bare socket does not serve DMR lifecycle paths.",
  "Assertion A (transport is daemon HTTP, not Engine API): with MODEL_RUNNER_HOST set, 'uname' inside sidecar is linux/arm64; run 'docker-model ps' and confirm it returns [] via GET {MODEL_RUNNER_HOST}/engines/ps. Corroborate: 'curl http://host.docker.internal:12434/engines/v1/models' returns the same model list the CLI sees; 'docker-model unload --all' prints 'Unloaded N model(s)' and a subsequent 'ps' shows empty.",
  "Assertion B (socket-only is insufficient): with MODEL_RUNNER_HOST unset and only /var/run/docker.sock mounted, 'docker-model run --detach ai/smollm2' must fail to reach the daemon (no /engines/* route served by Moby socket) — confirming the requirement for the host override rather than the socket.",
  "Document the exact DMR/llama.cpp versions ('docker-model version') alongside the result as the reproducibility baseline."
]

## Alternatives

[
  "Documented HTTP-only control (no daemon lifecycle): over the documented DMR REST surface a container can GET /engines/v1/models and /models, POST /models/create, DELETE /models/{ns}/{name}; inference via /engines/v1/chat/completions. This satisfies load-on-demand + explicit delete but NOT explicit unload/status/ps/logs.",
  "Host-side privileged controller: keep load/warm/unload/status as a host-launched wrapper invoking the HOST's docker-model CLI plugin (which has the working Docker Desktop socket), exposing only fixed operations to the internal console — the defense-in-depth pattern in the prior DMR research.",
  "Direct daemon TCP from the sidecar (recommended to avoid broad socket authority): enable host TCP ('docker desktop enable model-runner --tcp 12434') and point the sidecar's embedded/plugin docker-model at MODEL_RUNNER_HOST — narrow, documented, no Engine-daemon control. Note DMR API is unauthenticated, so keep the daemon host path internal only.",
  "Wait for/verify a stable public DMR lifecycle HTTP contract: currently unload/ps/status/logs exist only as plugin->daemon routes, not a documented external REST API; do not treat /engines/unload or /logs as stable product contracts without further product/security decision."
]

## Sources

[
  "https://github.com/docker/model-runner/blob/main/cmd/cli/Makefile (release: CGO_ENABLED=0 GOOS=linux GOARCH=arm64 build of docker-model plugin)",
  "https://github.com/docker/model-runner/blob/main/cmd/cli/Dockerfile (golang:1.25-alpine build base, ENV CGO_ENABLED=0)",
  "https://github.com/docker/model-runner/blob/main/cmd/cli/desktop/context.go (DetectContext, ModelRunnerContext, Desktop/Moby/MobyManual kind URL/client selection; Desktop uses dockerClient.Dialer() against http://localhost/exp/vDD4.40)",
  "https://github.com/docker/model-runner/blob/main/cmd/cli/desktop/desktop.go (Client.Preload->X-Preload-Only chat/completions; Client.Unload->POST /engines/unload; Client.PS->GET /engines/ps; Client.Status->GET /engines/status; Client.Logs->GET /logs)",
  "https://github.com/docker/model-runner/blob/main/cmd/cli/commands/run.go, ps.go, unload.go, status.go (cobra command wiring to the desktop.Client calls)",
  "https://pkg.go.dev/github.com/docker/model-runner/pkg/inference (ExperimentalEndpointsPrefix='/exp/vDD4.40', InferencePrefix='/engines', ModelsPrefix='/models'; prefix not applied to model-runner.docker.internal)",
  "https://docs.docker.com/ai/model-runner/api-reference/ (containers use http://model-runner.docker.internal, host TCP localhost:12434 via 'docker desktop enable model-runner --tcp', Docker Desktop host socket ~/.docker/run/docker.sock with /exp/vDD4.40 path, Engine host-gateway; DMR API unauthenticated)",
  "https://docs.docker.com/reference/cli/docker/model/run/ and /unload/, /status/, /ps/ (host CLI plugin semantics; no external REST equivalent documented for unload/status/ps/logs)"
]
