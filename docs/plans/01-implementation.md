# Mini-Inference MVP Implementation Plan

- **Status:** Implementation-ready
- **Owner / integration owner:** 后端开发工程师 (`backend`)
- **Target repository:** `/Users/lmqhx/code_space/TinyInference`
- **Authorized change set:** implementation planning only; this plan does not authorize implementation, commit, push, remote deployment, account/secret access, host dependency installation, host scheduler installation, public exposure, or destructive live-database recovery
- **Authoritative inputs:** `AGENTS.md`, `PROJECT_CONSTITUTION.md`, `docs/product/01-prd.md`, `docs/adr/0001-system-architecture.md`, `docs/adr/0002-project-built-compatible-dmr.md`, `docs/adr/0003-compute-appliance-compose-gpu-runtime.md`, `docs/technical/backend.md`, `docs/technical/frontend.md`, `docs/technical/operations.md`, and `docs/plans/02-compute-appliance-feasibility.md`
- **Independent verification input:** `docs/quality/01-test-spec.md`
- **Target acceptance:** PRD `AC-001` through `AC-037`, with real target-appliance Compose, DMR, GPU offload, model, database, backup/restore, and browser evidence
- **Specification review result:** reviewed specifications are aligned for contract generation and implementation. Gates A and B in `02-compute-appliance-feasibility.md` are mandatory prerequisites; this plan begins full implementation only after both pass.

## 1. Delivery rules

1. Implement only the approved P0 scope on the current Apple Silicon Mac. P1/P2 work, Linux or compute-appliance deployment, public exposure, multi-model support, a second inference runtime, tool execution, HA, accounts, external notifications, persistent prompt/KV cache, iOS local inference/server work, and full OpenAI compatibility are prohibited.
2. The implementation owner does not approve its own work. Runtime QA, code review, compliance/security review, and product acceptance are separate gates against an identified revision and compatibility-manifest digest.
3. 后端开发工程师 is the single integration owner. Integration ownership means sealing shared contracts, coordinating checkpoints, assembling the candidate revision, and resolving cross-component contract drift. It does **not** permit edits in another owner's paths.
4. No file has concurrent writers. Every task below has one owner and an exclusive allowed-path set. A task may consume another owner's files read-only after the named checkpoint.
5. The shared contract boundary is completed and sealed before backend/controller, frontend, and operations implementation proceeds in parallel. Consumers generate or hand-write code only inside their own owned paths; no generator writes across an ownership boundary.
6. During parallel implementation, owners run only scoped checks for their own slice. Project-wide build, full-suite test, Compose runtime acceptance, formatter, linter, and cross-component validation are prohibited mid-flight because sibling slices may be incomplete. Project-wide execution begins only at IC-04.
7. Any public/admin/controller schema, database state, error, or lifecycle change discovered during implementation returns to the integration owner and reopens IC-01. Consumers do not add aliases, compatibility shims, permissive decoding, or alternate contracts.
8. Every evidence artifact is content-safe: no prompt, answer, reasoning, tool arguments, authorization header, secret, raw DMR response, controller stdout/stderr, or unrestricted environment data.
9. Empirical failure of the controller CLI transport, per-request reasoning control, tokenizer non-undercount, 131072 context, scoped application/DMR/project resource observation, DMR privacy, verified `keep-alive=-1`, or restore proof blocks acceptance. It does not authorize degraded settings, an undocumented HTTP lifecycle path, a socket mount, another runtime, or silent fallback.

## 2. Exclusive ownership map

| Owner | Exclusive implementation paths | Must not write |
|---|---|---|
| 后端开发工程师 (`backend`) | `services/api/**`, `services/controller/**`, `db/**` | `web/**`, `compose.yaml`, `config/**`, `ops/**`, operational docs |
| 前端开发工程师 (`frontend`) | `web/**` | backend/controller/database, Compose/config/ops, generated producer contracts |
| 运维工程师 (`devops`) | `compose.yaml`, `config/**`, `ops/**`, `Makefile`, `docs/operations/**` | application/controller/frontend source, database migrations |
| 测试工程师 (`qa`) | `docs/quality/01-test-spec.md`, revision-bound `docs/quality/02-qa-acceptance.md`, external evidence bundle | all implementation paths |
| 代码审查专员 (`code-review`) | revision-bound `docs/quality/03-code-review.md` | implementation paths and QA verdict |
| 合规审查专员 (`compliance`) with 安全审查专员 (`security-reviewer`) evidence | revision-bound `docs/quality/04-compliance-security.md` | implementation paths and other gate verdicts |
| 产品经理 (`pm`) / user | revision-bound `docs/quality/05-product-acceptance.md` if a repository record is desired | implementation and independent gate reports |

