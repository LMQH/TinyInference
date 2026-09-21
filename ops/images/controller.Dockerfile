# syntax=docker/dockerfile:1.7
ARG GO_BUILDER_IMAGE
ARG DISTROLESS_STATIC_IMAGE
ARG MODEL_RUNNER_SOURCE
ARG MODEL_RUNNER_COMMIT

FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS controller-build
WORKDIR /src/services/controller
COPY --from=controller-src go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked go mod download
COPY --from=controller-src . ./
ARG TARGETOS
ARG TARGETARCH
RUN test "$TARGETOS" = linux && test "$TARGETARCH" = arm64 && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/controller ./cmd/controller

FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS plugin-compile
ARG MODEL_RUNNER_SOURCE
ARG MODEL_RUNNER_COMMIT
RUN test -n "$MODEL_RUNNER_SOURCE" && test -n "$MODEL_RUNNER_COMMIT"
ADD --keep-git-dir=true ${MODEL_RUNNER_SOURCE}#${MODEL_RUNNER_COMMIT} /src/model-runner
WORKDIR /src/model-runner/cmd/cli
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/docker-model .

FROM scratch AS plugin-export
COPY --from=plugin-compile /out/docker-model /docker-model

FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS plugin-verified
ARG DOCKER_MODEL_PLUGIN_SHA256
RUN test -n "$DOCKER_MODEL_PLUGIN_SHA256"
COPY --from=plugin-compile /out/docker-model /out/docker-model
RUN printf '%s  %s\n' "$DOCKER_MODEL_PLUGIN_SHA256" /out/docker-model | sha256sum -c -

FROM ${DISTROLESS_STATIC_IMAGE}
COPY --from=controller-build --chown=65532:65532 /out/controller /app/controller
COPY --from=plugin-verified --chown=65532:65532 /out/docker-model /usr/local/bin/docker-model
USER 65532:65532
ENV PATH=/usr/local/bin
EXPOSE 9090
ENTRYPOINT ["/app/controller"]
