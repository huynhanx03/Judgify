# Envoy gateway, authorization, and observability for Judgify

Date: 2026-09-13

## Goal and scope

Replace the production Nginx ingress with one standalone Envoy process in the existing Docker Compose deployment. Connect the already provisioned Grafana, Loki, Tempo, Prometheus, Alertmanager, and Alloy stack to real Judgify traffic. Move HTTP route-level authorization from scattered Gin `RequirePermission` calls to Envoy's `ext_authz` Check protocol, backed by Judgify's existing credential verifier and Casbin authorization service.

The public request path remains `browser -> Envoy -> API or website`. Envoy is the only Internet-facing gateway. This design does not introduce Kubernetes, the Envoy Gateway control-plane product, a new BFF, or a second public proxy. It does not move resource-specific authorization, CSRF protection, or WebSocket topic authorization out of the API.

This is one coordinated architecture change delivered in three independently verifiable waves: gateway parity; observability wiring; then route-level authorization migration. Existing Nginx and Gin permission checks remain available for rollback until the replacements have passed the stated acceptance checks.

## Current state and constraints

- `deploy/compose.production.yml` publishes HTTPS through Nginx. The API listens on port 8000 on both `edge` and `backend`; the website is on `edge`. Operational listeners are private.
- `deploy/reverse-proxy.conf` routes `/api/*` to API with the prefix removed, `/ws` to API with HTTP/1.1 upgrade, and all other paths to Next.js. It returns 404 for public `/api/live`, `/api/ready`, `/api/ping`, and `/api/metrics`.
- `ops/stack/docker-compose.yml` declares all six observability components. Alloy has no log or OTLP receiver; Prometheus scrapes only itself. The dashboard and alert rules therefore do not yet prove application monitoring.
- Go correlation uses `X-Correlation-ID`; the value is persisted in audit, operation, outbox, and judge records. Removing the field would require a broad schema and API migration.
- Gin route middleware calls the transport-neutral authorization service. The service uses the current Casbin policy snapshot. Some permissions are checked in use cases and WebSocket topic handlers after the HTTP route has been admitted.
- The existing working tree contains extensive unrelated changes. This design must not rewrite or discard them.

## Options considered

1. **Recommended: one Envoy gateway, a private Check endpoint in the existing Go API process, and the existing Grafana stack.** This reuses the policy source and deployable units. Each protected request adds one internal Check call. Resource-specific checks remain in the API.
2. **Separate authorization service deployment.** It isolates Check capacity and failure domains, but duplicates deployment, secret distribution, health checks, and policy synchronization for a single-host product. Reconsider only if measured Check load warrants it.
3. **Keep all authorization in Gin while replacing ingress.** It minimizes authorization migration risk, but does not meet the requested central route-level decision at Envoy.

## Gateway topology and behavior

Envoy replaces the `ingress` service without adding another request hop. It binds the existing HTTPS public port; the admin/stats listener is private and is never published. Its configuration is checked against the exact public hostname and TLS certificate. The gateway generates or overwrites untrusted incoming `X-Request-ID`, normalizes forwarding headers at the trusted edge, and sends only allowed headers upstream.

Route precedence is explicit: operational-path denials before the `/api/` prefix route; `/api` redirect; `/api/*` prefix rewrite; `/ws` WebSocket upgrade; static Next.js assets; then website fallback. Preserve the current 4 MiB body limit, security headers, HTTPS behavior, health semantics, and API/website timeouts. Use route-specific WebSocket idle behavior. Retries are disabled for mutations; any retry policy for idempotent reads requires separate testing. Envoy's administration endpoint is available only to internal health and metric scraping.

The API, website, worker, database, authorization listener, and telemetry endpoints remain unpublished. The API's externally routed listener is reachable only through the gateway's dedicated network path. Server-side website calls to protected API routes also traverse Envoy; internal jobs use separate service authority instead of impersonating a browser. Network isolation is accompanied by application verification of gateway admission; a Docker network alone is not an authorization credential.

## Request identity and trace contract

Envoy owns `X-Request-ID` at the Internet boundary. For an HTTP request admitted to API, Judgify uses that exact value as its existing correlation ID and returns it as `X-Correlation-ID`. Thus a gateway `request_id` and the persisted `correlation_id` are aliases for the same value, rather than independent identifiers. Autonomous jobs without an originating HTTP request continue to mint a UUIDv7 CID. The API validates accepted IDs and ignores client-provided `X-Correlation-ID` when a trusted gateway request ID is present.

OpenTelemetry owns W3C `traceparent`, `trace_id`, and `span_id`. The trace begins at Envoy and includes `ext_authz` and API spans. The API stores the origin trace context in the outbox event metadata, and the worker extracts it before creating its span. A long-delayed or retried job may start a new trace linked to the originating context while retaining the durable CID. `user_id` is attached only after credential verification. No organization identifier is invented.

The gateway logs the request ID, trace ID when available, route template or route name, method, status, duration, upstream, response flags, and bytes. API and worker structured logs retain CID, trace ID, active span ID when useful, event/outcome, and relevant nonsecret domain IDs. These high-cardinality IDs remain JSON fields or structured metadata, not Loki stream labels. Credentials, cookies, request bodies, source code, hidden tests, raw query strings, and complete WebSocket messages are excluded from ordinary logs and spans.