The existing GGUF under `models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf` is immutable input. No task may modify it. Runtime-created secret files and backup files live under ignored `var/secrets/**` and `var/backups/postgres/**`; they are evidence/runtime data, not source-owned files.

## 3. Dependency graph and checkpoints

```text
P-00 shared contract seal
   ├── B-01 database/fence ── B-02 queue+lifecycle ── B-03 public API+DMR ── B-04 admin/SSE+metrics+retention
   ├── C-01 fixed controller adapter
   ├── F-01 generated client+shell ── F-02 operations UI+SSE+a11y
   └── O-01 images+manifest ── O-02 Compose+model packaging ── O-03 backup/retention/restore+runbooks
                                      │
                         IC-02 lifecycle compatibility gate
                                      │
                         IC-03 integrated contract/data gate
                                      │
                         IC-04 candidate freeze
                                      │
        G-QA → G-CR → G-CS → G-PA (all revision-specific)
```


### IC-01 — Contract Seal

**Owner:** 后端开发工程师. **Participants:** frontend and operations read-only review; independent technical review records specification conformance.

Seal the exact files produced by P-00 and record their SHA-256 values in the handoff. The seal confirms listener/route separation, endpoints, JSON/SSE schemas, enums, nullable fields, errors, controller operations and authority headers, five-migration sequence, least-privilege roles, configuration names, resource provenance, and safe telemetry fields. After sealing, disjoint implementation may start. A contract change invalidates dependent generated code and reopens IC-01.

### IC-02 — Controller/DMR Feasibility Gate

**Owners of evidence:** 运维工程师 executes the approved local lifecycle-proof surface; 后端开发工程师 correlates backend/controller outcomes; 测试工程师 independently witnesses and later repeats it at G-QA.

Pass requires the pinned controller plugin, explicit host-loopback `MODEL_RUNNER_HOST`, no Engine socket, fixed serialized status/load/unload only, epoch+holder authority validation before spawn and throughout every operation, observed state matching, verified `keep-alive=-1` with stale-readiness/eviction failure, transient-only inaccessible DMR memory handling with privacy scans during the canary request, immediately after it, and after runner restart proving zero disk/log/history persistence.

### IC-03 — Integrated Contract and Data Gate

**Integration owner:** 后端开发工程师. Consume, without editing, frontend and operations artifacts. Verify generated frontend types match the sealed producer bundle, all five migrations precede API authority acquisition, prior running requests and model operations reconcile before lifecycle admission, LAN `:8888` is public-only, admin `:8889` is Compose-only behind the exact web proxy allowlist, authority header/turnover semantics match, Compose names/configuration match sealed inputs, the compatibility manifest references exact artifacts, and operational jobs use backend-owned predicates/assertions. Resolve drift with the owning task; never patch another owner's path.

### IC-04 — Candidate Freeze

The integration owner identifies one immutable candidate revision plus `config/compatibility-manifest.json` digest and evidence-bundle identifier. Only now may project-wide Compose/runtime/browser validation begin. Any subsequent implementation change invalidates affected gates and requires a new candidate identifier.

## 4. Ordered implementation slices

Each slice is a complete task contract. “Evidence” is the acceptance evidence the owner must hand to the next checkpoint; it is not permission to run project-wide validation mid-flight.


### P-00 — Authoritative contract generation boundary

- **Owner:** 后端开发工程师 (`backend`)
- **Allowed paths:** `services/api/contracts/**`, `services/api/cmd/contractgen/**`, `services/api/internal/contract/generated/**`, `services/controller/internal/contract/generated/**`
- **Inputs:** backend technical specification §§3–10 and 12; frontend specification §7; operations specification §§5–6 and 10
- **Outputs:**
  - `services/api/contracts/public.openapi.yaml` for the exact authenticated `/v1` text subset on LAN `:8888`, including rejection of every admin/internal/health/metrics path;
  - `services/api/contracts/admin.openapi.yaml` for the Compose-only `:8889` `/admin/v1` reads/actions, pagination, ETag, and internal health/readiness semantics used only by Compose health consumers; the web proxy allowlist exposes only the enumerated `/admin/v1/*` routes and never health/metrics;
  - `services/api/contracts/admin-events.schema.json` for SSE envelopes, replay, heartbeat, and `resync_required`;
  - `services/api/contracts/controller.openapi.yaml` for fixed serialized status/load/unload with `X-Authority-Epoch` and `X-Authority-Holder`;
  - `services/api/contracts/config.schema.json` for required backend/controller variable names, listener separation, `keep-alive=-1`, and safe constraints;
  - `services/api/contracts/contract-manifest.json` containing schema versions and hashes;
  - `services/api/cmd/contractgen/**` containing the pinned deterministic generation entry point; it accepts only the sealed contract sources and writes only the backend/controller generated directories.
  - generated Go contract types only under the two generated directories above.
