# ADR-0001: Mini-Inference System Architecture

- **Status:** Effective
- **Date:** 2026-09-21
- **Owner:** 架构师
- **Decision authority:** Product owner’s recorded autonomous architecture authorization
- **Scope:** Mini-Inference MVP
- **Superseded in part:** `docs/adr/0002-project-built-compatible-dmr.md` replaces the Docker Desktop-managed runtime assumptions in §3.6 and §3.11; all other boundaries and invariants remain effective.
- **Inputs:**
  - `PROJECT_CONSTITUTION.md` — Effective
  - `AGENTS.md`
  - `docs/product/00-approved-decisions.md` — Approved
  - `docs/product/01-prd.md` — Approved
  - `docs/architecture/00-dmr-research.md` — Evidence

## 1. Context

Mini-Inference is a private-LAN/VPN, single-model inference appliance. The approved product requires an authenticated OpenAI-compatible text API, an unauthenticated Simplified Chinese operations console, exactly one active inference request, a bounded FIFO wait queue, explicit model lifecycle control, PostgreSQL-backed metadata retention, and Docker Model Runner (DMR) using llama.cpp. Application services run through one Docker Compose design; the current Apple Silicon Mac is the MVP verification environment, while a future private Linux server must remain possible without creating a second product architecture.

The system must preserve strict data minimization: prompt, response, reasoning, and tool-argument bodies are transient and must not be persisted or logged. DMR is unauthenticated and must remain inaccessible to LAN clients. Lifecycle control is isolated in a dedicated controller exposing only fixed capabilities; the selected design does not mount the Docker Engine socket.

DMR research establishes a material lifecycle constraint: supported explicit load, status, and unload operations are Docker Model CLI operations. Explicit unload is not part of the documented DMR HTTP lifecycle surface. An observed internal unload HTTP route is undocumented, globally scoped, and unsuitable as a durable contract.

This ADR decides the runtime class, backend language, durable component and trust boundaries, queue consistency class, persistence class, DMR integration boundary, portability and pinning posture, and fail-closed consequences. It does not redefine any product behavior.

## 2. Approved product facts preserved

This decision preserves the following approved facts without modification:

- The appliance is limited to a trusted private LAN or VPN and has no public-internet exposure.
- The existing MiniCPM5-2B GGUF is the only model and is immutable input.
- DMR with llama.cpp is the only inference runtime; Metal is used on the MVP Mac.
- Application services run through Docker Compose. The same Compose design must remain usable for a future private Linux server, although Linux deployment is not part of MVP acceptance.
- The stable product surface is the PRD-defined OpenAI-compatible text subset on LAN port `8888`, protected by the approved fixed Bearer key.
- The Simplified Chinese operations console has no login. Any private-network visitor may perform the lifecycle and queue operations approved by the PRD.
- At most one inference request may reach the model at once. Up to 20 additional requests wait in FIFO order for at most 30 minutes.
- Platform startup leaves the model unloaded. Start loads and warms it. Stop cancels the active request, clears waiting requests, and explicitly unloads the model.
- Restarted work is marked `interrupted` and is never replayed.
- PostgreSQL persists request metadata and token counts, but never prompt, response, reasoning, or tool-argument bodies. Metadata retention is 30 days; logical backups are daily, repository-contained, and retained for 7 days.
- Tool calls may be returned but are never executed.
- Unsupported behavior, context overflow, unavailable model, overload, timeout, cancellation, interruption, lifecycle failure, and authentication failure are explicit; there is no silent fallback.
- High availability, horizontal scaling, multiple models, a second inference runtime, public exposure, and durable prompt/KV cache are outside scope.

## 3. Decision

### 3.1 Runtime and deployment class

Mini-Inference SHALL be a **single-node Compose appliance** with one authoritative application backend. It is not a distributed system and SHALL NOT be designed as a microservice fleet.

The durable runtime components are:

1. a browser frontend;
2. one API/control backend;
3. one separately isolated privileged lifecycle controller;
4. PostgreSQL;
5. host-provided DMR using llama.cpp;
6. browser and LAN API clients outside the application trust boundary.

The frontend, backend, controller, and PostgreSQL remain distinct runtime boundaries. DMR remains a host facility reached only through approved internal integration paths. Operational jobs such as retention and logical backup MAY be separate Compose-invoked tasks, but they MUST NOT introduce another business, queue, or lifecycle authority.

