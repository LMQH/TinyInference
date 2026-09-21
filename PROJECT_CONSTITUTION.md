# Mini-Inference Project Constitution

Status: Effective
Authority: User-approved project constraints

## Mission

Deliver a private-network, single-model inference appliance for MiniCPM5-2B with an authenticated OpenAI-compatible text API and a Chinese operations console.

## Non-negotiable invariants

1. Runtime isolation: application services run only through Docker Compose; inference runs through Docker Model Runner using llama.cpp.
2. Repository boundary: project artifacts, bind mounts, and backups remain under this repository whenever Docker does not own the storage.
3. Single execution: at most one inference request reaches the model at a time.
4. Private control plane: Docker Model Runner and lifecycle control are never exposed directly to LAN clients; application containers do not mount the Docker Engine socket.
5. Data minimization: prompts, responses, and reasoning content are neither persisted nor logged.
6. Explicit failure: authentication, unsupported parameters, unavailable model, overload, queue timeout, and context overflow produce stable errors; no silent fallback.
7. No tool execution: the service may return model-generated tool calls but cannot execute them.
8. Portable orchestration: the application uses one Compose design for the verified Mac environment and a future private Linux server.
9. Evidence before completion: real model, API, queue, persistence, backup/restore, and browser scenarios must pass before completion is claimed.

## Approved risk posture

- Private network or VPN only.
- Fixed MVP API key `888888`.
- Operations console has no login.
- A dedicated, network-isolated controller may invoke only allowlisted model lifecycle operations through a pinned Docker Model CLI/plugin and explicit internal DMR transport.
- Exact real-time Metal GPU utilization is not required; Metal status, unified memory, throughput, and latency are required.

## Change control

- Product scope changes require user approval and PRD revision.
- Durable architecture changes require a new or superseding ADR and user approval.
- Public exposure, multi-user access, multiple models, tool execution, high availability, or a second inference runtime are out of scope until explicitly approved.
- Implementation details may change within approved PRD and ADR boundaries when the technical specification and verification evidence are updated together.