- **Dependencies:** approved PRD, corrected effective ADR, and independently reviewed implementation-ready technical specifications
- **Acceptance evidence:** deterministic regeneration produces no diff; strict request/response examples validate; contract manifest hashes every source; no prompt/response/reasoning/tool-argument field exists in an administrative, event, persistence, logging, or operations shape; frontend and operations acknowledge the sealed names read-only
- **Trace:** AC-008–015, AC-017–020, AC-022–027, AC-029–037
- **Reviewer:** 技术审查专员 (`reviewer`), independent of implementation

### B-01 — PostgreSQL migrations, authority fence, and repository layer

- **Owner:** 后端开发工程师 (`backend`)
- **Allowed paths:** `db/migrations/000001_authority.sql`, `db/migrations/000002_requests.sql`, `db/migrations/000003_lifecycle_admin.sql`, `db/migrations/000004_metrics_operations.sql`, `db/migrations/000005_roles_grants.sql`, `db/queries/**`, `services/api/internal/store/**`, `services/api/internal/authority/**`
- **Inputs:** sealed P-00 bundle; backend specification §§5, 10, and 12
- **Outputs:** forward-only transactional migrations; schema/version check; dedicated-session advisory lock `741291551`; epoch acquisition/heartbeat/loss handling; current-epoch fenced writes and strictly-prior-epoch reconciliation; reconciliation of every prior-epoch `running` model operation to `interrupted` before any new lifecycle action; seven least-privilege database roles/grants; transactionally coupled state/event writes; keyset query, aggregation, operational evidence, and strict 30-day cleanup primitives
- **Dependencies:** IC-01
- **Acceptance evidence:** scoped migration rehearsal against a disposable PostgreSQL instance; schema inspection proves exact CHECK/index/FK constraints and absence of content columns; second authority cannot become ready; lock-session loss stops admission and cannot be reacquired by the same process; prior `waiting|active` requests and `running` model operations become `interrupted` before new work; exact 30-day boundary preserves equal and deletes strictly older data
- **Trace:** AC-016, AC-021, AC-022, AC-026, AC-029–033
- **Reviewer:** 代码审查专员 at G-CR; backend-spec verification cases V-14, V-15, V-18–V-21 plus mapped QA scenarios at G-QA

### B-02 — Request queue and model/request state machines

- **Owner:** 后端开发工程师 (`backend`)
- **Allowed paths:** `services/api/internal/queue/**`, `services/api/internal/requeststate/**`, `services/api/internal/modelstate/**`, `services/api/internal/lifecycle/**`
- **Inputs:** P-00 generated Go contracts; B-01 transactions/fence; backend specification §§5–6
- **Outputs:** one-active in-memory FIFO; capacity 20 excluding active; monotonic arrival sequence; a timeout scheduler covering every waiting node; O(1) waiting cancellation; legal terminal transitions; Stop cancellation; graceful shutdown/crash/restart interruption semantics; startup reconciliation of prior-epoch work to `interrupted`; no replay
- **Dependencies:** B-01 and IC-01
- **Acceptance evidence:** scoped deterministic concurrency scenarios prove 1 active + 20 waiting, 21st rejection, FIFO promotion, middle cancellation order, exact timeout terminality, Stop draining, restart interruption, and fail-closed fence loss; transition/event pairs commit atomically
- **Trace:** AC-002–004, AC-016–022
- **Reviewer:** 代码审查专员; backend-spec verification cases V-09–V-15 plus mapped QA scenarios

### B-03 — DMR inference adapter, tokenizer, public API, and streaming

- **Owner:** 后端开发工程师 (`backend`)
- **Allowed paths:** `services/api/cmd/api/**`, `services/api/internal/http/public/**`, `services/api/internal/dmr/**`, `services/api/internal/tokenizer/**`, `services/api/internal/stream/**`, `services/api/internal/auth/**`, `services/api/internal/safeerror/**`, `services/api/go.mod`, `services/api/go.sum`
- **Inputs:** P-00 public contract; B-02 admission/cancellation; compatibility settings supplied read-only by operations
- **Outputs:** authenticated models/chat/completions handlers on the LAN `:8888` public-only listener; explicit rejection of `/admin/*`, `/internal/*`, `/health/*`, and `/metrics*` on that listener; strict JSON and parameter validation; stable safe errors; context counting without undercount; per-request reasoning translation; DMR `keep-alive=-1`; incremental SSE and `[DONE]`; usage/reasoning/tool-call mapping; downstream cancellation and backpressure; no tool execution
- **Dependencies:** B-02; O-01 may supply version values but does not block source implementation
- **Acceptance evidence:** scoped protocol checks cover credential equivalence, every allowed/unsupported field, 131072/131073 boundary, direct non-public route rejection on `:8888`, stream chunking, reasoning reset, tool-call return without execution, disconnect cancellation, verified `keep-alive=-1`, DMR protocol/state mismatch, and safe error output; real-model claims are deferred to G-QA
- **Trace:** AC-006, AC-008–016, AC-022, AC-030, AC-037
- **Reviewer:** 代码审查专员; backend-spec verification cases V-03–V-08 and V-13 plus mapped QA scenarios