### 3.2 Backend language: Go

The API/control backend SHALL be implemented in **Go**, using the standard Go concurrency and cancellation model as the architectural baseline.

Go is selected over Python for this appliance because:

- goroutines and bounded channels provide low-runtime-overhead concurrency suitable for many waiting client connections while preserving one active model request;
- Go’s HTTP stack supports incremental streaming proxying without requiring a separate application-server concurrency model;
- context cancellation composes across client disconnects, queue cancellation, Stop, downstream DMR requests, and shutdown;
- content-free operational metrics and structured telemetry can be collected with low per-request overhead;
- a compiled service can be delivered as a small, predictable container artifact without a mutable language runtime or virtual environment;
- the same service source and container build model are practical on Apple Silicon and Linux architectures.

This is a language and runtime-class decision, not a library decision. Frameworks, packages, build layout, concrete concurrency primitives, and implementation algorithms remain Tech Spec choices. The Tech Spec MUST preserve streaming backpressure and cancellation propagation rather than buffering complete streamed bodies.

### 3.3 Component ownership

#### Browser frontend

The frontend owns Simplified Chinese presentation, accessibility, navigation, local view state, and mapping authoritative backend outcomes into operator-visible states. It is not an authorization, lifecycle, queue, persistence, or model-state authority.

The frontend MUST communicate only with the API/control backend through the published application surface. It MUST NOT communicate directly with DMR, PostgreSQL, the Docker Engine, or the lifecycle controller. It MUST NOT infer successful Start or Stop from button submission; only an authoritative backend result may change the displayed lifecycle outcome.

A frontend failure MUST NOT prevent authenticated API clients from using an otherwise healthy inference API.

#### API/control backend

The Go backend is the sole authority for:

- public API validation, inference authentication, and the supported OpenAI-compatible subset;
- model and request lifecycle semantics visible to clients;
- queue admission, FIFO ordering, timeout, cancellation, dispatch, and overload decisions;
- enforcing exactly one downstream inference request;
- streaming adaptation and cancellation propagation between clients and DMR;
- deciding when a lifecycle operation is authorized by the product contract and requesting its execution from the controller;
- durable metadata, token accounting, retention policy, backup policy ownership, audit-safe structured logs, and user-visible metrics;
- reconciliation of nonterminal request metadata after restart.

It MUST validate all authoritative behavior server-side. Browser state, hidden controls, client-side validation, DMR behavior, and controller responses do not replace backend validation.

The backend MUST NOT mount or access the Docker socket. It MUST NOT execute tools requested by the model. It MUST NOT use DMR request-history facilities as its business record or metrics authority.

#### Privileged lifecycle controller

The controller is a narrow execution boundary, not a business authority. It SHALL expose only fixed, allowlisted capabilities for DMR status, load/warm, and unload. It MAY expose the minimum read-only host/runtime status required by those fixed status capabilities, but it MUST NOT expose a shell, arbitrary Docker operations, arbitrary command arguments, image/container management, model deletion, or a general proxy to the Docker API.

The controller SHALL package a pinned Linux ARM64 Docker Model CLI/plugin and use an explicit `MODEL_RUNNER_HOST` to reach the internal-only DMR endpoint. It SHALL NOT mount the Docker Engine socket when direct DMR transport is available. It remains separately containerized, unavailable from LAN ingress, reachable only from the backend over a dedicated internal control path, and denied unnecessary application/data-plane connectivity except for a read-only PostgreSQL authority view used to reject stale callers.

The backend owns whether and when an operation is requested. The controller only executes an allowed operation and reports a bounded result. It MUST NOT autonomously alter product lifecycle state.

#### PostgreSQL

PostgreSQL is the durable source of truth for approved request metadata, token counts, lifecycle outcomes, retention evidence, and coordination needed to fence the single queue authority. It is not a prompt store, response store, reasoning store, tool-argument store, streaming buffer, model cache, or replay queue.

Only the backend, narrowly scoped approved database operational tasks, and the controller's read-only authority-check role may access PostgreSQL. The controller role may read only the current authority holder and epoch; it cannot read request metadata or mutate data. PostgreSQL SHALL have no LAN-published application port.

#### Docker Model Runner

DMR is the sole inference runtime and model-execution boundary. It receives only a request that has passed backend authentication, validation, lifecycle admission, and single-active scheduling. It is not authoritative for public authentication, queue semantics, product lifecycle state, retention, or supported-parameter policy.

