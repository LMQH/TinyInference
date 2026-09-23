# Mini-Inference Operations Technical Specification

- **Status:** Implementation-ready
- **Owner:** 运维工程师
- **Target environment:** MVP on the current Apple Silicon Mac, private LAN/VPN only
- **Authoritative inputs:** `PROJECT_CONSTITUTION.md`, `docs/product/01-prd.md`, `docs/adr/0001-system-architecture.md`, `docs/adr/0002-project-built-compatible-dmr.md`, `docs/adr/0003-apple-silicon-mac-and-ios-client-target.md`, `docs/architecture/00-dmr-research.md`, and `docs/architecture/01-controller-feasibility.md`

## 1. Scope and non-goals

This specification owns the Compose topology, immutable images, Docker Model Runner (DMR) integration, model OCI packaging, networks, configuration delivery, persistent volumes, the Compose-contained backup scheduler, operational one-shot jobs, health/readiness, telemetry transport, backup/restore, compatibility versioning, rollout/rollback, and runtime proof procedure.

The Go `api` remains the sole business, queue, lifecycle, public-contract, and database-schema authority. The React/TypeScript `web` consumes backend contracts. The `controller` is only a fixed lifecycle adapter. DMR is the sole inference runtime and is a loopback-only host facility, not a Compose service. Redis, a second inference runtime, multiple models, high availability, public exposure, external alert delivery, and iOS local inference are excluded.

No procedure in this document authorizes a commit, push, remote deployment, public exposure, account or secret access, host dependency installation, host startup modification, or destructive recovery of the live database. Those actions require separate authorization.

## 2. One Compose topology

The implementation SHALL use one Compose project named `mini-inference` and exactly these long-running services:

| Service | Responsibility | Networks | Published port | Persistent access |
|---|---|---|---|---|
| `api` | Go gateway and sole application authority; public listener `:8888`, Compose-only admin listener `:8889`; in-process aggregation and metadata retention | `ingress`, `data`, `control`, `dmr` | `${LAN_BIND_ADDRESS}:8888:8888` only | PostgreSQL only; no model/source bind |
| `web` | Static Simplified Chinese console and allowlisted same-origin proxy to `api:8889` | `ingress` | `${LAN_BIND_ADDRESS}:8080:8080` | none |
| `controller` | Fixed Docker Model CLI lifecycle adapter and authority-epoch validator | `control`, `data`, `dmr` | none | read-only PostgreSQL view only |
| `postgres` | Durable metadata and authority fence | `data` | none | Docker volume `postgres-data` |
| `backup-scheduler` | Daily logical backup, 7-day backup-file pruning, missed-run catch-up, and freshness reporting | `data` | none | `./var/backups/postgres:/backups` |

Operational Compose-profile services SHALL be one-shot and SHALL exit after one result:

| Service | Profile | Networks | Mounts | Purpose |
|---|---|---|---|---|
| `migrate` | `ops` | `data` | none | apply forward migrations before `api` acquires authority |
| `backup-prune` | `ops` | `data` | backup bind | operator-invoked fixed 7-day backup-file pruning proof; no metadata deletion |
| `postgres-restore` | `restore` | `restore` | disposable Docker volume | isolated temporary PostgreSQL target, never the live data network |
| `restore-drill` | `restore` | `restore` | backup bind read-only | restore into `postgres-restore`, verify, and tear it down |
| `restore-proof-recorder` | `restore` | `data` | content-free evidence file read-only | execute only production `record_restore_proof(...)`, then exit |
| `lifecycle-proof` | `ops` | `control`, `data`, `dmr` | none | execute the positive/negative controller feasibility gate without caller-supplied commands |

No operational service owns business semantics. Schema, restored-data assertions, hourly aggregation, and the daily 30-day metadata-retention schedule are backend-owned. The backup scheduler owns only backup mechanics and backup-file retention.

### 2.1 Networks

- `ingress`: ordinary bridge used only by `api` and `web`. Only `api:8888` and `web:8080` are published on the explicitly configured private interface; `api:8889` is Compose-only and unexposed. `0.0.0.0` is prohibited because it cannot prove private-only binding.
- `data`: `internal: true`; only `api`, `controller`, `postgres`, `backup-scheduler`, `migrate`, `backup-prune`, and `restore-proof-recorder` attach. Grants are narrower than network reachability; the restore engine itself never attaches.
- `control`: `internal: true`; only `api` and `controller` attach.
- `dmr`: dedicated DMR egress segment; only `api` and `controller` attach. It provides the platform-specific `model-runner.docker.internal` host route. It publishes no port.
- `restore`: `internal: true`; only `postgres-restore` and `restore-drill` attach. It has no route to `data`, `control`, `dmr`, or `ingress`.