### B-04 — Administrative API, SSE recovery, metrics, safe observability, and retention service

- **Owner:** 后端开发工程师 (`backend`)
- **Allowed paths:** `services/api/internal/http/admin/**`, `services/api/internal/adminstream/**`, `services/api/internal/metrics/**`, `services/api/internal/resources/**`, `services/api/internal/alerts/**`, `services/api/internal/safelog/**`, `services/api/internal/retention/**`, `services/api/internal/operations/**`
- **Inputs:** P-00 admin/event contracts; B-01 store; B-02 state snapshots; B-03 usage timings; controller status contract
- **Outputs:** all specified admin GET/action endpoints on Compose-only `:8889`; ETag/304; bounded SSE replay and heartbeat; authoritative snapshots; fixed metrics windows; safe alerts/log projection; scoped API-container CPU, DMR-process unified memory, project/model/runtime storage, and DMR-engine Metal status with exact provenance; pre-admission loaded-state verification that makes readiness unavailable on eviction/mismatch rather than reloading silently; hourly aggregation and daily 30-day pruning.
- **Dependencies:** B-01–B-03; controller response consumption from C-01 contract
- **Acceptance evidence:** scoped API checks prove `:8889` is reachable only from Compose peers through the web allowlist, nullable/unknown semantics, pagination order, legal metric combinations, replay ≤1000 and forced resync, exact resource provenance/staleness, eviction or stale loaded state fails readiness and admission, action conflicts, fixed log fields, no external alert transport, and unavailable resources without fabricated zeroes or mislabeled scopes
- **Trace:** AC-003, AC-004, AC-017, AC-020–033, AC-035–037
- **Reviewer:** 代码审查专员; backend-spec verification cases V-17–V-20 and V-24 plus mapped QA scenarios

### C-01 — Fixed DMR lifecycle controller adapter

- **Owner:** 后端开发工程师 (`backend`)
- **Allowed paths:** `services/controller/cmd/controller/**`, `services/controller/internal/config/**`, `services/controller/internal/http/**`, `services/controller/internal/adapter/**`, `services/controller/internal/authority/**`, `services/controller/internal/safestatus/**`, `services/controller/go.mod`, `services/controller/go.sum`
- **Inputs:** P-00 controller contract; exact plugin/model/config identities from operations specification §§4–5
- **Outputs:** fixed status/load/unload server on internal `9090`; exact-origin validation; required `X-Authority-Epoch` and `X-Authority-Holder` validation against the read-only authority view; one serialized lifecycle operation; authority revalidation before process spawn and throughout load/unload so turnover cancels/fails the operation before a stale result can commit; allowlisted fixed argv mapping; bounded safe runtime-resource output with truthful provenance; deadlines; observed-state verification; no body/query/argument/model/URL/timeout injection; no fallback transport
- **Dependencies:** IC-01; O-01 packages the pinned plugin separately
- **Acceptance evidence:** scoped adapter tests prove all unknown methods/paths/headers/body/query/model/argument attempts are rejected before spawn; missing/malformed/localhost/arbitrary `MODEL_RUNNER_HOST` fails startup; missing/stale/mismatched `X-Authority-Epoch` or `X-Authority-Holder` cannot spawn; concurrent lifecycle calls serialize; authority turnover immediately before spawn and during load/unload fails closed and cannot publish stale success; timeout/non-zero/state mismatch is failed or indeterminate; raw output and environment never cross the boundary. IC-02 supplies mandatory in-container real execution proof.
- **Trace:** AC-001, AC-003–005, AC-025, AC-034–037
- **Reviewer:** 代码审查专员 and compliance/security gate

### F-01 — Frontend generated consumer, application shell, and core operations surfaces

