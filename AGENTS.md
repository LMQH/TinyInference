# Mini-Inference Agent Rules

## Authority

1. The user's approved product decisions are authoritative.
2. `PROJECT_CONSTITUTION.md` defines project invariants.
3. Approved PRDs and ADRs under `docs/` govern downstream design.
4. Technical specifications may refine implementation details but must not change product scope or ADR invariants.

## Required delivery chain

Changes follow: PRD → ADR → technical specification → implementation plan → implementation → QA → independent code review → compliance/security review.

- Product behavior belongs in the PRD.
- Durable boundaries and trade-offs belong in ADRs.
- APIs, schemas, state transitions, failures, and rollout belong in technical specifications.
- Implementation owners must not approve their own changes.
- QA reports defects and does not repair implementation.

## Runtime isolation

- Runtime services MUST run through Docker Compose or Docker Model Runner.
- Do not install application dependencies or start application services directly on the host.
- Do not write outside this repository except Docker-managed storage and the authorized OMP agent-usage ledger.
- Do not delete or mutate files belonging to other projects.
- The existing GGUF model under `models/` is immutable input; never rewrite or delete it.
- Docker Model Runner's unauthenticated API MUST remain internal and MUST NOT bind directly to the LAN.
- Only the dedicated controller component may invoke fixed model lifecycle operations through the pinned Docker Model CLI/plugin and explicit internal `MODEL_RUNNER_HOST`. No application container may mount the Docker Engine socket or expose arbitrary command execution.

## Product invariants

- Exactly one model and one active inference request.
- Waiting requests use bounded FIFO: 20 waiting entries, 30-minute wait timeout.
- Public inference port: 8888.
- Public inference authentication: Bearer API key `888888` for this private-network MVP.
- The operations console is unauthenticated and private-network/VPN only.
- Prompt, response, and reasoning bodies must not be persisted or logged.
- Request metadata retention is 30 days; database backups are daily and retained for 7 days.
- Tool calls may be generated and returned but never executed by this platform.
- Unsupported OpenAI-compatible parameters fail explicitly; never silently ignore or reinterpret them.
- Context overflow fails explicitly; never silently truncate or summarize.

## Engineering rules

- Prefer the smallest implementation that satisfies the approved contract.
- Reuse one pattern per concern; do not add parallel frameworks or compatibility shims.
- Pin external images and dependencies. Do not use floating `latest` tags.
- Secrets and mutable configuration enter through runtime configuration; do not bake them into images.
- Structured logs must omit request and response content.
- All lifecycle and queue transitions must be explicit, observable, and safe under cancellation or restart.
- Redis is prohibited unless an approved technical design proves it is required.

## Verification

Behavioral changes require runtime evidence through Docker/DMR. Required acceptance coverage is defined in the approved PRD and test specification. Browser-visible changes require real browser verification. Performance is measured and recorded but has no release threshold in the MVP.

## Prohibited actions

Without separate user authorization, do not commit, push, deploy to a remote machine, access accounts or secrets, modify host startup configuration, or expose the service to the public internet.