The project-built DMR SHALL listen only on host loopback at `12435`; Compose SHALL NOT publish or proxy it. API admin `8889`, PostgreSQL `5432`, controller `9090`, DMR `12435`, health endpoints, and metrics endpoints SHALL never be LAN-published. No application container—including `api`, `web`, `controller`, `backup-scheduler`, or any operational job—mounts `/var/run/docker.sock`, a Docker Desktop/Engine socket, the host root filesystem, or an unrestricted host path.

### 2.2 Public and admin ingress

The published `api:8888` listener accepts only `GET /v1/models`, `POST /v1/chat/completions`, and `POST /v1/completions`. It rejects `/admin/*`, `/internal/*`, health, metrics, every other path, and wrong methods before application dispatch. The distinct `api:8889` listener is bound only inside Compose and never published; it serves the backend-defined admin surface to `web` plus Compose-only health/metrics checks. `/internal/*` is rejected on `:8889` too. `web` never republishes the health/metrics paths.

`web` proxies only these same-origin routes to `http://api:8889`: `GET /admin/v1/snapshot`, `GET /admin/v1/requests`, `GET /admin/v1/metrics`, `GET /admin/v1/alerts`, `GET /admin/v1/logs`, `GET /admin/v1/operations`, `GET /admin/v1/events`, `POST /admin/v1/model/start`, `POST /admin/v1/model/stop`, and `POST /admin/v1/queue/{request_id}/cancel`. It preserves method, backend-allowlisted query parameters, `X-Request-ID`, and `Last-Event-ID` only where permitted. Every other `/admin/` route, every `/internal/*` route, and health/metrics paths are rejected at the published `web:8080` entry.

For `GET /admin/v1/events`, the proxy uses HTTP/1.1 streaming, disables response/request buffering, caching, compression-induced buffering, and body-size accumulation, forwards `Last-Event-ID` unchanged, flushes each SSE event/comment immediately, and sets an upstream read timeout longer than two 15-second heartbeats. Disconnect closes the upstream request. Reconnection and replay semantics remain backend-owned; the proxy never synthesizes an event or action success.

## 3. Immutable images and process constraints

All image references SHALL resolve through the compatibility manifest to a content digest; floating tags including `latest` are invalid.

- `api`: multi-stage build, pinned Go builder, distroless/static runtime, non-root UID, read-only root filesystem, dropped capabilities, `no-new-privileges`, writable `tmpfs` only where required.
- `web`: pinned build and static-server images, non-root UID, read-only root filesystem, dropped capabilities, and no runtime package installation.
- `controller`: Linux ARM64 container image for Docker Desktop on the approved Mac. A multi-stage build compiles or installs a single pinned `docker-model` plugin release/commit, verifies its published or repository-recorded SHA-256, then copies it and the narrow controller binary into a non-root read-only runtime image. `PATH` SHALL contain the fixed plugin location. The image SHALL contain no Docker CLI, shell-facing HTTP facility, package manager, curl, or general command runner in its runtime layer.
- `postgres`: a fixed PostgreSQL major/minor image digest. Major upgrades require a separately reviewed migration and restore plan.
- operational jobs: fixed PostgreSQL-client/controller image digests; no mutable package installation at job start.

Compose SHALL set explicit memory/CPU/PID limits after the real-model baseline. Limits must fail explicitly rather than cause host swapping to masquerade as readiness. The `api` remains exactly one replica. Restart policy is `unless-stopped` for long-running services and `no` for one-shot jobs.

## 4. Model OCI packaging and identity

### 4.1 Immutable source

The only source is:

`models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf`

Its verified local identity is:

- size: `1561318368` bytes
- SHA-256: `ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd`
- local OCI reference: `local/minicpm5-2b:q4_k_m-ec2d58016400`

Packaging reads the GGUF in place. It SHALL NOT copy back, rewrite, truncate, chmod, rename, or delete the source. A before/after source hash equality check is mandatory. The generated OCI digest is recorded in the compatibility manifest; the tag alone is never sufficient release identity. No registry push is required or authorized.

### 4.2 Compose model declaration

The top-level Compose model key SHALL be `minicpm5` with:

- `model`: the manifest-pinned local OCI reference/digest;
- `context_size: 131072`;
- `runtime_flags`: `--batch-size 2048`, `--temp 0.8`, `--reasoning-budget -1`, the pinned llama.cpp compatibility set's verified prompt-cache reuse flag (`--cache-reuse 256` for the selected set), and DMR keep-alive `--keepalive -1`.

