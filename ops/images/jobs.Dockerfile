# syntax=docker/dockerfile:1.7
ARG GO_BUILDER_IMAGE
ARG POSTGRES_CLIENT_IMAGE
FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS proof-build
WORKDIR /src
COPY --from=ops-src lifecycle-proof/main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/lifecycle-proof main.go

FROM ${POSTGRES_CLIENT_IMAGE}
COPY --from=proof-build --chown=70:70 /out/lifecycle-proof /usr/local/bin/lifecycle-proof
COPY --from=migrations-src --chown=70:70 . /opt/mini-inference/migrations
COPY --from=ops-src --chown=70:70 jobs /opt/mini-inference/jobs
COPY --from=ops-src --chown=70:70 schedule /opt/mini-inference/schedule
USER 70:70
ENTRYPOINT ["/bin/sh"]