### Structured log contract

All product logs use one JSON object per line on stdout. The common fields are `timestamp` (UTC with timezone), `level`, `service`, `environment`, `version`, `component`, `log_class` (`access`, `application`, `security`, or `infrastructure`), `event` (stable machine-readable name), and `message` (short human-readable text). Request- or job-related records add `cid`; this is the existing Go logger field. Envoy writes the value of its `X-Request-ID` under `cid`, so a single Loki query finds gateway, API, Check, and worker records. `trace_id` is present when a valid trace context exists; `span_id` is present only when that record belongs to an active span. Neither is fabricated for process startup or periodic background events. The existing `correlation_id` database columns and response header remain unchanged; logs do not emit a second `request_id` or `correlation_id` field containing the same value.

The source-specific fields are intentionally small:

| Source and event | Required extra fields | Conditional fields |
| --- | --- | --- |
| Envoy `http.completed` | `method`, `route`, `status`, `duration_ms`, `upstream_cluster`, `response_flags`, `response_code_details`, `bytes_in`, `bytes_out` | `trace_id` |
| Go Check `authz.denied` or `authz.error` | `decision`, `reason_code`, `route`, `method`, `duration_ms`, `policy_revision` | verified `user_id`, `resource`, `action`, `trace_id`, `span_id` |
| API business transition, e.g. `submission.accepted` | `user_id` after authentication, `submission_id`, `problem_id`, `cid` | `contest_id`, `operation_id`, `error_code`, `trace_id`, `span_id` when relevant |
| Worker `judge.started`, `judge.completed`, or `judge.failed` | `job_id`, `submission_id`, `attempt_id`, `cid` | `verdict`, `phase`, `duration_ms`, `queue_wait_ms`, `retryable`, `error_code`, `trace_id`, `span_id` |
| WebSocket connect/disconnect or denied subscription | `connection_id`, `event`, `cid` | verified `user_id`, `close_code`, `duration_ms`, `topic_kind`, `reason_code` |

Envoy emits one completion record per dynamic HTTP request, including gateway denials and upstream failures. Health probes, metrics endpoints, and static asset success traffic are filtered from normal access logs; static failures remain visible. A restricted security event may include `client_ip` for 401, 403, 429, and gateway blocks; its retention is seven days. The Go API does not repeat a successful per-request completion log already supplied by Envoy. Replace the existing `RequestLogger` completion behavior that logs every request with `ip` and `query`, while preserving its CID/context injection. The API emits one structured log for meaningful business transitions and one root-cause error log for unexpected failures; slow API requests and direct/internal request failures may emit an API completion record. Check Allow decisions are counted in metrics and traced, but not individually logged at INFO. Deny/error decisions are logged with bounded `reason_code` values. WebSocket heartbeats and individual payload messages are not logged.

`INFO` describes completed business transitions, `WARN` describes rejected or retryable conditions needing attention, and `ERROR` describes failed dependencies or unexpected processing. A routine invalid login or lack of permission is not labeled as a server error. Metrics record full counts even where noisy log events are rate-limited. The production logger's generic sampling must not silently discard error, security, or audit-relevant events. Stack traces are attached only to unexpected errors; expected validation and authorization outcomes use stable error codes. Request and response bodies, raw URLs or query strings, bearer tokens, cookies, session IDs, source code, test data, compiler output, and arbitrary user input are excluded. Verified `user_id` is the only ambient identity field; other domain IDs appear only on events that need them.

Loki index labels are initially `service`, `environment`, and low-cardinality `log_class`; the security class supports its separate seven-day retention policy. `level`, `version`, `component`, route, CID, trace/span IDs, user ID, submission ID, and job ID remain JSON or structured metadata. This avoids a stream per user or request. Operators can filter one CID across services and follow `trace_id` to Tempo, but a sampled-out trace may have logs without a retained Tempo trace.

## Authorization boundary

Envoy's HTTP `ext_authz` filter calls a private gRPC `Check` endpoint in the existing Go API process. The Check handler reuses the current access-token/session verifier and Casbin authorization service; it does not duplicate policy rules in Envoy. It receives only allowlisted method, normalized path, authority, credential header or cookie, request ID, and trace context. It does not buffer or inspect request bodies.

A single Judgify-owned route policy manifest maps every method and normalized protected route to its required resource/action set, including routes with alternative capabilities. Public, optional-identity, authentication, OAuth callback, refresh, WebSocket, and operational routes are classified explicitly. Unknown or newly added protected routes cannot silently become public: route coverage tests compare the live router catalog to the manifest and fail the build on mismatch. Route-specific filter configuration and subsequent header/route mutations must not permit an authorization bypass.

The Check result distinguishes invalid/missing credentials (401), authenticated but unauthorized (403), and evaluation or dependency failure (503). Envoy is configured to fail closed on timeout or Check service error and to return 503 for infrastructure errors, not a misleading 403. The Check timeout and concurrency budget are measured under load; no cache of user permission decisions is assumed because revocation and policy revision must take effect predictably.