The `api` long-form model binding SHALL inject exactly `AI_MODEL_URL` and `AI_MODEL_NAME`. The backend keeps the total input plus requested output limit at `131072`, default temperature `0.8`, and one downstream request. Reasoning-on requests use the enabled reasoning budget; reasoning-off translation is a backend contract and must pass real-model proof because the research does not establish a stable generic per-request DMR toggle.

Prompt/KV reuse is enabled only in DMR memory. It SHALL have no application persistence. Explicit unload and platform restart must destroy the cache. The project patch removes the DMR request recorder. Every compatibility set has a mandatory privacy gate: the manifest identifies exact model, application image, DMR binary, and llama.cpp binary digests; API startup compares the live DMR, llama.cpp, and packaged-model identities before readiness. The selected set must also prove that request/response bodies do not reach logs or disk. An unevidenced or mismatched build fails `api` readiness. DMR request-history surfaces are never operational observability sources.

The 131072 context is a material inference-memory risk. OOM, unsupported flag, or engine rejection blocks readiness; the system must not silently reduce context, batch size, reasoning, or cache behavior.
The compatibility manifest records keep-alive `-1`. Controller status/load proof must observe the effective runner configuration and continued loaded state beyond the compatibility set's normal inactivity-eviction window. A missing, rejected, or mismatched keep-alive setting fails `api` readiness; inactivity eviction can never satisfy Stop or normal ready-state semantics.

## 5. Controller execution and backend alignment

The following names and contract are aligned with the backend owner:

- backend: `CONTROLLER_BASE_URL=http://controller:9090`
- controller: `MODEL_RUNNER_HOST=http://model-runner.docker.internal:12435`
- controller: `MODEL_ARTIFACT_REF=local/minicpm5-2b:q4_k_m-ec2d58016400`
- controller: `MODEL_SOURCE_SHA256=ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd`
- backend model binding: `AI_MODEL_URL`, `AI_MODEL_NAME`

The controller accepts only:

1. `GET /internal/v1/model/status` with no body or query, deadline 5 seconds;
2. `POST /internal/v1/model/load` with no body or query, deadline 10 minutes;
3. `POST /internal/v1/model/unload` with no body or query, deadline 60 seconds.

Only `X-Request-ID`, `X-Authority-Epoch`, and `X-Authority-Holder` are accepted. At request ingress, before waiting for controller serialization, the controller reads the authority view and requires the latter two headers to match the same current fresh backend authority row; failure rejects the call without enqueueing or spawning work. Callers cannot supply a model, backend, command, argument, URL, environment, or timeout. Any body, query, unknown method/path, or header used as an argument receives HTTP 400/404/405 before process execution.

Every parsed controller operation returns bounded JSON with `operation`, `outcome` (`succeeded`, `failed`, `indeterminate`), `observed_state` (`loaded`, `unloaded`, `unknown`), required `authority_epoch`, required `authority_holder`, `model_ref`, `source_sha256`, `runner`, `runtime_resources`, `observed_at`, and `error`. `authority_epoch` and `authority_holder` echo the authority accepted by the post-operation validation and MUST equal the backend's still-current values before a result can affect product or database state. `runner` contains only `available`, `dmr_version`, `engine`, `engine_version`, and required integer `keep_alive`; product success requires `keep_alive=-1`. `runtime_resources` contains only `sampled_at`, nullable `unified_memory_used_bytes`, nullable `unified_memory_total_bytes`, `unified_memory_source:"dmr_process|unavailable"`, nullable `disk_used_bytes`, nullable `disk_total_bytes`, `disk_source:"project_storage|unavailable"`, `metal:"enabled|disabled|unknown"`, and `metal_source:"dmr_process|unavailable"`; it has no CPU or host-total fields. A non-null `error` is exactly `{code,message,retryable}` with code `controller_unavailable|runner_unavailable|cli_failed|timeout|state_mismatch|invalid_request` and fixed safe text. Raw argv, stdout, stderr, paths, environment, logs, and inference content are never returned or logged. The sealed controller contract contains strict examples for status, load, and unload using this identical response shape.

HTTP 200 means only that the controller protocol was parsed. Product success requires `outcome=succeeded` and an observed state matching the requested action. Protocol errors use 4xx and an unavailable controller uses 503. Timeout, indeterminate result, and identity mismatch always fail closed.

The adapter maps fixed operations internally to the pinned plugin only:

- status: fixed `docker-model status` plus `docker-model ps` observation;
- load/warm: fixed `docker-model run --detach <configured MODEL_ARTIFACT_REF>`, followed by status/ps observation and the backend's content-safe warm probe;
- unload: fixed `docker-model unload <configured MODEL_ARTIFACT_REF>`, followed by status/ps observation proving it absent.

