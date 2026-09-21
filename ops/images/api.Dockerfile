# syntax=docker/dockerfile:1.7
ARG GO_BUILDER_IMAGE
ARG DISTROLESS_STATIC_IMAGE
FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS build
WORKDIR /src/services/api
COPY --from=api-src go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked go mod download
COPY --from=api-src . ./
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/api ./cmd/api

FROM ${DISTROLESS_STATIC_IMAGE}
COPY --from=build --chown=65532:65532 /out/api /app/api
COPY --from=tokenizer-src --chown=65532:65532 tokenizer.gguf /app/tokenizer.gguf
USER 65532:65532
EXPOSE 8888 8889
ENTRYPOINT ["/app/api"]
