# syntax=docker/dockerfile:1.20@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d

FROM golang:1.27.3-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

ARG TARGETOS=linux
ARG TARGETARCH
ARG SOURCE_DATE_EPOCH=0
WORKDIR /workspace/Judgify/api

# Keep the module graph cacheable. go-common is a named BuildKit context so
# the source tree keeps its existing sibling-module development layout.
COPY api/go.mod api/go.sum ./
COPY --from=go_common / /workspace/go-common/
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download \
    && go mod verify

COPY api/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/judgify \
      ./cmd/judgify \
    && CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/server \
      ./cmd/server \
    && CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/migrate \
      ./cmd/migrate \
    && CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/judge-worker \
      ./cmd/judge-worker \
    && CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/judgify-capability-inspector \
      ./cmd/judgify-capability-inspector

FROM docker:29.4.0-cli-alpine3.23@sha256:2efe7c8e806b35050def6ba1ba0ddad67b521a0a6d37f4910b3d88f11b495d03 AS docker_cli

FROM alpine:3.23.5@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS runtime

RUN apk add --no-cache \
      ca-certificates=20261020-r0 \
      tzdata=2026e-r0 \
    && addgroup -S -g 65532 judgify \
    && adduser -S -D -H -u 65532 -G judgify judgify \
    && mkdir -p /app/config \
    && chown -R 65532:65532 /app

ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z
ARG GO_COMMON_VCS_REF=unknown
LABEL org.opencontainers.image.title="Judgify" \
      org.opencontainers.image.description="Judgify role-pure runtime" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/huynhanx03/Judgify" \
      io.judgify.go-common.revision="${GO_COMMON_VCS_REF}"

WORKDIR /app
USER 65532:65532
FROM runtime AS api
COPY --from=builder --chown=65532:65532 /out/server /usr/local/bin/server
ENTRYPOINT ["/usr/local/bin/server"]
EXPOSE 8000

FROM runtime AS migrate
COPY --from=builder --chown=65532:65532 /out/migrate /usr/local/bin/migrate
ENTRYPOINT ["/usr/local/bin/migrate"]

FROM runtime AS worker
COPY --from=builder --chown=65532:65532 /out/judge-worker /usr/local/bin/judge-worker
COPY --from=docker_cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=builder --chown=65532:65532 /out/judgify-capability-inspector /usr/local/bin/judgify-capability-inspector
ENTRYPOINT ["/usr/local/bin/judge-worker"]

FROM runtime AS local
COPY --from=builder --chown=65532:65532 /out/judgify /usr/local/bin/judgify
COPY --from=builder --chown=65532:65532 /out/server /usr/local/bin/server
COPY --from=builder --chown=65532:65532 /out/migrate /usr/local/bin/migrate
COPY --from=builder --chown=65532:65532 /out/judgify-capability-inspector /usr/local/bin/judgify-capability-inspector
