#!/bin/sh
set -eu
umask 077
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
cd "$ROOT"
. ./config/runtime.mac.conf
for key in GO_BUILDER_IMAGE DISTROLESS_STATIC_IMAGE NODE_BUILDER_IMAGE NGINX_RUNTIME_IMAGE POSTGRES_CLIENT_IMAGE POSTGRES_IMAGE MODEL_RUNNER_SOURCE MODEL_RUNNER_COMMIT DOCKER_MODEL_PLUGIN_SHA256; do
  eval "value=\${$key:-}"
  [ -n "$value" ] || { echo "unresolved build identity: $key" >&2; exit 1; }
done
[ "$MODEL_RUNNER_SOURCE" = https://github.com/docker/model-runner.git ] || { echo 'unexpected plugin source' >&2; exit 1; }
./ops/model/verify.sh
mkdir -p var/artifacts/images
common="--platform linux/arm64 --load"
docker buildx build $common --iidfile var/artifacts/images/api.iid \
  --build-context api-src=./services/api --build-context tokenizer-src=./var/artifacts/tokenizer \
  --build-arg GO_BUILDER_IMAGE="$GO_BUILDER_IMAGE" --build-arg DISTROLESS_STATIC_IMAGE="$DISTROLESS_STATIC_IMAGE" \
  -f ops/images/api.Dockerfile ops/images/context
docker buildx build $common --iidfile var/artifacts/images/web.iid \
  --build-context web-src=./web --build-context ops-src=./ops \
  --build-arg NODE_BUILDER_IMAGE="$NODE_BUILDER_IMAGE" --build-arg NGINX_RUNTIME_IMAGE="$NGINX_RUNTIME_IMAGE" \
  -f ops/images/web.Dockerfile ops/images/context
docker buildx build $common --iidfile var/artifacts/images/controller.iid \
  --build-context controller-src=./services/controller \
  --build-arg GO_BUILDER_IMAGE="$GO_BUILDER_IMAGE" --build-arg DISTROLESS_STATIC_IMAGE="$DISTROLESS_STATIC_IMAGE" \
  --build-arg MODEL_RUNNER_SOURCE="$MODEL_RUNNER_SOURCE" --build-arg MODEL_RUNNER_COMMIT="$MODEL_RUNNER_COMMIT" \
  --build-arg DOCKER_MODEL_PLUGIN_SHA256="$DOCKER_MODEL_PLUGIN_SHA256" -f ops/images/controller.Dockerfile ops/images/context
docker buildx build $common --iidfile var/artifacts/images/jobs.iid \
  --build-context ops-src=./ops --build-context migrations-src=./db/migrations \
  --build-arg GO_BUILDER_IMAGE="$GO_BUILDER_IMAGE" --build-arg POSTGRES_CLIENT_IMAGE="$POSTGRES_CLIENT_IMAGE" \
  -f ops/images/jobs.Dockerfile ops/images/context
python3 ops/images/record-images.py config/runtime.mac.conf config/compatibility-manifest.json var/artifacts/images
echo 'application image IDs recorded; no registry push performed'