Application code SHALL NOT call `/engines/unload`, `/engines/ps`, `/engines/status`, other plugin-internal routes, or any undocumented DMR lifecycle route. Those routes are an implementation detail behind the pinned plugin compatibility boundary. Artifact deletion and inactivity eviction are not unload.

`MODEL_RUNNER_HOST` is required and validated as an exact allowed internal origin at controller startup. Missing, malformed, localhost, arbitrary host, or extra path values fail closed. There is no socket or localhost fallback. A timeout, plugin/runner mismatch, controller loss, keep-alive mismatch, or indeterminate observed state is failure; the backend cannot claim ready or unloaded. The backend never automatically retries load or unload; it may retry status once after one second of jitter. The controller never changes lifecycle state autonomously.

The controller uses one process-wide lifecycle mutex across status, load, and unload. It validates `public.controller_authority_epoch` three times with `mini_controller_epoch`: at request ingress; after acquiring the mutex immediately before spawning the fixed plugin operation; and after plugin completion plus observed-state verification, before releasing the mutex or returning. Every check requires both headers to match `epoch` and `holder_id`, with `heartbeat_at` no older than 6 seconds. A stale/missing authority, DB failure, holder/epoch change, or stale heartbeat at any check rejects or returns `state_mismatch`; if the operation already ran, its result is `indeterminate`, never success. The mutex remains held through the post-check, so old- and new-authority operations cannot overlap; a queued new-authority call is revalidated only after the old operation terminates.

## 6. Configuration and secrets

Compose SHALL be invoked with an explicit non-secret config file such as `config/runtime.mac.conf`; implicit project `.env` loading is prohibited. The repository `.env` file is neither created nor mutated. The committed configuration contains names, ports, paths, limits, and immutable digests but no secret values.

Secret values are runtime files under ignored `var/secrets/`, mounted with Compose `secrets:` and read through `_FILE` variables. There is one distinct credential file per login role; no service receives the owner password or another service's credential:

- `API_KEY_FILE=/run/secrets/api_key`;
- `mini_migrator LOGIN`: only `migrate`; may `SET ROLE mini_owner` for versioned DDL and has no application-entry permission;
- `mini_api LOGIN`: only `api`; `CONNECT`, schema `USAGE`, safe-view `SELECT`, and `EXECUTE` on business/authority `SECURITY DEFINER` procedures; no direct base-table/sequence writes;
- `mini_retention LOGIN`: only the backend scheduler's separate pool; execute `aggregate_closed_hours`, `purge_expired_metadata`, and retention-run procedures; no content/base-table or business-state access;
- `mini_backup LOGIN`: only `backup-scheduler`/`backup-prune`; approved-table/sequence `SELECT/USAGE` needed by `pg_dump` and `EXECUTE record_backup_run(...)`; no direct write/DDL/authority procedure;
- `mini_restore LOGIN`: owns only the isolated recovery database. In production it has only `CONNECT`, schema `USAGE`, and `EXECUTE record_restore_proof(...)`, with no base-table `SELECT` or other procedure; production `CONNECT` is revoked and its short-lived credential destroyed immediately after the drill;
- `mini_controller_epoch LOGIN`: only `controller`; `CONNECT`, schema `USAGE`, and `SELECT public.controller_authority_epoch`, with no other object/default privilege.

`mini_owner NOLOGIN` owns all schema/table/sequence/view/procedure objects and is never used at runtime. Every login role is `NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`. Migrations revoke all unintended `PUBLIC` grants and deny default privileges; credentials are delivered separately and never shared. Secret files must be regular, owner-readable only, not symlinks, absent from images/logs/manifests/backups, and rotated only through a separately authorized procedure. `web` receives no secret.

Required non-secret values are validated at startup; unset interpolation is a Compose configuration error rather than a default. Configuration drift is detected by recording the resolved Compose configuration hash after redacting secret paths/values.

## 7. Persistence, backup, retention, and restore