On Allow, the Check service returns a short-lived, signed admission assertion bound to the request ID, normalized method/path, principal/session, and expiration. Envoy strips any client-supplied assertion before Check and forwards only the Check-issued assertion. The API verifies its signature, purpose, expiry, and request binding before constructing principal context. Signing keys are separate from browser credential keys. This replaces the repeated route-level Casbin call with a narrow verification boundary; it is not permission derived from an untrusted `X-User-ID` header.

The API retains checks that require current domain data, such as ownership, contest registration, publication state, transition preconditions, and recent reauthentication. CSRF remains enforced on browser mutations. The `/ws` handshake retains its ticket/origin/authentication checks; each subscription still calls the API's topic authorizer. Internal worker and operational paths do not trust an Envoy header as user authority.

During migration, the Check service can return Allow in comparison mode while recording its candidate decision, and the existing Gin middleware remains authoritative. Only after route-by-route parity tests and direct-API bypass tests pass do protected routes switch to enforced `ext_authz` and remove their `RequirePermission` adapters. Rollback restores the prior ingress and Gin permission enforcement using the same policy store; no durable authorization schema migration is required.

## Observability pipeline and operations

Keep the current small, monolithic self-hosted components. Alloy receives OTLP traces from Envoy, API, and worker and forwards them to Tempo. For production Docker Compose, the application containers use Docker's RFC5424 syslog driver to send structured stdout logs over host loopback TCP to Alloy's `loki.source.syslog`; Alloy forwards them to Loki. The log driver uses a bounded nonblocking buffer so an Alloy outage does not stall the product; a collector outage can lose ordinary logs, while durable audit records remain in PostgreSQL. This avoids giving Alloy a Docker daemon socket. Prometheus scrapes private API, worker, Envoy, Alloy, Loki, Tempo, and its own metric endpoints; it sends alert events to Alertmanager. Grafana uses its provisioned Loki, Tempo, and Prometheus data sources, with bidirectional links between a log's trace ID and the corresponding Tempo trace.

Production telemetry endpoints are private. Grafana requires a nondefault secret and restricted operator access; Prometheus and Alloy UI ports are not publicly published. Set bounded retention and disk budgets: initial targets are 7 days for metrics, 7 days for traces, and 14 days for ordinary logs, subject to measured disk volume. Audit records remain durable in the application database rather than depending on log retention. The first deployment can use local storage on one host; this has no host-level high availability and is documented as such.

Alerts must cover absent API/worker/Envoy scrape targets, gateway 5xx and upstream failures, Check error/timeout rate and latency, API p95/5xx, queue age, worker capacity, and telemetry ingest failure. Each alert has a verified runbook and one tested notification route. A separate off-host availability check is a later reliability enhancement, not a prerequisite for connecting this stack.

## Verification and rollout

1. Validate Envoy configuration statically and exercise production-route parity in a local Compose stack: exact hostname/TLS, `/api` redirect and rewrite, hidden operational paths, static content, upload limit, WebSocket connect/reconnect, forwarding header sanitization, gateway health, and rollback to Nginx.
2. Send one real authenticated request and one submission through Envoy. In Grafana, find the same request ID in gateway logs and CID in API/worker logs, then navigate to the Tempo trace with distinct Envoy, Check, API, and worker spans or an explicitly linked worker trace. Demonstrate that metrics and alerts contain real application data, not only self-scrapes.
3. Test the route policy manifest against the live API router. Exercise anonymous, user, and admin access, policy revocation, 401/403/503 mapping, Check timeout, malformed assertions, forged forwarding headers, direct API access, OAuth/refresh, CSRF, and WebSocket topic denial. Compare Check and legacy middleware decisions before the enforcement cutover.
4. Measure p50/p95/p99 gateway and Check latency plus CPU/memory at representative and burst load. Do not claim an acceptable overhead without those measurements. Verify that a telemetry outage does not change request authorization or product availability, while a Check outage fails closed on protected routes.
5. Release gateway/telemetry first, then authorize route groups incrementally. Keep a documented one-command rollback path and re-run the end-to-end authorization matrix after every route group switch.

## Completion criteria

The old public Nginx service is absent; Envoy is the only published ingress. FE, API, WebSocket, and security headers pass route-parity tests. A request can be followed through logs and traces in Grafana using the shared request/CID value and trace ID. Prometheus sees application and gateway targets, and a tested alert reaches its configured destination. Every protected HTTP route has an explicit Check policy and returns correct 401/403/503 outcomes; no direct-API or WebSocket topic path bypasses authorization. The system has measured gateway/Check overhead, bounded telemetry storage, and a tested rollback.

## Technical references

- [Envoy external authorization](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter)
- [Envoy HTTP ext_authz API and per-route configuration](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/ext_authz/v3/ext_authz.proto)
- [Grafana Alloy syslog source](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.syslog/)
- [Docker syslog logging driver](https://docs.docker.com/engine/logging/drivers/syslog/)
- [Grafana Tempo deployment modes](https://grafana.com/docs/tempo/latest/reference-tempo-architecture/deployment-modes/)
