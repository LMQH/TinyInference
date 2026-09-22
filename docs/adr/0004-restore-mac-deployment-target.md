# ADR-0004: Restore Apple Silicon Mac Deployment Target

- **Status:** Effective
- **Date:** 2026-09-21
- **Owner:** Architecture owner
- **Decision authority:** Product owner’s explicit decision to make macOS the final deployment target and to permit iOS only as an API-client platform.
- **Scope:** Mini-Inference MVP deployment and acceptance.
- **Supersedes:** ADR-0003 in full. It restores the Mac-specific portions of ADR-0001 and ADR-0002 that ADR-0003 had superseded.

## Context

The CoreX appliance has a working MR-V100 host driver/runtime, but the required vendor-supported, least-privilege Docker GPU integration is not available or discoverable on the inspected appliance. Proceeding would require an unverified device/runtime contract or a host-runtime redesign. Neither is justified for this MVP.

The previously verified deployment design uses the current Apple Silicon Mac, a project-built compatible DMR, llama.cpp with Metal, and a loopback-only host runtime reached from the fixed Compose topology. That design preserves the approved immutable GGUF model and existing lifecycle, privacy, identity, and acceptance contracts.

## Decision

1. The current Apple Silicon Mac is the sole formal deployment and MVP acceptance environment.
2. The MVP returns to the project-built compatible DMR host runtime, bound only to host loopback and reached from the Compose application through the existing documented internal route. DMR remains unexposed to LAN clients.
3. Metal is the required inference acceleration backend for MVP acceptance. The prior CoreX/CUDA Compose-internal DMR requirement is removed.
4. iOS is an approved API-client platform only. An iOS client may call the authenticated private-LAN/VPN API; it does not host the service, run DMR, run the model locally, alter lifecycle authority, or expand the public API contract.
5. The compute-appliance feasibility investigation is retained as evidence, but it is no longer a deployment gate or delivery path.

## Consequences

- The Mac runtime build, runtime identity, privacy gate, host-loopback DMR boundary, Metal observability, and Mac acceptance evidence are again authoritative.
- All CoreX-specific Compose service, runtime identity, resource schema, and QA requirements introduced by ADR-0003 are superseded for MVP work.
- A future compute-appliance deployment requires a new product authorization and ADR after a vendor-supported container integration is available.
- iOS client work, if implemented, consumes the existing OpenAI-compatible API and does not create an iOS inference-server or local-model scope.

## Alternatives considered

### Continue compute-appliance deployment

Rejected. The required Docker-to-MR-V100 GPU contract is not available. Bypassing that gap with broad mounts, `privileged`, host networking, or an undocumented device contract would weaken the approved isolation boundary.

### Make iOS an inference host

Rejected. The MVP relies on Docker Compose, DMR lifecycle control, persistent metadata, backup/restore, and a continuously available private-LAN appliance. iOS remains a client platform only.

### Replace DMR/llama.cpp with an appliance-native framework

Rejected for MVP. It would replace the fixed GGUF/DMR/llama.cpp compatibility set and requires a separate product and architecture decision.