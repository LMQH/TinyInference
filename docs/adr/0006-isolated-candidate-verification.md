# ADR-0006: Isolated candidate verification phase

- **Status:** Accepted 2026-09-23 by the product owner's explicit “批准，执行”.
- **Related:** ADR-0002 §Decision item 4 and ADR-0005.

## Context

A new image or schema invalidates prior exact-build evidence. The current API requires `release.status=verified` and completed privacy evidence before it listens, while that evidence requires running the new image. This circular gate blocks acceptance of the model-name mapping release.

## Decision

Add a distinct `candidate` phase to the compatibility manifest and API startup gate. It runs only under a dedicated, locally isolated Compose verification configuration with loopback-only host bindings, a disposable PostgreSQL volume, separate candidate backups, and the same pinned images and host-loopback DMR identity intended for release. The ordinary production Compose configuration continues to require `release.status=verified`, completed privacy evidence, and no unresolved fields. A candidate never serves production clients or migrates the live database.

Before a candidate controller accesses the sole DMR lifecycle authority, quiesce the current web/API/controller containers so they cannot execute lifecycle operations. The current verified containers may be paused and retained as the exact recovery path; prove that they resume with the same container/image identities before the longer candidate run. If that recovery rehearsal fails, do not start the candidate. A candidate controller must also reject startup without a guard created only after quiescence. Verify the candidate topology, configuration hash, image digests, and DMR identity before startup. Collect Docker/DMR, browser, privacy, backup/restore, network, and resource evidence against the candidate image IDs and immutable runtime/model identities. After independent QA and security approval, record those evidence IDs and promote the manifest to `verified`; re-check that all operational settings and images match the evidenced candidate before ordinary local rollout. A failed or incomplete candidate is never promoted.

## Alternatives

Marking the candidate `verified` before running it would claim nonexistent evidence and is rejected. Reusing old evidence would not describe the new image and is also rejected.

## Consequences

ADR-0002's startup requirement for completed privacy evidence remains mandatory for production; the isolated candidate phase is the sole pre-release exception. It does not permit a second model, public API contract change, LAN exposure of DMR/admin, Docker socket mount, or an unchecked runtime identity. The disposable database proves migration and restore behavior but does not authorize live-data restore or live schema migration. Local rollout retains the existing backup and rollback gates. If a future deployment target or second runtime is proposed, this exception must be reassessed.
