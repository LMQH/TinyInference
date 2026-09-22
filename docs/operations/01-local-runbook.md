# Mini-Inference local operations runbook

## Status

This runbook is effective under ADR-0004 for the current Apple Silicon Mac. It is limited to the approved private `8888` and `8080` bindings; `8889`, `9090`, `5432`, and DMR host-loopback `12435` remain unexposed to the LAN. Never substitute `0.0.0.0`.

DMR is a project-built host-loopback facility. API and controller reach it only through `http://model-runner.docker.internal:12435`; Compose does not publish or proxy the DMR port, and no service mounts an Engine socket.

## Fail-closed prerequisites

Any empty empirical identity field is unresolved: Compose and image builds fail closed until Main records immutable image/base/plugin/model/runtime identities and measured resource limits. Do not replace an empty value with `latest`, a mutable tag, or a sample digest. Complete `docs/operations/03-compatibility-rollout.md` first.

Create `var/secrets` and `var/backups/postgres` inside the repository with mode `0700`. Both trees are excluded from Git and Docker build contexts. Create each declared secret as a regular, non-symlink file with mode `0600`: `api_key`, `postgres_bootstrap_admin_password`, `postgres_bootstrap_admin_dsn`, `mini_migrator_password`, `mini_migrator_dsn`, `mini_api_dsn`, `mini_retention_dsn`, `mini_controller_epoch_dsn`, `mini_backup_dsn`, `mini_restore_isolated_dsn`, `mini_restore_isolated_admin_dsn`, `mini_restore_proof_dsn`, `mini_api_password`, `mini_retention_password`, `mini_backup_password`, `mini_restore_password`, and `mini_controller_epoch_password`. The bootstrap-administrator password is mounted only into PostgreSQL; its corresponding DSN is mounted only into one-shot migration and proof-credential grant/revocation boundaries. The migrator DSN must authenticate `mini_migrator` using the separate migrator password. The isolated restore DSNs must target only the disposable `postgres-restore` cluster: `mini_restore` performs `pg_restore`, while `restore_admin` alone executes the content-safe assertion. Use distinct database credentials. Never echo, log, or commit a secret. The API key file must contain the approved value; the repository does not embed it.

The approved runtime uses a host-loopback DMR with llama.cpp Metal. It requires no host network, privileged mode, unrestricted device mounts, arbitrary host mounts, or Engine socket.

## Local rollout

Run only after all compatibility fields and secrets are resolved:

```sh
make runtime-build
make runtime-start
make resolve-config
make render
make migrate
make start
make status
make lifecycle-proof
make privacy-proof
```

The runtime build/start procedure builds the pinned project DMR and llama.cpp Metal compatibility set, verifies Metal enablement, extracts deterministic tokenizer metadata, and records only content-safe identities. It must preserve context `131072`, batch `2048`, cache reuse `256`, and keep-alive `-1`; an unsupported setting, OOM, or missing Metal blocks acceptance. The controller remains the exclusive production lifecycle owner.

The privacy proof remains a three-timepoint canary gate against the host-loopback DMR. It does not replace QA's browser-storage, restored-database, LAN, or DMR-store evidence.

Content-safe log inspection is limited to:

```sh
make logs
```

The web proxy exposes only the specified admin GET/action routes. The SSE route uses HTTP/1.1, no request/response buffering, no cache or compression buffering, a 45-second upstream timeout, and forwards only `Last-Event-ID` and `X-Request-ID`. It never proxies health, metrics, `/internal/*`, or the public inference listener.

## Health and failure policy

- PostgreSQL readiness targets the expected database. The built-in `postgres` credential is confined to PostgreSQL initialization and explicit one-shot migration/proof-credential-grant/revocation boundaries; long-running application services receive only their least-privilege credentials.
- Controller readiness requires exact configuration, plugin checksum, DMR identity, authority-view access, and a known observed state.
- API readiness requires schema compatibility, authority fence, reconciliation, controller/model identity, privacy evidence, and unloaded-or-ready state.
- Backup scheduler readiness requires database access, canonical backup containment, and a successful backup no older than 26 hours after the catch-up allowance.
- Web health proves only the static server/proxy process.

Database/fence loss, controller ambiguity, model/plugin/version mismatch, stale loaded-state proof, missing privacy evidence, backup corruption/staleness, or an unresolved manifest fails closed. Do not lower context, batch, reasoning, cache, or keep-alive values; do not select another model/runtime or add a fallback.

## Graceful local stop

```sh
make stop
```

This preserves the Docker-managed PostgreSQL volume, repository backups, compatible runtime artifacts, and packaged model. Never run `docker compose down -v`, `docker system prune`, remove `var/artifacts/compatible-runtime`, direct SQL repair, registry push, or a remote Docker context under this runbook.

## Rollback gate

Rollback requires the immediately preceding fully evidenced compatibility manifest and immutable image/model identities. Stop admission, explicitly unload, take a fresh logical backup, confirm schema backward compatibility with the backend owner, then recreate from the prior identities and repeat unloaded-startup and lifecycle proofs. If schema rollback is not proven, keep the Mac deployment unavailable. Live-data restore requires separate explicit user authorization and the procedure in `02-backup-restore.md`.