DMR’s unauthenticated API SHALL remain internal and MUST NOT be published to the LAN. DMR request inspection or telemetry MUST NOT be used where it could persist or log prohibited content. The Tech Spec and verification plan MUST prove that the selected DMR configuration and operational procedures do not retain prohibited request or response bodies.

#### Browser and LAN clients

Browser and LAN clients are outside the trusted application boundary even though the deployment network is trusted. API clients receive only the approved authenticated inference surface. Console clients receive the intentionally unauthenticated control surface approved by the PRD. Neither client class receives network reachability to PostgreSQL, the controller, the Docker socket, or DMR.

### 3.4 Queue consistency and Redis exclusion

Redis is **excluded**.

The live execution queue SHALL be process-local, bounded, non-durable state owned by the single backend instance. This matches the product semantics: queued requests are tied to live client connections, restart turns them into `interrupted`, and they must never be replayed.

Correctness SHALL be preserved as follows:

- exactly one backend instance may hold queue-admission and dispatch authority;
- the backend MUST obtain an exclusive PostgreSQL-backed authority fence before becoming ready to accept inference or lifecycle work;
- any additional or replacement backend instance that cannot obtain authority MUST fail closed and MUST NOT admit requests;
- the active request and FIFO wait queue remain in the authority holder’s memory, with PostgreSQL retaining only approved metadata, lifecycle facts, and fencing state—not replayable request bodies;
- after backend restart, previously nonterminal durable records are reconciled to `interrupted` before new work is admitted; no queued body is reconstructed or replayed.

This is a single-writer, strongly serialized queue consistency class. It intentionally trades availability for simple, auditable correctness. Redis, a durable message broker, or a distributed lock service would add a second coordination authority without satisfying a current product requirement.

### 3.5 Communication paths

The following communication classes are permitted:

- LAN API client → API/control backend, through the approved authenticated inference ingress;
- LAN browser → frontend, and frontend → API/control backend, through the approved private-network console surface;
- API/control backend → DMR, over the documented internal inference API for validated inference and non-lifecycle availability checks;
- API/control backend → PostgreSQL, over a private data network;
- API/control backend → lifecycle controller, over a dedicated private control network using only the fixed lifecycle/status contract;
- lifecycle controller → internal DMR endpoint, solely through the pinned, allowlisted Docker Model CLI adapter using explicit `MODEL_RUNNER_HOST`;
- lifecycle controller → PostgreSQL read-only authority view, solely to validate the current backend authority epoch before lifecycle execution;
- approved database operational task → PostgreSQL and repository-contained backup storage, limited to retention, backup, and restore duties.

The following paths are prohibited:

- any LAN client or browser → DMR;
- any LAN client or browser → PostgreSQL, Docker socket, or lifecycle controller;
- frontend → DMR, PostgreSQL, Docker socket, or lifecycle controller;
- any application component → Docker Engine socket;
- lifecycle controller → arbitrary command execution, unrestricted Docker API functionality, or application-code calls to plugin-internal lifecycle routes;
- DMR → business database authority or queue authority;
- any component → a second inference runtime;
- any model output → tool execution or external side effect;
- any component → persistence or logging of prompt, response, reasoning, or tool-argument bodies.

No deployment convenience, reverse proxy, debugging port, or observability endpoint may weaken these prohibitions.

### 3.6 DMR lifecycle integration and fail-closed gate

Explicit lifecycle control SHALL use a narrow adapter around the **supported Docker Model CLI lifecycle operations**. The controller MUST NOT call an undocumented DMR unload HTTP endpoint, interpret artifact deletion as unload, rely on inactivity eviction as Stop, or report unload success merely because inference traffic ceased.

Before lifecycle implementation is accepted, the Tech Spec must define and the target-environment verification must prove, from inside the controller container launched by the project Compose design, all of the following:

1. the pinned Linux ARM64 Docker Model plugin is discoverable and executable;
2. explicit `MODEL_RUNNER_HOST` reaches the actual Docker Desktop DMR from the controller without a Docker Engine socket mount;
3. allowlisted status, explicit load/warm, and explicit unload complete successfully against the approved model;
4. exit status and observed runtime state distinguish success from failure;
5. arbitrary command names and arbitrary arguments cannot cross the controller contract;
6. Stop is not reported complete until explicit unload is observed;
7. deployment inspection proves that neither the Docker Engine socket nor a LAN-published DMR/controller port exists;
8. removing `MODEL_RUNNER_HOST` makes the controller fail closed rather than silently choosing a socket or localhost fallback.

