# Compute-Appliance Feasibility and Delivery Plan

- **Status:** Superseded by ADR-0004; retained as historical feasibility evidence
- **Owner:** DevOps / integration owner
- **Former scope:** The investigated Debian 12 ARM64 private compute appliance with Iluvatar MR-V100 and CoreX CUDA-compatible runtime
- **Historical inputs:** `docs/adr/0003-compute-appliance-compose-gpu-runtime.md` and the recorded appliance evidence
- **Non-authorization:** This historical plan authorizes no remote login, package installation, source transfer, secret access, image publication, service startup, deployment, or appliance change.

## 1. Goal and non-goals

Prove, before adapting the full application, that the approved appliance can run the required single DMR/llama.cpp compatibility set as a least-privilege Compose-internal CoreX CUDA service while preserving the immutable MiniCPM5-2B GGUF and fixed inference settings.

This plan does not authorize daily development on the appliance, a host DMR, a LAN-published DMR, `privileged` containers, host networking, unrestricted device mounts, a Docker socket, CPU/Vulkan fallback, a second inference runtime, a reduced context, or changes to the approved model.

## 2. Environment ownership

| Environment | Permitted role | Prohibited role |
|---|---|---|
| Local Mac | Source-of-truth Git worktree; implementation, contracts, deterministic non-GPU tests, and artifact definition | CoreX acceptance evidence or manually maintained target runtime state |
| Compute appliance | Clean, fixed-revision Linux ARM64 build/integration and release-candidate verification | Ad-hoc source edits, uncommitted fixes, mutable dependency state, or acceptance evidence detached from a revision |

Every appliance attempt SHALL identify the source revision, dirty-patch digest if any, DMR and llama.cpp commit/binary hashes, CoreX version, MR-V100 PCI identity, image digests, resolved Compose digest, model digest, and evidence bundle identifier. Local Mac artifacts, Docker Desktop behavior, Metal evidence, and macOS binaries are not target evidence.

## 3. Gate sequence

### Gate A — CoreX container spike

**Purpose:** Establish vendor-supported, least-privilege GPU injection before any Mini-Inference service integration.

**Minimal scope:** An isolated throwaway Compose project with one GPU probe/build container. It has no API, web, database, controller, model lifecycle surface, secrets, production model package, published ports, host networking, privileged mode, Docker socket, or host root mount.

**Required observations:**

1. The vendor-supported integration identifies the exact CoreX devices, libraries, environment, and permissions granted to the container.
2. The container enumerates the MR-V100 through an approved vendor query path.
3. Native Linux ARM64 llama.cpp builds with the CoreX CUDA-compatible toolchain.
4. A controlled GGUF inference reports the CUDA backend and actual GPU-layer offload.
5. GPU memory and utilization change while inference executes.

**Pass:** All five observations are revision-bound and content-safe.

**Block:** Missing vendor container support, an unverifiable or over-broad device grant, build failure, device-enumeration failure, absent offload, or evidence that only the host—not the container—uses the GPU. A block permits no host-DMR, CPU, Vulkan, or parameter-reduction workaround.

### Gate B — Internal DMR spike

**Purpose:** Prove the approved Compose topology before adding application services.

**Minimal scope:** `controller` and GPU-enabled `dmr` on an internal `dmr` network. No public API, web, PostgreSQL, backup service, browser, or LAN-published DMR port.

**Required observations:**

1. The controller reaches only `http://dmr:12435` and fails closed for missing, malformed, localhost, arbitrary-host, or extra-path configuration.
2. Pinned lifecycle operations execute only status, load/warm, and unload; arbitrary caller-selected commands/arguments remain impossible.
3. Load/warm/status/unload use the approved model identity and retain CoreX GPU-layer offload.
4. DMR is unreachable from the LAN and has no host-gateway, reverse-proxy, host-network, Docker socket, or host-process control path.
5. DMR/llama.cpp/model runtime identity and the no-content privacy gate remain observable for later exact-build binding.

**Pass:** All operations and negative boundaries are proven from the target Compose topology.

**Block:** Any host path, exposed DMR listener, lifecycle ambiguity, missing identity, privacy-gate failure, or GPU-offload loss.

### Gate C — Full application adaptation

**Purpose:** Apply the already approved source changes only after Gates A and B pass.

**Local work:** Build the Linux/CoreX profile; add the internal DMR service; change the backend/frontend resource contract to `inference_memory_*` and `accelerator_*`; update compatibility-manifest schema and generated types; retain all existing API, queue, privacy, lifecycle, backup, and network contracts.

**Appliance work:** Build or consume only the fixed revision; create a new Linux ARM64/CoreX exact-build set; then run the PRD and QA target-environment suite.

**Required acceptance:** `131072` context, batch `2048`, temperature `0.8`, reasoning budget `-1`, cache reuse `256`, and keep-alive `-1` must all be accepted and exercised. OOM, rejection, ignored flags, missing offload, or a mismatched live identity blocks readiness and release.

## 4. Progress ledger

| Item | Status | Completion evidence |
|---|---|---|
| Product target authorization | Complete | Product owner decision; PRD and approved-decision register updated |
| Compose-internal CoreX architecture | Complete | ADR-0003 effective |
| Accelerator-neutral contract design | Complete | Backend, frontend, operations, and QA specifications aligned |
| Fixed-model/fixed-context rule | Complete | PRD, ADR-0003, operations, and QA retain `131072` / `2048` and fail-closed behavior |
| Gate A — CoreX container spike | Blocked | 2026-09-21 read-only host inspection: MR-V100/CoreX 4.4 is present, Docker Engine 29.6.1 exposes only `runc`, no Docker plugins or CDI specs exist, no CoreX container-toolkit package was found, and no `/dev/ix*` node was present. The PCI driver is `iluvatar`. Official TY1100 AI-framework documentation confirms supported Python/C++ framework development but does not define a Docker runtime, CDI spec, device contract, or llama.cpp/GGUF support. Vendor-supported container integration is absent or undiscoverable; no arbitrary device mount will be attempted. |
| Gate B — internal DMR spike | Pending | Depends on Gate A pass |
| Gate C — full adaptation and target acceptance | Pending | Depends on Gates A and B pass |

## 5. Handoff rule

This historical ledger is closed by ADR-0004. It records the evidence that no vendor-supported least-privilege Docker GPU integration was available; it authorizes no further gate or appliance work.