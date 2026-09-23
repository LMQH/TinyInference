# ADR-0003: Apple Silicon Mac and iOS Client Target

- **Status:** Effective
- **Date:** 2026-09-23
- **Owner:** Architecture owner
- **Decision authority:** Product owner’s explicit decision to use macOS and iOS as the product targets
- **Scope:** Mini-Inference deployment, acceptance, and client platform boundary

## Context

Mini-Inference needs one unambiguous deployment and acceptance environment. The existing implementation uses an Apple Silicon Mac, a project-built compatible DMR, llama.cpp with Metal, and a loopback-only host runtime reached from the fixed Compose topology. iOS participates as a private-LAN/VPN API client.

## Decision

1. The current Apple Silicon Mac is the sole inference-service deployment and MVP acceptance environment.
2. The project-built compatible DMR runs on the Mac host, binds only to host loopback, and is reached from the Compose application through the documented internal route. DMR remains unexposed to LAN clients.
3. Metal is the required inference acceleration backend for MVP acceptance.
4. iOS is an approved API-client platform. An iOS client may call the authenticated private-LAN/VPN API; it does not host the service, run DMR, run the model locally, alter lifecycle authority, or expand the public API contract.
5. Deployment, runtime, QA, and compatibility work must target this macOS service and iOS client boundary only.

## Consequences

- Mac runtime build, runtime identity, privacy gate, host-loopback DMR boundary, Metal observability, and Mac acceptance evidence are authoritative.
- The Compose topology, controller allowlist, single-model rule, queue semantics, content-minimization rules, and no-Docker-socket invariant remain unchanged.
- iOS client work consumes the existing authenticated OpenAI-compatible API and does not create a second inference runtime or service deployment.
- Any future target-platform expansion requires a new product authorization and ADR.

## Alternatives considered

### Make iOS an inference host

Rejected. The product relies on Docker Compose, DMR lifecycle control, persistent metadata, backup/restore, and a continuously available private-LAN Mac service. iOS remains a client platform only.

### Replace DMR/llama.cpp

Rejected for the MVP. It would replace the fixed GGUF/DMR/llama.cpp compatibility set and requires a separate product and architecture decision.
