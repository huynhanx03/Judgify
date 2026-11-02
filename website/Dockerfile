# syntax=docker/dockerfile:1.20@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d

FROM node:22.22.0-alpine3.23@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34 AS dependencies
ENV NEXT_TELEMETRY_DISABLED=1
WORKDIR /app
COPY website/package.json website/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

FROM dependencies AS builder
ARG NEXT_PUBLIC_API_URL=/api
ARG NEXT_PUBLIC_REALTIME_WS_URL=
ARG SOURCE_DATE_EPOCH=0
ENV NEXT_PUBLIC_API_URL="${NEXT_PUBLIC_API_URL}" \
    NEXT_PUBLIC_REALTIME_WS_URL="${NEXT_PUBLIC_REALTIME_WS_URL}" \
    SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}"
COPY website/ ./
RUN npm run build

FROM node:22.22.0-alpine3.23@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34 AS runtime

ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=3000

RUN addgroup -S -g 10001 nodejs \
    && adduser -S -D -H -u 10001 -G nodejs nextjs

WORKDIR /app
COPY --from=builder --chown=10001:10001 /app/public ./public
COPY --from=builder --chown=10001:10001 /app/.next/standalone ./
COPY --from=builder --chown=10001:10001 /app/.next/static ./.next/static

ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=1970-01-01T00:00:00Z
LABEL org.opencontainers.image.title="Judgify Website" \
      org.opencontainers.image.description="Judgify Next.js standalone runtime" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/huynhanx03/Judgify"

USER 10001:10001
EXPOSE 3000
CMD ["node", "server.js"]