- `postgres-data` is a Docker-managed named volume. The only repository bind used for durable runtime data is `./var/backups/postgres`.
- `backup-scheduler` is a Compose-contained long-running service with no LAN port. At `03:00 UTC` daily it acquires PostgreSQL advisory lock `741291553` using `mini_backup`, and skips the cycle if held. It writes `pg_dump --format=custom --no-owner --no-privileges` to a mode-0600 `.partial`, verifies `pg_restore --list`, writes a `.sha256`, then atomically renames both to `mini-inference-YYYYMMDDTHHMMSSZ-<schema-version>.dump[.sha256]`.
- After each successful archive, and on every scheduled/catch-up/prune cycle, the scheduler deletes **all** completed archives and checksums whose age is strictly greater than 7 x 24 hours, including a lone stale valid archive. It never follows symlinks, traverses outside the backup root, or deletes a `.partial` file owned by a live cycle. Retention does not preserve an over-age “newest” exception. If no valid archive within the 7-day window remains, the cycle records failure and raises a critical freshness/recovery alert.
- On startup and after time discontinuity, the scheduler checks the latest successful `operational_runs(kind='backup')`. If no successful backup exists for the current UTC day or age exceeds 26 hours, it immediately performs exactly one locked catch-up cycle before scheduling the next `03:00 UTC` run. Duplicate starts are suppressed by advisory lock `741291553`. Backup age over 26 hours or absence of a current retained archive is a critical freshness failure.
- The backend, not an ops container, runs content-free aggregation hourly at `HH:05` and 30-day metadata retention daily at `02:15 UTC` through `mini_retention` and PostgreSQL advisory lock `741291552`. On startup it catches up missed buckets and a missed daily retention boundary before reporting operations healthy. It aggregates first, deletes strictly older-than-cutoff metadata through backend-owned fenced procedures in bounded transactions, and records counts/cutoff only.
- `restore-drill` verifies archive SHA-256 and restores through `mini_restore` only into `postgres-restore` on the isolated `restore` network and disposable volume. It runs backend-owned schema/version/count/token/no-prohibited-content assertions and writes a content-free evidence file. Only after the restore target is stopped does separate `restore-proof-recorder` attach to `data` and use the short-lived `mini_restore` production grant solely for `record_restore_proof(...)`; it cannot read or mutate live base tables. The production `CONNECT` grant is then revoked, the credential destroyed, and the disposable containers/volume/evidence removed.
- A live restore is a destructive recovery action requiring explicit user authorization, outage declaration, fresh pre-restore backup, exact archive digest, isolated rehearsal, and backend semantic verification. No command in this specification performs it.

Recovery objectives are not an SLA: RPO is at most the latest successful daily backup; restore-drill RTO is measured without a pass/fail duration threshold.

## 8. Health, readiness, and startup order

Container liveness means the process event loop responds; readiness is stricter:

- `postgres`: `pg_isready` succeeds and the expected database accepts the relevant least-privilege role.
- `backup-scheduler`: loop is live, advisory lock `741291553`/operational-run query succeeds, latest successful backup is no older than 26 hours after initial catch-up allowance, and backup directory containment is valid.
- `controller`: configuration/plugin checksum/DMR status and view-only epoch access are valid. Backend polls fixed status every 5 seconds; a status observation older than 10 seconds, failed authority validation, identity mismatch, or keep-alive mismatch makes lifecycle/inference state unavailable and fails closed. An authoritatively observed unloaded model is healthy.
- `api`: migrations are compatible and the exclusive PostgreSQL authority fence is held. A new leader first atomically marks every prior-epoch waiting/active request and running model operation `interrupted`, then performs current-authority reconcile unload; only after that, missed aggregation/retention catch-up, authoritative controller/DMR identity, effective keep-alive `-1`, and a matching passing privacy gate may inference/admin readiness succeed. Startup posture is observed unloaded.
- `web`: static health succeeds and its proxy allowlist/rejection/SSE configuration is loaded; frontend health does not imply API readiness.

Startup order is `postgres` healthy, `migrate` successful, `backup-scheduler` initial catch-up complete, `controller` healthy, `api` ready, then `web`. Compose dependency order is convenience only; every service independently performs bounded dependency checks and fails closed. Startup explicitly observes or performs supported unload before the backend reports unloaded. No inference is replayed or used as a readiness probe.

## 9. Observability transport

All services emit newline-delimited JSON to stdout/stderr for Docker's pinned local logging driver with bounded rotation. Required common fields are `timestamp`, `level`, `service`, `event`, `correlation_id`, `operation`, `outcome`, `duration_ms`, and safe error code where applicable. Prompt, completion, reasoning, tool arguments, authorization headers, secrets, request bodies, response bodies, raw DMR/plugin output, and unbounded labels are forbidden.

The backend is authoritative for request/queue/token/performance metrics and hourly aggregates. PostgreSQL is their durable source. Resource telemetry is deliberately **application and DMR inference-runtime usage**, never whole physical-Mac or Docker-VM utilization.

The authoritative flat `ResourceSnapshot` is `{status,sampled_at,cpu_percent,cpu_source,unified_memory_used_bytes,unified_memory_total_bytes,unified_memory_source,disk_used_bytes,disk_total_bytes,disk_source,metal,metal_source,reason_code}`. `status` is `available|partial|unavailable|stale`; source enums are exactly `api_container|dmr_process|project_storage|unavailable`. Fixed mappings are CPU=`api_container`, unified memory=`dmr_process`, disk=`project_storage`, and Metal=`dmr_process`; unavailable values are null (Metal `unknown`) with source `unavailable`, never zero or guessed. One unavailable field with others available yields `partial`; a sample older than 15 seconds is `stale` while preserving provenance; all unavailable yields `unavailable`.

