# Backup, retention, and restore drill

## Daily backup behavior

`backup-scheduler` is a Compose-contained service. At `03:00 UTC`, and once on startup when today's successful run is absent or the last success is older than 26 hours, it runs a fixed backup cycle. Advisory lock `741291553` suppresses duplicate cycles.

A cycle writes a mode-`0600` custom-format `pg_dump` to `.partial`, validates it with `pg_restore --list`, creates a SHA-256 sidecar, and atomically renames both to:

```text
var/backups/postgres/mini-inference-YYYYMMDDTHHMMSSZ-6.dump
var/backups/postgres/mini-inference-YYYYMMDDTHHMMSSZ-6.dump.sha256
```

The job rejects a non-canonical root, symlinks, and unexpected entries. Every completed archive and checksum strictly older than `604800` seconds is deleted; there is no “keep newest” exception. If no valid in-window archive remains, the job fails and the backend records a critical freshness condition. `.partial` files are never presented as valid archives.

Fixed local actions:

```sh
make backup
make backup-prune
```

The API-owned scheduler, not an operations job, performs hourly aggregation at `HH:05 UTC` and strict 30-day metadata retention at `02:15 UTC` under advisory lock `741291552`. Operations scripts never delete database metadata.

## Disposable restore drill

Select only a basename produced by the current schema version. Record and independently approve its SHA-256 before the drill. Then run:

```sh
python3 ops/restore-drill/select-backup.py mini-inference-YYYYMMDDTHHMMSSZ-6.dump
make restore-drill
make restore-record
make restore-revoke-proof
make restore-clean
```

The drill verifies the checksum and archive list and restores only to `postgres-restore` on the internal `restore` network and disposable `restore-data` volume. The `mini_restore` connection performs only `pg_restore`; it never invokes the schema assertion. A separate DSN for that disposable cluster's `restore_admin` calls backend-owned `assert_restore_schema()`. The assertion returns only request count and input/output/reasoning token totals and verifies the schema/content-minimization contract. The evidence volume contains only those counts, timestamps, status, and a run UUID.

Run `restore-record` only after `postgres-restore` has stopped. The target opens a fixed, short-lived production credential window immediately before recording: a one-shot administrator job refuses active `mini_restore` sessions, reads only the dedicated password and administrator DSN secrets, grants database `CONNECT`, and grants EXECUTE only on `record_restore_proof(...)`. The recorder then attaches separately to `data` and invokes that function. The target unconditionally runs the revoker after grant/record success or failure; revocation failure fails the target. Revocation removes all public-function EXECUTE from `mini_restore`, revokes database `CONNECT`, and sets only its password to null. `restore-revoke-proof` remains available as an explicit recovery action. The repository-local DSN file remains in place so ordinary Compose rendering is not broken, but its credential is invalid outside the bounded window. Run `restore-clean` afterward. `restore-clean` removes only explicitly disposable restore containers and volumes; it never touches `postgres-data` or repository backups.

Any checksum mismatch, malformed basename, archive-list failure, schema mismatch, unexpected evidence field, or database error fails closed. Neither archive validation nor a recorded proof substitutes for an actual restore.

## Live recovery boundary

No Make target restores the live database. A live restore is destructive and requires separate exact user authorization, an outage declaration, a fresh pre-restore backup, the approved archive digest, a successful isolated rehearsal, a backend-owned semantic repair/verification plan, and a rollback decision. Without all of these, keep the service unavailable and do not point restore credentials or `pg_restore` at live PostgreSQL.

RPO is the latest successful daily backup. Restore-drill RTO is measured evidence, not a pass/fail SLA.
