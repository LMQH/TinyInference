# ADR-0002: Project-Built Compatible DMR Runtime

- **Status:** Effective
- **Date:** 2026-09-21
- **Owner:** Implementation owner
- **Decision authority:** User continuation authorization in the delivery session after the compatible-runtime replacement, its host boundary, and its verification status were reported
- **Scope:** Mini-Inference MVP on the approved Apple Silicon Mac
- **Supersedes:** ADR-0001 §3.6 item 2 and its prohibition on a project-managed host DMR; ADR-0001 §3.11 statements that independent llama.cpp pinning is unavailable

## Context

The Docker Desktop-bundled DMR compatibility set could not satisfy two approved contracts simultaneously: authoritative `reasoning_tokens` from llama.cpp and proof that no request/response body recorder retained prohibited content. Treating those gaps as application behavior would either fabricate token accounting or weaken the privacy gate. A second inference runtime is prohibited.

The replacement must preserve the established architecture: DMR with llama.cpp remains the sole inference runtime; the application services remain in the one Compose topology; the controller continues to use the pinned Docker Model CLI and explicit `MODEL_RUNNER_HOST`; no component receives a Docker Engine socket; and the unauthenticated runtime is unreachable from the LAN.

## Decision

For the MVP Mac, the host-provided runtime is a project-built, project-pinned DMR at commit `ca5782d0408d33e579144f7f9a6e1cd2e65cc351` with llama.cpp at commit `72874f559c598b8f89fbb24864868337cf5afb4c`. The repository patch disables DMR request recording, binds DMR only to `127.0.0.1:12435`, exposes content-free resource provenance, exposes a read-only runtime identity endpoint, and applies the reviewed reasoning-token accounting patch to llama.cpp.

This is a replacement for the Docker Desktop-managed DMR, not an additional runtime. Exactly one DMR process, one llama.cpp engine binary, and one approved model package may exist in the compatibility set.

The runtime is fail-closed:

1. `ops/runtime/start.sh` verifies the approved GGUF, build manifest, DMR binary, and llama.cpp binary before startup.
2. An existing listener is accepted only when its PID file identifies a live process whose command is the exact managed DMR executable and fixed arguments. An arbitrary responder on port `12435` is rejected.
3. The live identity endpoint hashes the executing DMR and configured llama.cpp binaries and returns the sole packaged model digest.
4. API readiness parses the complete seven-artifact `privacy_gate.exact_build_set`, binds application image digests to the compatibility manifest, and compares the live DMR, llama.cpp, and model identities. Missing, malformed, duplicate, or mismatched identity blocks startup.
5. DMR remains loopback-only. Compose containers reach it through `model-runner.docker.internal:12435`; the port is never published or proxied to the LAN.
6. Lifecycle status/load/unload still cross only the fixed controller contract and pinned Docker Model CLI. Application code does not call undocumented lifecycle HTTP routes.

Resource totals that cannot be attributed to the DMR process or project-managed storage are reported as unavailable (`null`), not relabeled physical-host or filesystem totals.

## Consequences

- The exact DMR and llama.cpp commits and binary SHA-256 values become compatibility-manifest identities and require revalidation after any rebuild.
- Runtime build and startup are currently Darwin ARM64-specific. Future Linux support remains unauthorized and requires a separately verified host-runtime implementation while preserving the same application contracts.
- A local operator with permission to replace project artifacts remains inside the accepted host trust boundary. Network clients and application containers gain no new authority.
- Docker Desktop remains the container engine for the Compose application, but its bundled DMR is not part of the accepted inference compatibility set.
- Loss of the identity endpoint, PID ownership, exact artifact match, controller observation, or privacy evidence makes the API unavailable rather than selecting a fallback.
