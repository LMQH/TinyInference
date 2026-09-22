# ADR-0003: Compute-Appliance Compose GPU Runtime

- **Status:** Superseded by ADR-0004
- **Date:** 2026-09-21
- **Owner:** Architecture owner
- **Decision authority:** Product owner’s explicit deployment authorization: the approved compute appliance is the formal target; prefer an isolated Compose-internal GPU service; generalize the accelerator status; preserve the fixed model and `131072` context.
- **Scope:** Mini-Inference MVP deployment on the approved Debian 12 ARM64 private compute appliance with Iluvatar MR-V100 and CoreX CUDA-compatible runtime.
- **Supersedes:** ADR-0002 §§7, 18 and 28 where they limit the runtime to the Apple Silicon Mac, require a host-provided DMR, or require host-loopback DMR exposure. ADR-0001 §§18, 32–33, 176–177 and 229 where they limit acceptance to the Mac, require Metal, assume a host DMR, or exclude Linux deployment. All unaffected invariants remain effective.

## Context

The approved appliance has a 32 GiB Iluvatar MR-V100 GPU and CoreX 4.4 CUDA-compatible runtime. It is the formal deployment and MVP verification target, not a future portability candidate. The old Mac-specific deployment depended on a project-built host DMR reachable through a Docker Desktop host route. That topology cannot prove Linux container-to-host loopback access while preserving DMR non-exposure, and it would make a host runtime a second operational plane.

DMR remains unauthenticated and is therefore a high-risk inference boundary. The approved product retains exactly one immutable MiniCPM5-2B GGUF model, one DMR/llama.cpp compatibility set, one active inference request, and the fixed context, batch, temperature, reasoning, cache, and keep-alive contracts.

## Decision

1. The approved Debian 12 ARM64 appliance is the sole formal deployment and MVP acceptance environment.
2. DMR and its GPU-enabled llama.cpp engine SHALL run as one Compose service on a dedicated internal DMR network. Only `api` and `controller` may attach to that network. DMR has no published host or LAN port, reverse-proxy route, host-gateway route, Docker socket, or host-process control path.
3. GPU access is mediated through a vendor-supported CoreX container integration. The integration must grant only the devices, libraries, and permissions required by the DMR service. `privileged`, unrestricted `/dev` exposure, host-networking, and arbitrary host mounts are prohibited.
4. llama.cpp SHALL use the CoreX CUDA-compatible backend. A failure to compile, enumerate the MR-V100, offload GPU layers, or preserve the fixed model configuration blocks deployment; it MUST NOT silently substitute Metal, Vulkan, CPU inference, another runner, or reduced context/batch/reasoning/cache settings.
5. The user-facing resource contract is accelerator-neutral. It reports the active backend and truthful status (`cuda`, `cpu`, `unavailable`, or `unknown`) with provenance. It MUST NOT label non-Metal execution as Metal, infer physical-host totals, or fabricate device utilization.
6. The exact DMR, llama.cpp, model, CoreX runtime, GPU integration, application images, and resolved Compose configuration form one compatibility set. API readiness remains fail-closed unless the live identity matches its manifest and the privacy gate passes.

## Consequences

- Compose gains a DMR service and an internal-only DMR network member; the public ingress, controller allowlist, single-model rule, queue semantics, content-minimization rules, and no-Docker-socket invariant remain unchanged.
- Vendor container GPU support is a mandatory deployment feasibility gate. A host DMR bridge exposure is not an implicit fallback; it requires a new ADR and explicit authorization.
- The console, backend contract, compatibility-manifest schema, resource provenance, technical specifications, and QA evidence must migrate from Metal-specific fields to accelerator-neutral fields as one contract change.
- Existing Mac runtime artifacts and Mac evidence cannot be reused. The appliance requires a new Linux ARM64 exact-build set and full target-environment lifecycle, inference, privacy, browser, network-boundary, backup/restore, and recovery evidence.
- `131072` context, batch `2048`, temperature `0.8`, reasoning budget `-1`, cache reuse `256`, and keep-alive `-1` remain mandatory. Any OOM, rejected flag, or incompatible CoreX execution is a release block, not an optimization opportunity.

## Alternatives considered

### Host-provided DMR via Docker bridge

Rejected for this approved deployment. It weakens the existing isolation model by introducing a container-to-host network boundary and requires a non-loopback listener or platform-specific forwarding proof. It may be reconsidered only through a new ADR with explicit product/security authorization.

### CPU fallback

Rejected as an automatic fallback. It would violate the approved hardware/backend contract and could hide a failed GPU integration. It may be used only as a separately authorized diagnostic mode and cannot satisfy MVP acceptance.

### Vulkan or another inference runtime

Rejected. Vulkan availability is not established on the appliance, and a second runtime is a product non-goal.

## Downstream design ownership

- **DevOps:** CoreX container integration, immutable Linux ARM64 build, Compose topology, compatibility manifest, rollout/rollback, and host boundary verification.
- **Backend:** accelerator-neutral resource/status contract, readiness identity validation, and safe error handling.
- **Frontend:** Chinese presentation of backend/status without Metal-specific wording.
- **QA:** appliance-specific evidence matrix, GPU offload proof, network non-exposure, and fixed-configuration failure gates.

No implementation detail in this ADR authorizes installation, deployment, secret access, host configuration changes, or modification of the approved appliance.