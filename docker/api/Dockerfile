# syntax=docker/dockerfile:1.20@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d

FROM golang:1.27.1-alpine3.23@sha256:d9e2f2f07b10cc922da3e80e035c3058810b328d5aef82d2c63680967c5e2ec9 AS builder

ARG TARGETOS=linux
ARG TARGETARCH
ARG SOURCE_DATE_EPOCH=0
WORKDIR /workspace/Judgify/api

# Keep the module graph cacheable. go-common is a named BuildKit context so
# the source tree keeps its existing sibling-module development layout.
COPY api/go.mod api/go.sum ./
COPY api/third_party ./third_party
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
      -o /out/judgify-capability-inspector \
      ./cmd/judgify-capability-inspector

FROM docker:29.4.0-cli-alpine3.23@sha256:2efe7c8e806b35050def6ba1ba0ddad67b521a0a6d37f4910b3d88f11b495d03 AS docker_cli

FROM alpine:3.23.5@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS runtime

RUN apk add --no-cache \
      ca-certificates=20260611-r0 \
      tzdata=2026c-r0 \
    && addgroup -S -g 65532 judgify \
    && adduser -S -D -H -u 65532 -G judgify judgify \
    && mkdir -p /app/config \
    && chown -R 65532:65532 /app

COPY --from=builder --chown=65532:65532 /out/judgify /usr/local/bin/judgify

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
ENTRYPOINT ["/usr/local/bin/judgify"]

FROM runtime AS api
EXPOSE 8000
CMD ["api"]

FROM runtime AS migrate
CMD ["migrate"]

FROM runtime AS worker
COPY --from=docker_cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=builder --chown=65532:65532 /out/judgify-capability-inspector /usr/local/bin/judgify-capability-inspector
CMD ["judge-worker"]