If that integration cannot be proven executable in the controller container on Docker Desktop, **MVP lifecycle acceptance is blocked**. The implementation MUST NOT fall back to an undocumented HTTP endpoint, a timer, inactivity eviction, artifact deletion, a host-installed application daemon, a socket mount in the main backend, or a false-success state. Any replacement lifecycle architecture requires a new ADR and user authorization.

Controller loss, CLI incompatibility, or an indeterminate unload result SHALL make lifecycle control unavailable and prevent the backend from claiming the model is unloaded or ready. The backend MUST reject new inference whenever it cannot establish an authoritative safe model state.

### 3.7 Cancellation and streaming invariants

The backend SHALL proxy streams incrementally and propagate cancellation across the full request path. Client disconnect, individual queue cancellation, queue timeout, Stop, and service shutdown must terminate eligibility for later execution. Stop completion additionally requires the controller’s explicit unload result.

No component may transform a cancellation into success, continue a cancelled queued request, or retain a complete content body for metrics or retry. Retries MUST NOT cause automatic replay of an inference request. Exact transport behavior and state transitions belong to the Tech Spec.

### 3.8 DMR exposure and lifecycle authority invariants

DMR is the high-risk runtime boundary:

- DMR is unauthenticated and therefore MUST have no LAN-published listener or reverse-proxy route. Only the backend and lifecycle controller may reach its internal endpoint for their distinct permitted purposes.
- No application component, including the controller, mounts the Docker Engine socket in the selected architecture. The approved constraint that only the controller may hold socket authority is satisfied without granting that authority.
- The Docker Model CLI/plugin is the lifecycle compatibility boundary. Application code MUST NOT call its underlying lifecycle routes directly or treat them as public stable REST contracts.
- Network isolation does not make a broad controller API safe; the controller contract itself MUST be allowlisted and non-extensible by caller input.
- The controller receives no inference content, public API credentials, general database credentials, or frontend traffic. It receives only a dedicated read-only credential for the current-authority view.
- The backend is intentionally authorized to request fixed lifecycle operations, so backend compromise can yield model load/unload authority; this is an accepted residual risk of the no-login private control surface. It still cannot yield arbitrary lifecycle arguments or Docker Engine access. The controller MUST validate the current PostgreSQL authority epoch to reject stale backend instances. Frontend compromise reaches lifecycle only through the same fixed backend contract.

### 3.9 Persistence and data-minimization class

Approved metadata is durably relational in PostgreSQL. Live request bodies, streamed output, reasoning content, tool arguments, queue contents, and prompt/KV cache are ephemeral only.

Every telemetry and failure path—including DMR diagnostics, reverse-proxy access logs, panic/error reporting, metrics labels, controller output, database backups, and restore evidence—MUST preserve the no-content rule. Metrics SHALL use bounded, non-content dimensions. DMR may transiently hold request and response bodies only as part of in-memory request processing or a bundled in-memory recorder that is inaccessible from the LAN and application services; those bodies MUST NOT reach disk, structured logs, metrics, or project persistence. Compatibility verification SHALL scan during a canary request, immediately after it, and after runner restart. Any body-bearing request-history surface reachable from the LAN or application, or any body persistence after restart, blocks readiness.

Loss of PostgreSQL or loss of the exclusive authority fence SHALL prevent new inference and lifecycle admission. This availability cost is accepted to avoid successful work without durable metadata or multiple simultaneous queue authorities.

### 3.10 Portability posture

The project SHALL maintain one logical Compose topology and the same component boundaries on the MVP Mac and a future private Linux server. Portability means preserving contracts and topology, not pretending host runtimes are identical.

Host variation is confined to documented integration seams for:

- the internal DMR endpoint supplied by the platform;
- the supported Docker Model CLI/runner integration used by the controller;
- read-only application, DMR-process, and project/runtime storage measurements needed for approved console metrics;
- architecture-specific immutable container artifacts.

Physical-host and Docker-VM totals are outside the approved metric contract and MUST NOT be inferred, relabeled, or implemented through host helpers.

The backend’s public behavior, queue semantics, trust boundaries, persistence semantics, and DMR non-exposure invariant MUST NOT vary by host OS. Host detection MUST NOT select an undocumented lifecycle fallback. A platform on which the required supported integration cannot be proven is unsupported until separately designed and verified.