CPU is only the `api` container's self/cgroup usage. Unified memory is only DMR model-process resident usage and a proven process/runtime limit. Disk is only repository/model/runtime storage usage and a proven project capacity/limit. Metal is only the DMR inference-engine state. `controller.runtime_resources` supplies evidenced DMR memory/project storage/Metal; the backend adds API-container CPU. No host helper, Engine socket, host-root mount, or physical-host API is introduced, and Docker VM/container values are never relabeled as physical-host totals. Missing fields do not by themselves make inference unready, but AC-025 cannot pass unless all four scoped values are evidenced.

The UI labels are exactly “API 容器 CPU”, “DMR 模型进程统一内存”, “项目/模型/运行时存储”, and “DMR 推理引擎 Metal”, and displays source and sample time.

The DMR privacy compatibility gate permits only transient in-memory inference and recorder state inside DMR, inaccessible from LAN clients and application components. It permits no request/response body persistence to disk or logs. The gate combines build/source evidence with one runtime canary and records every inspected surface at three timepoints: while a unique prohibited-content sentinel request is executing, immediately after it completes, and after an explicit DMR runner restart. At each applicable timepoint it scans every content-capable surface available through documented Compose and pinned DMR/plugin interfaces—application/controller/runner logs, repository binds and Docker-managed project volumes, DMR request-history output, and evidenced recorder storage—while separately proving that any transient recorder surface is unreachable from LAN/application components. The sentinel must occur zero times in disk or logs at every timepoint and zero times in any history/storage surface after restart. Evidence records exact versions/digests, canary hash (not text), inspected surfaces, commands, timestamps, reachability, and zero-match results. Any LAN/application reachability to recorder state, body in logs/disk at any timepoint, persisted history/storage after restart, inaccessible claimed surface, version mismatch, or missing evidence fails the privacy gate and `api` readiness; there is no fictitious disable setting or host-access fallback.

Alerts exist only as backend state rendered by `web` and as structured log events. No email, IM, desktop notification, webhook, remote log sink, or external telemetry service is configured.

## 10. Compatibility manifest and upgrades

The repository SHALL contain one immutable release manifest (implementation path `config/compatibility-manifest.json`) with:

- release/schema version and creation timestamp;
- source GGUF path, size, SHA-256, OCI reference, and OCI digest;
- `api`, `web`, `controller`, PostgreSQL, and job image digests and target architectures;
- controller plugin version/commit and binary SHA-256;
- Docker Desktop, Compose, project-built DMR commit/binary SHA-256, llama.cpp commit/binary SHA-256, and PostgreSQL versions;
- context, batch, temperature, reasoning, and cache settings;
- per-field resource provenance mapping and the manifest-bound DMR privacy-gate evidence identifier, inspected surfaces, exact build set, and pass result;
- resolved configuration hash and database migration version;
- the evidence bundle identifier for lifecycle, inference, privacy, backup/restore, browser, and network-boundary checks.

An upgrade changes the compatibility set as a unit. Any changed member requires renewed controller positive/negative proof, model load/inference/unload, cancellation, privacy scan, backup/restore drill, health/readiness, and browser/resource evidence. The project-built DMR and llama.cpp commits and binary SHA-256 values are independently pinned and compared with the live runtime identity before API readiness.

## 11. Rollout, rollback, and failures

### 11.1 Local rollout

1. Confirm private-interface binding, required Docker/Compose/DMR versions, free disk/unified memory, role-secret permissions, immutable digests, model source hash, and backup directory containment.
2. Package or verify the local model OCI artifact and write its digest to the manifest; recheck the source hash.
3. Render Compose configuration and reject published ports other than private-interface `8888` and `8080`, any socket/root/model-source mount, unpinned image, extra service, unexpected network attachment, shared database credential, or restore-to-live route.
4. Start `postgres`, run `migrate` once, then start `backup-scheduler` and prove its lock/catch-up/freshness behavior.
5. Start `controller`; prove its view-only epoch grant and run the mandatory lifecycle gate.
6. Start `api`; verify unloaded startup, authority fence, reconciliation, missed aggregation/retention catch-up, and readiness.
7. Start `web`; prove the exact proxy allowlist, `/internal/*` rejection, SSE streaming/reconnect, and real browser behavior.
8. Perform API, queue, metrics, privacy, backup retention, and isolated restore-drill proofs; record the evidence bundle and compatibility manifest. Evidence is not product approval.

### 11.2 Rollback

