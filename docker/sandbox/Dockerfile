# syntax=docker/dockerfile:1.20@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d

FROM golang:1.27.1-alpine3.23@sha256:d9e2f2f07b10cc922da3e80e035c3058810b328d5aef82d2c63680967c5e2ec9 AS inspector_builder

ARG TARGETOS=linux
ARG TARGETARCH
ARG SOURCE_DATE_EPOCH=0
WORKDIR /workspace/Judgify/api
COPY docker/sandbox/inspector-go.mod ./go.mod
COPY api/cmd/judgify-capability-inspector ./cmd/judgify-capability-inspector
COPY api/cmd/judgify-sandbox-init ./cmd/judgify-sandbox-init
COPY api/internal/judge/checker ./internal/judge/checker
COPY api/internal/judge/capability/inspector ./internal/judge/capability/inspector
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/judgify-capability-inspector \
      ./cmd/judgify-capability-inspector \
    && CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    go build \
      -buildvcs=false \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/judgify-sandbox-init \
      ./cmd/judgify-sandbox-init

FROM alpine:3.23.5@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

# Install compilers and runtimes: C++, Python, Java, Go
RUN apk add --no-cache \
    g++=15.2.0-r2 \
    python3=3.12.14-r0 \
    openjdk17-jdk=17.0.20_p8-r0 \
    go=1.25.10-r0 \
    time=1.9-r1

# Defense in depth: Docker also forces this UID/GID and mounts a private 0700
# tmpfs at runtime. Keeping the image non-root prevents an unsafe ad-hoc run
# from silently granting contestant code root privileges.
RUN addgroup -S -g 65532 sandbox \
    && adduser -S -D -H -u 65532 -G sandbox sandbox \
    && mkdir -p /sandbox \
    && chown 65532:65532 /sandbox \
    && chmod 0700 /sandbox

ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z
ARG CPP17_PROFILE_VERSION
ARG CPP17_COMPILER_VERSION
ARG CPP17_RUNTIME_BOOTSTRAP_VERSION
ARG CPP17_COMPILE_COMMAND_ID
ARG CPP17_RUN_COMMAND_ID
ARG CPP17_MANIFEST_VERSION
ARG CPP17_MANIFEST_CHECKSUM
ARG PYTHON3_PROFILE_VERSION
ARG PYTHON3_COMPILER_VERSION
ARG PYTHON3_RUNTIME_BOOTSTRAP_VERSION
ARG PYTHON3_COMPILE_COMMAND_ID
ARG PYTHON3_RUN_COMMAND_ID
ARG PYTHON3_MANIFEST_VERSION
ARG PYTHON3_MANIFEST_CHECKSUM
ARG JAVA17_PROFILE_VERSION
ARG JAVA17_COMPILER_VERSION
ARG JAVA17_RUNTIME_BOOTSTRAP_VERSION
ARG JAVA17_COMPILE_COMMAND_ID
ARG JAVA17_RUN_COMMAND_ID
ARG JAVA17_MANIFEST_VERSION
ARG JAVA17_MANIFEST_CHECKSUM
ARG GO_PROFILE_VERSION
ARG GO_COMPILER_VERSION
ARG GO_RUNTIME_BOOTSTRAP_VERSION
ARG GO_COMPILE_COMMAND_ID
ARG GO_RUN_COMMAND_ID
ARG GO_MANIFEST_VERSION
ARG GO_MANIFEST_CHECKSUM
LABEL org.opencontainers.image.title="Judgify Sandbox" \
      org.opencontainers.image.description="Network-isolated contestant code runtime" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/huynhanx03/Judgify" \
      io.judgify.runtime.cpp17.profile-version="${CPP17_PROFILE_VERSION}" \
      io.judgify.runtime.cpp17.compiler-version="${CPP17_COMPILER_VERSION}" \
      io.judgify.runtime.cpp17.runtime-bootstrap-version="${CPP17_RUNTIME_BOOTSTRAP_VERSION}" \
      io.judgify.runtime.cpp17.compile-command-id="${CPP17_COMPILE_COMMAND_ID}" \
      io.judgify.runtime.cpp17.run-command-id="${CPP17_RUN_COMMAND_ID}" \
      io.judgify.runtime.cpp17.manifest-version="${CPP17_MANIFEST_VERSION}" \
      io.judgify.runtime.cpp17.manifest-checksum="${CPP17_MANIFEST_CHECKSUM}" \
      io.judgify.runtime.python3.profile-version="${PYTHON3_PROFILE_VERSION}" \
      io.judgify.runtime.python3.compiler-version="${PYTHON3_COMPILER_VERSION}" \
      io.judgify.runtime.python3.runtime-bootstrap-version="${PYTHON3_RUNTIME_BOOTSTRAP_VERSION}" \
      io.judgify.runtime.python3.compile-command-id="${PYTHON3_COMPILE_COMMAND_ID}" \
      io.judgify.runtime.python3.run-command-id="${PYTHON3_RUN_COMMAND_ID}" \
      io.judgify.runtime.python3.manifest-version="${PYTHON3_MANIFEST_VERSION}" \
      io.judgify.runtime.python3.manifest-checksum="${PYTHON3_MANIFEST_CHECKSUM}" \
      io.judgify.runtime.java17.profile-version="${JAVA17_PROFILE_VERSION}" \
      io.judgify.runtime.java17.compiler-version="${JAVA17_COMPILER_VERSION}" \
      io.judgify.runtime.java17.runtime-bootstrap-version="${JAVA17_RUNTIME_BOOTSTRAP_VERSION}" \
      io.judgify.runtime.java17.compile-command-id="${JAVA17_COMPILE_COMMAND_ID}" \
      io.judgify.runtime.java17.run-command-id="${JAVA17_RUN_COMMAND_ID}" \
      io.judgify.runtime.java17.manifest-version="${JAVA17_MANIFEST_VERSION}" \
      io.judgify.runtime.java17.manifest-checksum="${JAVA17_MANIFEST_CHECKSUM}" \
      io.judgify.runtime.go.profile-version="${GO_PROFILE_VERSION}" \
      io.judgify.runtime.go.compiler-version="${GO_COMPILER_VERSION}" \
      io.judgify.runtime.go.runtime-bootstrap-version="${GO_RUNTIME_BOOTSTRAP_VERSION}" \
      io.judgify.runtime.go.compile-command-id="${GO_COMPILE_COMMAND_ID}" \
      io.judgify.runtime.go.run-command-id="${GO_RUN_COMMAND_ID}" \
      io.judgify.runtime.go.manifest-version="${GO_MANIFEST_VERSION}" \
      io.judgify.runtime.go.manifest-checksum="${GO_MANIFEST_CHECKSUM}"

COPY --from=inspector_builder /out/judgify-capability-inspector /usr/local/bin/judgify-capability-inspector
COPY --from=inspector_builder /out/judgify-sandbox-init /usr/local/bin/judgify-sandbox-init

WORKDIR /sandbox
USER 65532:65532

# Ad-hoc starts fail closed. The supervisor always supplies a reviewed direct
# argv behind the fixed bootstrap handshake.
ENTRYPOINT ["/usr/local/bin/judgify-sandbox-init"]
