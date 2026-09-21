# Compatibility identity, rollout, and rollback

## Unresolved-by-design manifest

`config/compatibility-manifest.json` is schema-complete but initially `unresolved`. Null empirical fields are not release values. Startup/build configuration fails until Main observes and records them. `config/compatibility-manifest.schema.json` permits null only during resolution; a `verified` release must have no unresolved fields and must carry a passed, matching privacy gate.

The compatibility set is atomic: Docker Desktop, Engine, Compose, project-built DMR commit/binary digest, llama.cpp commit/binary digest, Docker Model plugin commit/binary checksum, PostgreSQL, build bases, application/job images, model OCI digest, tokenizer-metadata artifact, migrations, resolved Compose configuration, settings, and evidence IDs. Changing any member invalidates affected lifecycle, inference, reasoning, tokenizer/context, cache, cancellation, privacy, backup/restore, resources, LAN, and browser evidence.

## Exact empirical-resolution commands for Main

These are local identity observations only; they do not start application services, push, deploy, or modify a remote environment:

```sh
docker version
docker compose version
docker model version
docker info --format '{{json .}}'
shasum -a 256 models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf
```

Resolve each chosen base/postgres tag to its `linux/arm64` registry digest before entering it in `config/runtime.mac.conf` and the manifest. Repeat this command for the approved Go builder satisfying both the Go 1.24 application modules and pinned plugin commit, distroless static runtime, Node builder compatible with `web/package-lock.json`, nginx-unprivileged runtime, PostgreSQL client, and PostgreSQL server:

```sh
docker buildx imagetools inspect --format '{{json .Manifest}}' IMAGE:EXACT_VERSION
```

Record only an observed `sha256:<64 hex>` for the `linux/arm64` manifest. Set each image value as `repository@sha256:...`; never retain the mutable tag as the runtime identity.

Choose and record one Docker Model Runner source commit, then produce the checksum candidate without producing a controller image:

```sh
docker buildx build --platform linux/arm64 --target plugin-export \
  --build-arg GO_BUILDER_IMAGE='REPOSITORY@sha256:OBSERVED' \
  --build-arg MODEL_RUNNER_SOURCE='https://github.com/docker/model-runner.git' \
  --build-arg MODEL_RUNNER_COMMIT='OBSERVED_40_HEX_COMMIT' \
  --build-context controller-src=./services/controller \
  --output type=local,dest=var/artifacts/docker-model-plugin \
  -f ops/images/controller.Dockerfile ops/images/context
shasum -a 256 var/artifacts/docker-model-plugin/docker-model
```

Record that checksum in `ops/images/checksums/docker-model-plugin.json`, `config/runtime.mac.conf`, and `config/compatibility-manifest.json`. The final controller build recompiles the same commit and fails unless the result matches. Record the plugin version reported by the verified binary/DMR compatibility set; do not infer it from a branch name.

Build the compatible host runtime, package exactly one immutable model locally, and return it to a stable unloaded state:

```sh
make runtime-build
make runtime-start
```

`runtime-build` verifies pinned DMR and llama.cpp commits, applies the repository patches that disable request recording and produce authoritative reasoning-token usage, builds the Metal host binaries, and extracts only the deterministic GGUF metadata prefix required by the API tokenizer into `var/artifacts/tokenizer/tokenizer.gguf`. `runtime-start` checks source size/hash/mode, packages without push into the project-local DMR store, applies `--keep-alive=-1`, context `131072`, batch `2048`, and cache reuse `256`, waits for a stable unloaded initial state, and records the model/runtime hashes in `var/artifacts/compatible-runtime/state/runtime.json`. Record those exact hashes and model digest in the runtime config and compatibility manifest before building application images. The flow never deletes, renames, chmods, rewrites, or pushes the source GGUF.

After dependency lockfiles exist and all base/plugin fields are resolved:

```sh
make build-images
docker image inspect --format '{{.Id}}' "$(cat var/artifacts/images/api.iid)"
docker image inspect --format '{{.Id}}' "$(cat var/artifacts/images/web.iid)"
docker image inspect --format '{{.Id}}' "$(cat var/artifacts/images/controller.iid)"
docker image inspect --format '{{.Id}}' "$(cat var/artifacts/images/jobs.iid)"
```

`build-images` records the observed local immutable image IDs in the runtime config and manifest. Separately record the PostgreSQL image digest/version, Docker/Compose/Desktop/DMR/plugin/engine versions, measured resource limits, release timestamp, and exact evidence identifiers. Do not set `release.status=verified` or `privacy_gate.passed=true` until the exact build set passes the required gates.

Finally:

```sh
make resolve-config
make render
```

The resolved-config hash excludes only the resolved-config and compatibility-manifest digest carrier fields, avoiding their mutual circular digest while hashing every operational setting. The final resolved YAML is stored under `var/artifacts/compose.resolved.yaml`; it contains secret file paths, never secret values.

## Release and rollback gates

Before local rollout, independently inspect the resolved topology for exactly five long-running services, one API replica, only private-interface `8888/8080`, no socket/root/model-source mount, internal admin/controller/PostgreSQL listeners, one model/runtime, exact networks, read-only non-root containers, dropped capabilities, PID/CPU/memory limits, and immutable image identities.

Run lifecycle, inference, queue, privacy, backup/restore, resource-provenance, LAN, and browser evidence against one revision plus manifest digest. Evidence is not product approval.

Rollback selects the immediately preceding fully evidenced manifest as a unit. Never roll back by editing tags, retaining a new schema with an incompatible old binary, lowering settings, bypassing migration, selecting a second runtime/model, or restoring live data without separate authorization. When backward schema compatibility is uncertain, remain unavailable and escalate to the backend owner and user authorization gate.