Rollback selects the immediately preceding fully evidenced compatibility manifest and image/model digests. Stop new admission; complete explicit unload; preserve logs and a fresh logical backup; run only backward-compatible database rollback approved by the backend owner; recreate services from previous digests; prove unloaded startup and the lifecycle gate before admission. If schema rollback is not proven safe, keep the appliance unavailable and request a separately authorized live recovery after an isolated restore rehearsal. Never roll back by editing tags, reducing safety settings, selecting another model/runtime, bypassing migrations, or pointing restore credentials at live PostgreSQL.

### 11.3 Failure policy

PostgreSQL/fence loss, stale authority view, DMR ambiguity, controller/plugin incompatibility, missing version identity, model hash mismatch, backup staleness/corruption, retention catch-up failure, or readiness mismatch fails closed as defined by the owning contract and raises a local alert. No inference retry replays content. Load/unload is not automatically retried after an indeterminate result; status is re-observed and an operator chooses the next fixed action. Disk pressure stops backup atomically and may stop admission according to backend policy; it never deletes live business data or the immutable GGUF.

## 12. Mandatory lifecycle feasibility gate

From the Compose-launched Linux ARM64 controller, with no Engine socket:

**Positive gate**

1. Prove `uname -m` is `aarch64`, plugin path/version/checksum match the manifest, and `MODEL_RUNNER_HOST` equals the allowed internal DMR origin.
2. Run the fixed status operation and observe a bounded unloaded/known result.
3. Run fixed load/warm for the configured model; observe plugin success, the single approved loaded model, successful content-safe warm probe, and backend transition to ready.
4. Run fixed unload; observe the approved model absent before Stop succeeds.
5. Inspect resolved Compose networking/mounts/ports: no socket; no LAN DMR/controller/PostgreSQL port; no second model/runtime.

**Negative gate**

1. Remove `MODEL_RUNNER_HOST`: controller startup and every lifecycle operation fail closed; no localhost/socket fallback.
2. Submit an unknown verb, model override, argument, query, and body: each is rejected before spawning the plugin.
3. Point to an unreachable allowed-form endpoint and force plugin timeout/non-zero exit: response is failed/indeterminate, never success; backend remains unavailable.
4. Make observed state disagree with plugin exit: `state_mismatch`, never ready/unloaded.
5. Confirm direct LAN attempts to `12435`, `9090`, and `5432` fail.

Any failed gate blocks MVP lifecycle acceptance. There is no fallback to undocumented HTTP calls, artifact deletion, inactivity eviction, an unattested host process, or Engine socket. Runtime replacement is governed by ADR-0002 and any further replacement requires another authorized ADR.

## 13. Exact local command allowlist

These commands are the implementation/runbook surface once their referenced files exist. They are local-only; they do not install host dependencies, commit, push, publish, or deploy remotely.

```sh
# Evidence-only preflight
docker version
docker compose version
docker model version
shasum -a 256 models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf

# Local OCI packaging; no --push
docker model package --gguf "$PWD/models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf" local/minicpm5-2b:q4_k_m-ec2d58016400
docker model list --json
shasum -a 256 models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf

# Render and inspect without starting
docker compose --project-name mini-inference --env-file config/runtime.mac.conf config

# Local rollout
docker compose --project-name mini-inference --env-file config/runtime.mac.conf up -d postgres
docker compose --project-name mini-inference --env-file config/runtime.mac.conf --profile ops run --rm migrate
docker compose --project-name mini-inference --env-file config/runtime.mac.conf up -d backup-scheduler controller
docker compose --project-name mini-inference --env-file config/runtime.mac.conf --profile ops run --rm lifecycle-proof
docker compose --project-name mini-inference --env-file config/runtime.mac.conf up -d api web

# Content-safe status/log inspection
docker compose --project-name mini-inference --env-file config/runtime.mac.conf ps
docker compose --project-name mini-inference --env-file config/runtime.mac.conf logs --no-color --since 30m api web controller backup-scheduler postgres

# Operational one-shot proofs
docker compose --project-name mini-inference --env-file config/runtime.mac.conf --profile ops run --rm backup-prune
docker compose --project-name mini-inference --env-file config/runtime.mac.conf --profile restore up --abort-on-container-exit --exit-code-from restore-drill restore-drill
docker compose --project-name mini-inference --env-file config/runtime.mac.conf --profile restore run --rm restore-proof-recorder
docker compose --project-name mini-inference --env-file config/runtime.mac.conf --profile restore rm -sfv postgres-restore restore-drill

# Graceful local stop; database volume and repository backups remain
docker compose --project-name mini-inference --env-file config/runtime.mac.conf stop web api controller backup-scheduler postgres
```