- **Owner:** 前端开发工程师 (`frontend`)
- **Allowed paths:** `web/package.json`, `web/package-lock.json`, `web/tsconfig*.json`, `web/vite.config.*`, `web/src/api/generated/**`, `web/src/api/client/**`, `web/src/app/**`, `web/src/routes/overview/**`, `web/src/routes/requests/**`, `web/src/styles/**`, `web/public/**`
- **Inputs:** sealed P-00 files read-only; frontend specification §§2–7 and 9–12
- **Outputs:** pinned Carbon/React dependency set; generated wire types under frontend ownership; same-origin strict client; five-route shell; Chinese overview; model Start/Stop confirmation; active/FIFO/history UI; waiting cancellation; loading/empty/error/stale/unavailable/unknown states; no login or browser-held secret
- **Dependencies:** IC-01; may proceed in parallel with B/C/O work
- **Acceptance evidence:** deterministic frontend generation from sealed contracts; scoped component/browser checks for routes, Chinese labels, no optimistic success, Stop/cancel focus return, fixed producer ordering, unknown enums, no reordering controls, and no content in DOM/storage/URL/console fixtures
- **Trace:** AC-002–004, AC-016–024, AC-030, AC-034–037
- **Reviewer:** 代码审查专员; QA real-browser scenarios 1–6, 12–15

### F-02 — Metrics/data/alerts surfaces, SSE recovery, and accessibility completion

- **Owner:** 前端开发工程师 (`frontend`)
- **Allowed paths:** `web/src/routes/metrics/**`, `web/src/routes/alerts/**`, `web/src/routes/data/**`, `web/src/realtime/**`, `web/src/components/**`, `web/src/accessibility/**`
- **Inputs:** F-01 client/shell; sealed admin/SSE contract; frontend specification §§4.3–4.5, 7.3, 8, 10, and 14
- **Outputs:** metrics charts plus equivalent tables; exact application/DMR/project resource labels and provenance from the producer contract; safe alerts/logs; retention/backup/restore evidence; Last-Event-ID reconnect/resync; polling fallback; stale presentation; responsive/keyboard/VoiceOver/reduced-motion behavior
- **Dependencies:** F-01; may proceed in parallel with backend/operations after IC-01
- **Acceptance evidence:** scoped browser checks cover every legal metric window, gaps vs zero, exact API-container/DMR-process/project-storage/Metal provenance and unavailable/stale behavior, local-only alert presentation, stale/recovery behavior, duplicate/unknown events, 320/768/1280 widths, 200%/400% zoom, keyboard-only actions, semantic tables, focus, live regions, and reduced motion. Real data correlation is deferred to G-QA.
- **Trace:** AC-023–033, AC-036, AC-037
- **Reviewer:** 代码审查专员; QA real-browser scenarios 7–15

### O-01 — Immutable images and compatibility manifest

- **Owner:** 运维工程师 (`devops`)
- **Allowed paths:** `ops/images/api.Dockerfile`, `ops/images/web.Dockerfile`, `ops/images/controller.Dockerfile`, `ops/images/jobs.Dockerfile`, `ops/images/postgres.digest`, `ops/images/checksums/**`, `config/compatibility-manifest.json`, `config/compatibility-manifest.schema.json`
- **Inputs:** operations specification §§3, 5, and 10; P-00 contract/config manifests; application dependency locks read-only
- **Outputs:** digest-pinned, non-root, read-only image definitions; pinned controller plugin and checksum; no runtime package manager/general command runner in controller; complete compatibility manifest including model, images, platform/DMR/engine/plugin, schema, settings, resolved-config hash, and evidence identifier
- **Dependencies:** IC-01; final image digests are completed after B/C/F artifacts exist, but file ownership never changes
- **Acceptance evidence:** scoped image inspection proves pinned bases, target architecture, non-root/read-only constraints, plugin checksum, no Engine/Docker CLI shell surface in controller runtime, and manifest schema completeness; no floating tag
- **Trace:** AC-001, AC-005–007, AC-028, AC-034–037
- **Reviewer:** compliance/security gate and 代码审查专员

### O-02 — Compose topology and immutable model packaging

