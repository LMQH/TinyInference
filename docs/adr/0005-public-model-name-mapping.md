# ADR-0005: Persist one replaceable public model name

- **Status:** Effective for the approved 2026-09-23 requirement
- **Decision authority:** Product owner's model-name mapping request and strict-replacement clarification

## Decision

Store exactly one public model name in the existing PostgreSQL database. The default is `openbmb/MiniCPM5-2B-Q4_K_M`. The existing API authority changes it through a fenced, transactional administrative operation exposed only by the console proxy. Public API reads use that persisted value, and each admitted request captures the name for its response and metadata.

The internal DMR ref, GGUF source identity, controller lifecycle authority, one-model limit, and private-network topology remain fixed. The original public name is a reset value, not a second concurrently accepted alias.

## Rationale and consequences

PostgreSQL is already the authority for durable application state and is included in daily backup/restore. A single row avoids another configuration store. Reading the current name when a new public request begins makes a completed edit visible to subsequent requests without cache synchronization. Requests already admitted use their captured name while finishing. The console's existing unauthenticated private-network boundary also applies to this setting.