Future Linux deployment and acceptance remain out of MVP scope. Portability work in the MVP is limited to avoiding a second Compose architecture and keeping the seams above explicit.

### 3.11 Version and pinning posture

All controllable runtime artifacts and dependencies SHALL be reproducibly pinned; floating `latest` tags are prohibited. This includes application images, PostgreSQL, the Docker CLI/plugin packaged for the controller, Go toolchain and module dependencies, frontend dependencies, and the model artifact identity/content.

Docker Desktop DMR bundles its llama.cpp engine and does not currently provide a documented independent engine-commit pin. Therefore the project SHALL:

- record the verified Docker Desktop, Compose, DMR, Docker Model plugin, and reported inference-engine versions as a compatibility set;
- constrain operation to a verified compatibility set rather than claim an unavailable independent llama.cpp pin;
- treat upgrades to any member of that set as requiring renewed lifecycle, inference, cancellation, privacy, and portability evidence before adoption;
- pin the Linux runner version when future Linux deployment is separately authorized.

A recorded version is evidence, not a substitute for runtime verification.

### 3.12 Failure isolation and consequences

- **Frontend failure:** API inference may continue; operator UI is unavailable.
- **Backend failure or restart:** live and waiting requests are interrupted, never replayed, and reconciled in PostgreSQL before new admission. The model returns to the product’s unloaded startup posture through the verified lifecycle path; inability to prove that posture keeps inference unavailable.
- **PostgreSQL failure or lost authority fence:** new inference and lifecycle admission fail closed. No in-memory success may bypass required durable metadata.
- **DMR failure or loss of trustworthy readiness:** the backend stops admission, exposes an explicit unavailable state, and never silently selects another runtime.
- **Controller or CLI adapter failure:** lifecycle operations fail explicitly; no alternate path is used, and the backend does not claim Stop, unload, or readiness without evidence.
- **Client disconnect or cancellation:** the request loses eligibility to execute or continue, and cancellation is propagated downstream.
- **Controller compromise:** model lifecycle and internal DMR access may be abused. Isolation, minimality, fixed operations, pinning, and independent security review reduce but do not eliminate this residual risk; the absence of a Docker Engine socket prevents direct general Docker control.
- **Backend compromise:** the attacker may invoke the same fixed model lifecycle operations available to the unauthenticated private console, but cannot supply arbitrary lifecycle arguments, bypass controller epoch validation, or obtain Docker Engine access.
- **Single-node failure:** service is unavailable. This is consistent with the approved absence of HA and formal SLA.

## 4. Alternatives considered

### 4.1 Python backend

Python with an asynchronous web framework is viable and would offer rapid development and familiar ML ecosystem integration. It was not selected because the gateway’s dominant work is long-lived concurrent HTTP streaming, cancellation, queue coordination, and metrics rather than in-process ML. Go offers lower baseline runtime overhead, simpler deployment as a compiled artifact, and one native cancellation model across HTTP and concurrency boundaries. The consequence is that implementation contributors need Go proficiency and must use DMR as an external runtime rather than rely on Python ML libraries.

### 4.2 Multiple backend services or microservices

Separating API gateway, queue manager, metrics service, lifecycle orchestrator, and persistence service was rejected. It would add network contracts, partial-failure modes, duplicated state, and operational weight to a single-owner appliance. The lifecycle controller remains separate because unauthenticated DMR lifecycle authority is a genuine trust boundary, not because independent scaling is needed.

### 4.3 Redis-backed or broker-backed queue

Rejected because there is one backend authority, waiting work is coupled to live client connections, restart must interrupt rather than replay, and HA is out of scope. Redis would add deployment, persistence, security, and split-brain concerns without improving the approved semantics.

### 4.4 PostgreSQL as a durable inference work queue

Rejected. Durable request replay conflicts with the approved `interrupted`-on-restart behavior and would require persisting or reconstructing content that must remain transient. PostgreSQL remains the metadata source of truth and authority fence, not a body-bearing work broker.

### 4.5 Multiple backend replicas with distributed coordination

Rejected as outside the single-node, no-HA product scope. It would require a different queue consistency model and materially expand failure and authorization boundaries.

### 4.6 Docker Engine socket in application containers