- **Owner:** 运维工程师 (`devops`)
- **Allowed paths:** `compose.yaml`, `config/runtime.mac.conf`, `ops/model/package.sh`, `ops/model/verify.sh`, `ops/lifecycle-proof/**`, `ops/web-proxy/**`, `Makefile`
- **Inputs:** operations specification §§2, 4–6, 8, and 12–13; O-01 image/manifest identities; sealed P-00 configuration names
- **Outputs:** one `mini-inference` Compose topology; long-running api/web/controller/postgres/backup-scheduler; isolated one-shot migration/restore jobs; internal data/control/dmr/restore networks; LAN `:8888` exposing only authenticated public inference, LAN `:8080` serving the console, and Compose-only API admin `:8889` reachable only through the web's exact same-origin allowlist; direct admin/internal/health/metrics access rejected; explicit secrets files; exact model declaration/settings with verified DMR `keep-alive=-1`; immutable local OCI packaging with before/after GGUF hash; lifecycle and privacy proof positive/negative cases; exact allowlisted local command facade
- **Dependencies:** O-01 and IC-01; IC-02 requires runnable C-01 and backend lifecycle consumer
- **Acceptance evidence:** rendered Compose and listener inspection reject `0.0.0.0`, LAN reachability to `:8889`, direct `/admin/*`, `/internal/*`, `/health/*`, or `/metrics*` on public ingress, unexpected port/network/service/mount, socket/root/model-source write mount, extra model/runtime, and unset required config; packaging records OCI digest while source size/SHA remain unchanged; lifecycle proof verifies `keep-alive=-1`, status before admission, stale readiness/eviction fail-closed, fixed operations, and authority turnover; privacy proof permits only transient in-memory DMR processing inaccessible to LAN/application services and records all surfaces during a canary request, immediately after it, and after runner restart, proving zero request/response body persistence in disk/logs at every timepoint and zero persisted history/storage after restart
- **Trace:** AC-001–007, AC-034–037
- **Reviewer:** compliance/security gate; backend-spec verification cases V-01, V-02, V-22, V-23 plus mapped QA scenarios

### O-03 — Backup, retention, restore drill, and operational runbooks

- **Owner:** 运维工程师 (`devops`)
- **Allowed paths:** `ops/jobs/migrate/**`, `ops/jobs/backup/**`, `ops/jobs/restore-drill/**`, `ops/schedule/**`, `docs/operations/01-local-runbook.md`, `docs/operations/02-backup-restore.md`, `docs/operations/03-compatibility-rollout.md`
- **Inputs:** B-01 migrations/predicates/assertions read-only; B-04 operational evidence endpoint; operations specification §§7, 10–13
- **Outputs:** one-shot migration, atomic custom-format backup, strict deletion of every completed archive/checksum older than 7×24 hours, Compose-contained daily backup scheduling/catch-up/freshness reporting, integration and health reporting for the backend-owned aggregation/30-day retention schedule, disposable isolated-database restore drill, checksums/locks/content-safe result recording, and complete local rollout/rollback/failure/recovery runbooks
- **Dependencies:** B-01 and B-04 interfaces; O-02 topology
- **Acceptance evidence:** scoped disposable-data exercise proves `.partial` never becomes a valid failed archive, SHA/list validation, single-run lock, repository containment, no symlink traversal, and deletion of **every** completed backup older than 7×24 hours including a lone stale archive; disposable restore verifies schema/count/token/no-content assertions and cleans up on success/failure. Runbooks contain the exact approved command allowlist and explicitly prohibit live restore without separate authorization.
- **Trace:** AC-029–033, AC-036, AC-037
- **Reviewer:** 代码审查专员 and compliance/security gate; backend-spec verification cases V-19–V-21 plus mapped QA scenarios

## 5. Parallel execution waves

| Wave | Work allowed | Entry condition | Exit condition |
|---|---|---|---|
| 0 | P-00 only | specifications independently reviewed; no unresolved product/architecture/authorization decision | IC-01 contract hashes sealed |
| 1 | B-01, C-01, F-01, O-01 in parallel | IC-01 | each slice's scoped evidence passes; no cross-owner writes |
| 2 | B-02, F-02, O-02 in parallel; O-01 finalizes digests only after artifacts exist | required predecessor slice | IC-02 passes; queue/UI/topology are individually complete |
| 3 | B-03 then B-04; O-03 after backend data/operations interfaces | predecessor slices and IC-02 | every implementation slice complete; no stub, mock, fake fallback, or deferred required behavior |
| 4 | IC-03, then candidate assembly and IC-04 | all slice handoffs | one revision and manifest digest frozen |
| 5 | independent gates G-QA → G-CR → G-CS → G-PA | IC-04 | all gates pass on the same candidate, or candidate returns to the owning slice and gates are rerun as affected |

Within a wave, an owner may further serialize work but may not transfer ownership or expand paths. No wave permits a project-wide check before IC-04.

## 6. Integration evidence bundles

The integration owner indexes evidence by stable IDs without storing prohibited content:

- `E-CONTRACT`: sealed contract hashes and deterministic generation result.
- `E-SCHEMA`: migration version, schema constraints, fence behavior, and no-content column inspection.
- `E-LIFECYCLE`: controller positive/negative gate, model source before/after hash, OCI digest, loaded inventory, warm/unload outcome, llama.cpp Metal backend and enabled state.
- `E-API`: authentication, strict parameter matrix, chat/completion stream/non-stream, reasoning, tools-no-execution, context boundary, cancellation.
- `E-QUEUE`: one-active/FIFO/capacity/timeout/cancel/Stop/restart/fence evidence.
- `E-OBS`: safe logs/alerts, metrics/token/timing correlation, exact application/DMR/project resource-provenance proof, verified loaded-state/`keep-alive=-1`, and performance baseline conditions.
- `E-PRIVACY`: proof that DMR content exists only transiently in inaccessible in-memory processing, with a per-surface result during the canary request, immediately after it, and after runner restart across database, safe logs, container/runner logs, metrics, browser stores, repository/project storage, DMR history/recorder seams, backup, and restored database; disk/log matches are zero at every timepoint and persisted history/storage matches are zero after restart.
- `E-DATA`: 30-day boundary, daily backup, 7-day retention, checksum, disposable restore, token totals, measured restore duration.
- `E-BROWSER`: real-browser Chinese UI, lifecycle/queue flows, accessibility, responsive layout, stale/reconnect, and no-content inspection.
- `E-TOPOLOGY`: rendered Compose, listeners, networks, mounts, images, egress, secret delivery, no public/DMR/controller/PostgreSQL/socket/second-runtime path.

Evidence records the candidate revision, compatibility-manifest digest, timestamps, environment/version identities, request/operation IDs where safe, expected/observed result, and artifact checksum. Actual prompt/response/reasoning/tool arguments are never captured; privacy canaries are searched for absence and results record only location/count/outcome.

## 7. Independent gates

### G-QA — Runtime QA acceptance

- **Owner:** 测试工程师 (`qa`), independent and read-only over implementation
- **Input:** frozen candidate from IC-04, `docs/quality/01-test-spec.md`, all evidence bundle definitions
- **Execution:** run the real Docker Compose/host-loopback DMR/Metal/model/API/queue/database/backup-restore/browser scenarios on the current Apple Silicon Mac. Unit or mock evidence cannot replace runtime acceptance.
- **Required report:** `docs/quality/02-qa-acceptance.md`, naming revision and manifest digest; per-scenario pass/fail, exact executed commands, evidence references, defects, residual risks, and full AC-001–AC-037 trace
- **Pass rule:** every P0 scenario passes; performance values are recorded without a numeric threshold. A failure returns to the exclusive owner and invalidates affected later gates.

### G-CR — Independent code review

- **Owner:** 代码审查专员 (`code-review`)
- **Input:** the exact G-QA-passed revision and diff against its declared base
- **Required report:** `docs/quality/03-code-review.md`, naming both revisions and reviewed paths
- **Review focus:** contract fidelity; queue races/cancellation/backpressure; fence and transaction correctness; migration safety; strict parsing/error convergence; content leakage; controller argument injection; frontend authoritative-state handling/accessibility; operational failure/rollback paths; absence of stubs, mocks, shims, alternate runtimes, and residual obsolete paths
- **Pass rule:** no blocking defect. Any implementation change after review invalidates affected G-QA and G-CR evidence.

### G-CS — Compliance and security review

- **Owners:** 合规审查专员 (`compliance`) issues the gate; 安全审查专员 (`security-reviewer`) supplies independent security findings
- **Input:** exact G-CR-passed revision, compatibility manifest, topology/privacy/lifecycle/data evidence
- **Required report:** `docs/quality/04-compliance-security.md`, naming revision and manifest digest
- **Review focus:** private-interface binding; LAN `:8888` public-inference-only and LAN `:8080` console-only; Compose-only admin `:8889`; direct admin/internal/health/metrics rejection; DMR/controller/PostgreSQL internal; no Engine socket; serialized controller capability and epoch+holder turnover resistance; secret-file delivery; immutable/pinned artifacts; accepted fixed-key/no-login risks; transient-only inaccessible DMR memory handling and zero prohibited disk/log/backup/browser persistence; no external alert/tool-execution/public/multi-model/second-runtime/Linux-deployment path; strict backup expiry; restore authorization boundary
- **Pass rule:** explicit pass on both compliance and security findings, or a clearly blocking verdict. Accepted PRD risks are documented, not “fixed” by scope expansion.

### G-PA — User product acceptance

- **Owner:** 产品经理 (`pm`) coordinates; only the user/product owner accepts
- **Input:** same candidate revision with passed G-QA, G-CR, and G-CS reports
- **Record:** `docs/quality/05-product-acceptance.md` only when the product owner provides the acceptance decision
- **Pass rule:** every PRD product-acceptance checklist item and AC-001–AC-037 is supported by evidence; no gate report substitutes for user acceptance.