`down -v` outside the explicitly disposable `restore` profile, `docker system prune`, `docker model rm`, direct `psql` repair, registry push, remote contexts, and commands with arbitrary controller/plugin arguments are not authorized runbook commands.

## 14. PRD acceptance proof matrix

| AC | Operations evidence and owner seam |
|---|---|
| AC-001–007 | Resolved Compose topology; source before/after hash; manifest settings including keep-alive `-1`; real Metal load/warm/status/unload; one loaded identity; loaded-state survival beyond the normal eviction window; cache reuse then unload/restart non-reuse. Backend owns request-level observation. |
| AC-008–015 | Private-interface `8888` plus manifest/model settings; backend proof supplies auth, supported endpoint, reasoning, tool-call, unsupported-parameter, and context-boundary results. Operations proves no bypass route to DMR. |
| AC-016–022 | Single `api` replica, fence/readiness evidence, restart/stop logs, and backend queue evidence. Operations proves no replay service/broker and unloaded restart posture. |
| AC-023–024 | `web` on private `8080`, no login/secret, real-browser evidence; frontend/backend own Chinese states and actions. |
| AC-025 | The browser shows “API 容器 CPU”, “DMR 模型进程统一内存”, “项目/模型/运行时存储”, and “DMR 推理引擎 Metal” with exact per-field provenance and sample time. Values come only from container self-metrics and content-safe pinned DMR/plugin/project accounting; unavailable fields block AC-025 but not inference readiness. |
| AC-026 | Correlate one controlled request ID across backend metrics and UI for token/throughput/first-token/total-duration values without content. |
| AC-027 | Trigger one safe local alert; observe only UI and JSON logs; inspect configuration for absence of external sinks. |
| AC-028 | Evidence bundle records manifest, model digest, settings, prompt class (not body), timestamps, throughput, first-token latency, and duration; no numeric release threshold. |
| AC-029 | Complete a request, restart services, and verify backend-owned metadata/token fields persist in PostgreSQL. |
| AC-030 | Database/log/archive scans cover normal, reasoning, and tool-call cases. The manifest-bound DMR privacy gate additionally runs a unique canary, restarts the runner, and proves zero sentinel persistence across all documented/pinned log, storage, volume, and request-history surfaces; any unevidenced surface/build fails readiness and AC-030. |
| AC-031 | Backend-owned aggregation at `HH:05` and daily 30-day retention at `02:15 UTC` use `mini_retention`, advisory lock `741291552`, startup catch-up, fenced procedures, and controlled boundary timestamps to prove keep/delete behavior. No ops job deletes metadata. |
| AC-032 | `backup-scheduler` creates daily repository-contained archives atomically, catches up a missed run, flags age over 26 hours, locks duplicate cycles, deletes every archive/checksum strictly older than 7 days including a lone stale archive, and raises critical failure when no current valid archive remains. |
| AC-033 | `restore-drill` restores a verified archive only into isolated `postgres-restore`, validates metadata/tokens/privacy, and emits content-free evidence; separate `restore-proof-recorder` may only call `record_restore_proof(...)`, after which production CONNECT/credential and disposable resources are removed. |
| AC-034 | LAN scan reaches only configured `8888` and `8080`; direct DMR `12435` fails while authenticated inference via `8888` succeeds. |
| AC-035 | Contract-negative tests reject arbitrary verb/path/body/query/model/argument; resolved mounts prove no Engine socket; fixed status/load/unload succeed. |
| AC-036 | Evidence identifies fixed API key/no-login console/limited controller as accepted private-network risks and confirms private interface binding. |
| AC-037 | Resolved topology, listener inspection, image/manifest inventory, and egress configuration prove no public listener, second runtime/model, tool executor, or external alert sink. |

## 15. Open gates and residual risks

There is no outstanding product, architecture, or research decision. Implementation and gates are fully specified. Acceptance requires empirical evidence for 131072-context memory viability; selected DMR/plugin/engine reasoning and cache flags; the no-socket in-container lifecycle path; the manifest-bound privacy canary/restart/zero-persistence gate; scoped application/DMR/project resource provenance; and isolated restore correctness. An unavailable metric is explicit and may preserve readiness but cannot pass AC-025. No gate authorizes degraded parameters, invented physical-host values, host helpers, or alternate lifecycle mechanisms.

Residual accepted risks are single-node availability, the fixed weak API key, unauthenticated console, unauthenticated loopback-only DMR, narrowly privileged controller lifecycle authority plus read-only epoch view, project-built host-runtime availability, and daily-backup RPO. Independent technical/security review must verify topology, runtime identity binding, proxy policy, grants, scheduler, restore isolation, and configuration before product acceptance.
