# Judgify Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Connect Judgify's existing Grafana/Loki/Tempo/Prometheus/Alertmanager/Alloy stack to real gateway, API, website, and worker traffic with a lean, traceable log contract.

**Architecture:** Docker sends RFC5424 structured stdout to an Alloy syslog listener bound on host loopback; Alloy assigns trusted low-cardinality labels and forwards JSON lines to Loki. Envoy, API, and worker export traces to Alloy/Tempo; Prometheus scrapes private metrics endpoints. App logs retain only contextual IDs needed to join operations, while Envoy owns successful HTTP completion logs.

**Tech Stack:** Grafana Alloy v1.10.2, Loki v3.5.3, Tempo v2.8.2, Prometheus v3.5.0, Alertmanager v0.28.1, Go 1.27.1, OpenTelemetry, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-13-envoy-gateway-observability-authz-design.md`

## Global Constraints

- Follow gateway plan `docs/superpowers/plans/2026-09-13-envoy-gateway-identity.md`; Envoy owns public IDs and the HTTP completion log.
- JSON lines contain `timestamp`, `level`, `message`; retain `version` per production line. `service` and `environment` are Loki stream labels only, never JSON fields.
- `request_id`, `correlation_id`, `trace_id`, `span_id`, `user_id`, and domain IDs must not be index labels; `cid` is removed as a log alias, not as a database column.
- `duration_ms` appears only on a completed named operation; `submission.accepted` has no synthetic duration.
- Ordinary logs: 14 days; traces and metrics: 7 days. Security-log 7-day retention is enabled only after a working selector and access boundary are tested.
- No public Prometheus, Alloy, Envoy admin, Tempo, Loki, or API/worker metrics listener; telemetry outages must not stall or change product authorization.

## File map and boundaries

- `ops/stack/alloy/config.alloy` receives RFC5424 logs and OTLP traces, applies source labels, and forwards to Loki/Tempo.
- `ops/stack/docker-compose.yml` binds the syslog listener to host loopback, keeps UI/control ports private, and gives collectors private network access.
- `deploy/compose.production.yml` configures per-service Docker logging and a private telemetry network; no Docker daemon socket enters Alloy.
- `ops/stack/loki/config.yml`, `ops/stack/tempo/config.yml`, and `ops/stack/prometheus/prometheus.yml` own retention and scrape budgets. `api/internal/platform/processhealth/metrics_server.go` serves metrics on dedicated private addresses while existing loopback health probes stay unchanged.
- `api/internal/infrastructure/logging/logger.go` and `../go-common/pkg/logger/zap.go` own Go field names/sampling; `api/internal/infrastructure/router.go` controls duplicate request completion logs.
- `api/internal/platform/eventing/outbox.go` and `api/internal/judge/log_context.go` carry durable correlation and trace links.
- `ops/provisioning/prometheus-rules/judgify.yml`, `ops/provisioning/grafana-dashboards/judgify-service-overview.json`, and `ops/runbooks/*.md` own dashboards, alerts, and operator proof.

---

### Task 1: Real log ingestion and trustworthy Loki labels

**Files:** Modify `ops/stack/alloy/config.alloy`, `ops/stack/docker-compose.yml`, `deploy/compose.production.yml`, `scripts/verify/topology.sh`; create `scripts/verify/observability_logs_test.sh`.

**Interfaces:** Docker logging driver sends RFC5424 over `tcp://127.0.0.1:1514` with explicit `tag` values `judgify-envoy`, `judgify-api`, `judgify-website`, `judgify-worker`. Alloy maps only allowlisted RFC5424 APP-NAME values (verified from Docker output because Docker appends the configured tag) to `service`, adds `environment="production"` from trusted collector configuration, and rejects or quarantines unknown tags. The JSON message remains unchanged.

- [ ] **Step 1 — red ingestion test:** Start the isolated ops stack, send a synthetic RFC5424 message with `APP-NAME=judgify-api`, and query Loki for `{service="judgify-api",environment="production"} |= "collector.contract"`. Assert the JSON line has no `service` or `environment` keys and the label set contains no ID. Confirm the test fails before Alloy is wired.
- [ ] **Step 2 — pipeline:** Configure Docker `logging.driver: syslog`, `syslog-format: rfc5424`, a stable per-service `tag`, and bounded nonblocking delivery. Add `loki.source.syslog` -> `loki.relabel` allowlist -> `loki.write`; do not promote all `__syslog_*` fields or JSON IDs to labels. Bind the receiving port only on host loopback. The app logger never copies the resulting labels back into JSON:

  ```yaml
  logging:
    driver: syslog
    options:
      syslog-address: tcp://127.0.0.1:1514
      syslog-format: rfc5424
      tag: judgify-api
      mode: non-blocking
      max-buffer-size: 4m
  ```
- [ ] **Step 3 — source proof:** Send one line from each running app container and assert the four service labels are distinct. Run `docker compose -f ops/stack/docker-compose.yml config`, `bash scripts/verify/topology.sh`, and `bash scripts/verify/observability_logs_test.sh`.
- [ ] **Step 4 — outage proof:** Stop Alloy, issue an API request, and assert the request still completes and no authorization decision changes; restart Alloy and confirm new logs resume. Record that best-effort ordinary logs may be lost while PostgreSQL audit records remain durable.
- [ ] **Step 5 — commit:** Commit the collector, Compose, topology, and test changes after the ingestion and outage checks pass.

### Task 2: Lean Go/application log contract

**Files:** Modify `../go-common/pkg/logger/zap.go`, `../go-common/pkg/common/http/middlewares/logger.go`, their tests, `api/internal/infrastructure/logging/logger.go`, `api/internal/infrastructure/router.go`, `api/internal/submission/adapters/driver/http/submission.go`, `api/internal/judge/log_context.go`, `api/internal/realtime/adapters/driver/http/routes.go`; add focused API log tests beside each changed package.

**Interfaces:** Logger context emits `request_id` for HTTP, `correlation_id` for causal work, and `trace_id`/`span_id` only when valid span context exists. It never emits `cid` alias or placeholder IDs. `message` is a stable namespaced event for business/security transitions. Static `service`/`environment` fields are removed from Go JSON only after Task 1 proves Alloy labels. Production `version` remains.

- [ ] **Step 1 — red schema tests:** Capture one API success, one denied action, one unexpected failure, and one worker completion as JSON. Assert `service`, `environment`, `cid`, `event`, raw query, and raw IP are absent from ordinary lines; assert IDs and `version` are correct. A worker log without HTTP context must omit `request_id`. Run the focused Go packages and confirm failure.
- [ ] **Step 2 — logger changes:** In `go-common`, remove unconditional `service`/`environment` JSON fields for this product configuration, preserve `version`, and exempt ERROR/security/audit-relevant events from generic sampling. Replace successful per-request API logging with context injection; Envoy emits `http.completed`. Keep slow/internal failure completion logs only where Envoy cannot supply equivalent detail. The schema test should make the omission and conditional identity explicit:

  ```go
  require.NotContains(t, fields, "service")
  require.NotContains(t, fields, "environment")
  require.NotContains(t, fields, "cid")
  require.Equal(t, originCID, fields["correlation_id"])
  require.NotContains(t, workerFields, "request_id")
  ```
- [ ] **Step 3 — business events:** Emit `submission.accepted` with verified `user_id`, `submission_id`, `problem_id`, and `idempotent_replay`; on replay include `origin_correlation_id`. Emit `judge.completed` with numeric `duration_ms` and separate `queue_wait_ms`. Log WebSocket connect/disconnect and denied subscriptions, but not heartbeats or payload messages. Do not log bodies, source code, test data, cookies, credentials, or unbounded paths.
- [ ] **Step 4 — dependency proof:** Run `cd ../go-common && go test ./pkg/logger ./pkg/common/http/middlewares`, then `bash scripts/verify/go.sh`. Commit `go-common` separately and, during transition only, update `GO_COMMON_REF` to its 40-character commit. Publish an immutable module version from that commit; update `api/go.mod`/`api/go.sum` to that version and remove the sibling `replace` and redundant CI sibling checkout as specified by Gateway Task 4. Run `make verify-release` with required evidence environment and approved image inventory from a clean checkout. Record module version and commit SHA in release evidence.
- [ ] **Step 5 — commit:** Commit only Judgify logger/business changes and test updates after all focused and canonical Go tests pass.

### Task 3: Traces and durable cross-process correlation

**Files:** Modify `ops/stack/alloy/config.alloy`, `ops/stack/tempo/config.yml`, `deploy/envoy/envoy.yaml.tmpl`, `api/internal/infrastructure/api.go`, `api/internal/infrastructure/worker/runtime.go`, `api/internal/platform/eventing/event.go`, `api/internal/platform/eventing/outbox.go`, `api/internal/ent/schema/outbox_event.go`, `api/internal/ent/schema/judge_job.go`, `api/internal/submission/adapters/driven/db/intake_writer.go`, `api/internal/submission/adapters/driven/db/job.go`, `api/internal/judge/durable_worker.go`, `api/internal/judge/log_context.go`; create a checked-in forward migration under `api/internal/ent/migrate/migrations/` and focused trace propagation tests in those packages.

**Interfaces:** Public Envoy request starts a new W3C trace independent of request/correlation IDs. API and Check inherit its `traceparent`. Extend the currently closed `eventing.Event` metadata with optional bounded `Traceparent`/`Tracestate` and persist them in nullable outbox/judge-job columns; the present schemas have no trace metadata, so an expand-only migration is required. Worker continues or links the origin trace without inventing a request ID. OpenTelemetry resources set distinct `service.name` and release `service.version`.

- [ ] **Step 1 — red tests:** Assert an untrusted public `traceparent` is not retained; an authenticated internal hop inherits the gateway-owned trace; outbox serialization round-trips trace context and `correlation_id`; a worker log with an active span has both trace/span IDs and no request ID.
- [ ] **Step 2 — instrumentation:** Configure Envoy tracing and bounded sampling, OTLP receiver/exporter in Alloy, and Go OTel SDK initialization/shutdown for API and worker. Add nullable, immutable `traceparent`/`tracestate` columns to outbox events and judge jobs with a checked-in Atlas migration; regenerate Ent code and keep legacy rows valid without trace context. Add matching optional fields to `eventing.Event`, pass them through submission intake, judge-job claim, and outbox relay, then instrument HTTP ingress, Check, durable outbox publish/consume, and judge operation boundaries. Do not attach `user_id` before authentication or persist baggage. The serialization test must preserve W3C context without treating it as the correlation ID:

  ```go
  require.Equal(t, originCID, decoded.CorrelationID)
  require.Equal(t, carrier.Get("traceparent"), decoded.Traceparent)
  require.NotEqual(t, originCID, trace.SpanContextFromContext(ctx).TraceID().String())
  ```
- [ ] **Step 3 — migration and running trace proof:** Regenerate Ent and update `atlas.sum`, then run `bash scripts/verify/migrations.sh --integration` against an isolated PostgreSQL fixture. Verify pre-existing outbox/job rows have nullable trace columns and still process under both new and N-1 binaries; do not require a down migration. Send one authenticated submission through Envoy, find its API and Check spans in Tempo, follow the outbox/worker continuation or link, and confirm the shared correlation ID in Loki. Verify an unsampled trace still leaves searchable correlation logs.
- [ ] **Step 4 — outage and load:** Stop Tempo/Alloy, issue a request and a judge job, and verify product completion; restore collector and ensure bounded exporter queues recover without unbounded memory growth. Record sampled request rate and exporter CPU/memory.
- [ ] **Step 5 — commit:** Commit instrumentation, collector config, and tests after focused Go tests and running trace proof pass.

### Task 4: Metrics, storage, dashboards, and alerts

**Files:** Modify `ops/stack/prometheus/prometheus.yml`, `ops/stack/docker-compose.yml`, `ops/stack/loki/config.yml`, `ops/stack/tempo/config.yml`, `ops/provisioning/prometheus-rules/judgify.yml`, `ops/provisioning/grafana-dashboards/judgify-service-overview.json`, `.github/workflows/ci.yml`, `scripts/verify/supply_chain.sh`, `scripts/verify/topology.sh`, `api/internal/infrastructure/api.go`, `api/internal/infrastructure/worker/runtime.go`, `api/internal/infrastructure/configuration/config.go`, `deploy/config.production.example.yaml`; create `api/internal/platform/processhealth/metrics_server.go`, `api/internal/platform/processhealth/metrics_server_test.go`, `scripts/verify/observability_metrics_test.sh`; update `ops/runbooks/api-availability.md`, `ops/runbooks/outbox-lag.md`, and `ops/runbooks/authorization-stale.md`.

**Interfaces:** Prometheus can scrape API, worker, Envoy, Alloy, Loki, Tempo, and itself through a private telemetry path. Existing API/worker probe servers bind `127.0.0.1`; do not silently change this security invariant. Add a metrics-only listener bound to each app container's fixed address on an internal observability network; Prometheus joins that network. Keep the existing loopback health probes unchanged. Test reachability from Prometheus and non-reachability from the public gateway, then update topology contracts explicitly.

- [ ] **Step 1 — red target test:** In `observability_metrics_test.sh`, query Prometheus `/api/v1/targets` and require every named target to be `up`; from the public gateway, assert `/api/metrics` and the telemetry ports are unreachable. Confirm the current self-scrape-only stack fails.
- [ ] **Step 2 — private scrape path:** Create one external Docker network with `docker network inspect judgify_telemetry >/dev/null 2>&1 || docker network create --internal --subnet 172.31.249.0/24 judgify_telemetry` and record the command plus a subnet-collision check in `ops/runbooks/deploy.md`. Attach API at `172.31.249.10`, worker at `172.31.249.11`, Envoy at `172.31.249.12`, and Prometheus/Alloy on the same network. Bind dedicated API and worker metrics-only listeners to `172.31.249.10:9092` and `172.31.249.11:9092`; leave `127.0.0.1:9090/9091` health probes intact and publish no telemetry host ports. Pin Prometheus, Alertmanager, Loki, Tempo, Alloy, and Grafana images by approved digest in `ops/stack/docker-compose.yml`; include all six in the production ops SBOM/CVE inventory. Give Prometheus exact scrape targets on the private network:

  ```yaml
  - job_name: judgify-api
    static_configs:
      - targets: ["172.31.249.10:9092"]
  - job_name: judgify-worker
    static_configs:
      - targets: ["172.31.249.11:9092"]
  ```

  Bind Prometheus and Alloy UIs to `127.0.0.1` host ports, require a nondefault Grafana admin secret, and keep Grafana reachable only through the operator tunnel. Add gateway 5xx/upstream and Check duration/error counters, API HTTP latency/5xx, worker capacity/queue age, and collector ingest failure metrics with bounded labels.
- [ ] **Step 3 — budgets:** Enable Loki compactor retention for ordinary 14-day logs, Tempo 7-day trace retention, and Prometheus 7-day TSDB retention; set disk-budget alerts. Apply 7-day security-log retention only with a proven `log_class=security` selector and enforce its read-access boundary separately.
- [ ] **Step 4 — dashboard and alert proof:** Update Grafana panels and data links from a log's `trace_id` to Tempo and from a trace to correlated Loki logs. Trigger one safe test alert through the configured Alertmanager receiver and record delivery evidence; verify runbook links resolve.
- [ ] **Step 5 — final checks and commit:** Run `bash scripts/verify/observability_metrics_test.sh`, the log and trace proofs, `bash scripts/verify/topology.sh`, `make verify-release`, and the existing CI source contracts; commit only telemetry/ops files after each check passes. Wire the live metrics/log/trace contract into the existing cross-stack CI jobs so `final-gate` reports its status alongside the existing separate API and website tracks.

## Wave acceptance

Grafana can follow one request from Envoy through API and worker logs and into Tempo where sampled. Loki labels are only trusted `service`/`environment` (plus conditional security policy label), with no high-cardinality IDs. Prometheus has real application/gateway targets and a delivered test alert. Telemetry failure does not block product traffic; retention and disk budgets are active.
