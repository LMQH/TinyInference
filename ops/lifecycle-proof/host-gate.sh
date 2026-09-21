#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
cd "$ROOT"
COMPOSE='docker compose --project-name mini-inference --env-file config/runtime.mac.conf'
$COMPOSE --profile ops run --rm lifecycle-proof
if $COMPOSE --profile ops run --rm --no-deps --entrypoint /app/controller -e MODEL_RUNNER_HOST= controller; then
  echo 'controller accepted missing MODEL_RUNNER_HOST' >&2
  exit 1
fi
if $COMPOSE --profile ops run --rm --no-deps --entrypoint /app/controller -e MODEL_RUNNER_HOST=http://127.0.0.1:12435 controller; then
  echo 'controller accepted localhost MODEL_RUNNER_HOST' >&2
  exit 1
fi
if $COMPOSE --profile ops run --rm --no-deps --entrypoint /app/controller -e MODEL_RUNNER_HOST=http://example.invalid:12435 controller; then
  echo 'controller accepted arbitrary MODEL_RUNNER_HOST' >&2
  exit 1
fi
echo 'lifecycle gate completed; inspect content-safe evidence for fixed operations and fail-closed startup'