No commit, push, registry publish, remote deployment, host scheduler installation, production operation, secret rotation, or live restore is part of these gates without separate exact authorization.

## 8. Complete AC trace

| AC | Owning slices | Required independent evidence |
|---|---|---|
| AC-001 | C-01, O-01, O-02 | E-LIFECYCLE, E-TOPOLOGY |
| AC-002 | B-02, F-01, O-02 | E-QUEUE, E-BROWSER |
| AC-003 | B-02, B-04, C-01, F-01 | E-LIFECYCLE, E-BROWSER |
| AC-004 | B-02, B-04, C-01, F-01 | E-LIFECYCLE, E-QUEUE, E-BROWSER |
| AC-005 | C-01, O-01, O-02 | E-LIFECYCLE, E-TOPOLOGY |
| AC-006 | B-03, O-01, O-02 | E-API, E-LIFECYCLE |
| AC-007 | O-01, O-02 | E-LIFECYCLE |
| AC-008 | P-00, B-03 | E-API |
| AC-009 | P-00, B-03 | E-API |
| AC-010 | P-00, B-03 | E-API |
| AC-011 | P-00, B-03 | E-API |
| AC-012 | P-00, B-03 | E-API |
| AC-013 | P-00, B-03 | E-API, E-PRIVACY |
| AC-014 | P-00, B-03 | E-API |
| AC-015 | P-00, B-03 | E-API |
| AC-016 | B-01, B-02, B-03, F-01 | E-QUEUE |
| AC-017 | P-00, B-02, B-04, F-01 | E-QUEUE, E-BROWSER |
| AC-018 | P-00, B-02 | E-QUEUE |
| AC-019 | P-00, B-02, B-04, F-01 | E-QUEUE, E-BROWSER |
| AC-020 | P-00, B-02, B-04, F-01 | E-QUEUE, E-BROWSER |
| AC-021 | B-01, B-02, B-04, F-01 | E-QUEUE, E-BROWSER |
| AC-022 | P-00, B-01–B-04, F-01 | E-API, E-QUEUE, E-BROWSER |
| AC-023 | P-00, F-01, F-02 | E-BROWSER |
| AC-024 | P-00, B-04, F-01, F-02 | E-OBS, E-BROWSER |
| AC-025 | P-00, B-04, C-01, F-02 | E-OBS, E-BROWSER |
| AC-026 | P-00, B-01, B-04, F-02 | E-OBS, E-BROWSER |
| AC-027 | P-00, B-04, F-02 | E-OBS, E-TOPOLOGY |
| AC-028 | B-01, B-04, F-02, O-01 | E-OBS |
| AC-029 | P-00, B-01, B-04, F-02, O-03 | E-DATA, E-BROWSER |
| AC-030 | P-00, B-03, B-04, F-01, O-03 | E-PRIVACY |
| AC-031 | P-00, B-01, B-04, F-02, O-03 | E-DATA, E-BROWSER |
| AC-032 | P-00, B-04, F-02, O-03 | E-DATA, E-BROWSER |
| AC-033 | P-00, B-01, B-04, F-02, O-03 | E-DATA, E-PRIVACY, E-BROWSER |
| AC-034 | P-00, B-03, B-04, C-01, F-01, O-02 | E-TOPOLOGY, E-API |
| AC-035 | P-00, B-04, C-01, F-01, O-02 | E-LIFECYCLE, E-TOPOLOGY |
| AC-036 | P-00, B-04, C-01, F-01, F-02, O-01–O-03 | E-TOPOLOGY, G-CS report |
| AC-037 | P-00, B-03, B-04, C-01, F-01, F-02, O-01–O-03 | E-TOPOLOGY, E-PRIVACY, G-CS report |

## 9. Completion definition and risk handling

The implementation is complete only when every slice output exists without incomplete stubs or deferred required behavior, IC-01 through IC-04 pass, and G-QA, G-CR, G-CS, and G-PA all apply to the same unchanged candidate. There is no unassigned bucket for required P0 work.

Known empirical risks are acceptance blockers, not open scope: 131072-context memory viability; pinned DMR/plugin/engine support for reasoning, cache reuse, and `keep-alive=-1`; tokenizer/chat-template agreement; in-container no-socket lifecycle; scoped application/DMR/project resource accuracy; transient-only DMR content handling with zero disk/log persistence after restart; authority turnover during lifecycle work; and real restore correctness. If any fails, the integration owner reports the exact failed evidence and returns to the authorized decision chain. The team must not substitute weaker settings, alternate architecture, undocumented interfaces, fake data, or a narrowed acceptance scenario.