Rejected because the bare Engine socket does not transport the required DMR lifecycle operations and grants unnecessary host-level Docker authority. The separately isolated controller uses the pinned Docker Model CLI/plugin with explicit internal DMR transport and no socket mount.

### 4.7 Direct browser or LAN access to DMR

Rejected because DMR is unauthenticated, does not enforce the approved public contract, and can load or execute models outside backend queue and validation authority.

### 4.8 Undocumented DMR HTTP unload or inactivity eviction

Rejected. The internal unload route is not a supported public lifecycle contract, and inactivity eviction cannot satisfy explicit Stop semantics. Neither may be used as a fallback.

### 4.9 Host-resident controller daemon

Rejected under the current architecture because application services must run through Compose and the result would create a host-specific deployment path. If containerized Docker Model CLI execution proves infeasible, acceptance blocks pending a newly authorized architecture; the constraint is not silently weakened.

## 5. Consequences

### Positive

- A single Go backend keeps queue, cancellation, streaming, metrics, and business lifecycle semantics in one auditable authority.
- Process-local FIFO state exactly matches interruption-without-replay semantics.
- PostgreSQL fencing prevents accidental duplicate backend authorities without adding Redis.
- The controller isolates DMR lifecycle authority from LAN ingress and inference content without mounting the Docker Engine socket.
- DMR remains replaceable only through a future architecture decision rather than leaking runtime-specific behavior into clients.
- Explicit compatibility-set recording is honest about Docker Desktop’s bundled engine while retaining reproducibility evidence.
- One Compose topology preserves the approved Mac-to-Linux portability posture without claiming unverified Linux support.

### Negative and accepted tradeoffs

- The backend and PostgreSQL are availability-critical single points of failure.
- PostgreSQL unavailability rejects new work even if DMR could technically answer it.
- The controller depends on tightly isolated internal DMR TCP reachability; misconfigured exposure of that unauthenticated endpoint is a high-impact risk requiring independent security review.
- Go may offer less direct access to Python-centric ML tooling, which is acceptable because inference stays in DMR.
- Containerized Docker Model CLI integration on Docker Desktop is a hard feasibility gate and may block MVP lifecycle acceptance.
- Exact llama.cpp commit pinning is unavailable on Docker Desktop; compatibility is managed at the DMR/Desktop release level with revalidation.
- Fail-closed state handling may reduce availability when DMR or controller status is ambiguous.
- Resource metrics intentionally describe the managed application and DMR runtime rather than the whole physical host; explicit provenance prevents container or VM measurements from being misrepresented.

## 6. Downstream Tech Spec obligations

The Tech Spec SHALL define, without changing this ADR or the PRD:

- frontend/backend interface contracts and the single authoritative contract source;
- internal backend/controller contract and its complete allowlist;
- public and internal error schemas and lifecycle state transitions;
- queue admission, fencing, cancellation, timeout, shutdown, and restart reconciliation behavior;
- PostgreSQL data model, indexes, retention mechanism, backup/restore procedure, and authority-fence representation;
- content-safe streaming, logging, metrics, and DMR privacy controls;
- DMR request translation and supported-parameter enforcement;
- exact controller image, pinned CLI/plugin compatibility, explicit `MODEL_RUNNER_HOST`, no-socket network topology, and the mandatory in-container lifecycle proof;
- readiness semantics that fail closed when PostgreSQL fencing, DMR state, or controller state is unavailable;
- application/DMR/project resource metric adapters with explicit provenance for the verified Mac and the future Linux seam, without a physical-host helper;
- immutable version manifest and upgrade revalidation procedure;
- verification scenarios for all PRD acceptance criteria and every fail-closed gate in this ADR.

Concrete database tables or fields, endpoint payload fields, error codes not already fixed by the PRD, file names, functions, libraries, configuration keys, algorithms, command syntax, and request ordering are intentionally deferred to the Tech Spec.

## 7. Compliance with authorization

This ADR is **Effective**, not Draft, because the product owner explicitly authorized the architect to select Go or Python, component boundaries, implementation contracts, queue and persistence classes, trust boundaries, portability invariants, and the DMR lifecycle posture within the approved product facts. It changes no product behavior and authorizes no public exposure, deployment, secret access, commit, or infrastructure operation.

No architecture decision remains unresolved. The controller’s in-container Docker Model CLI execution is a mandatory feasibility and acceptance gate, not an undecided fallback: failure blocks lifecycle acceptance and requires a newly authorized ADR rather than weakening Stop semantics.
