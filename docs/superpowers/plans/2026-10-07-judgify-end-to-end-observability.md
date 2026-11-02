# Judgify End-to-End Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Follow a Judgify user journey through website, Envoy, Go API, PostgreSQL, durable work, judge, and realtime with trustworthy metrics, traces, correlated logs, dashboards, and alerts.

**Architecture:** Retain the existing Grafana, Prometheus, Loki, Tempo, and Alloy stack. Add bounded telemetry at existing ownership boundaries; persist minimal W3C origin context for durable work and link later worker spans while keeping the existing correlation ID authoritative. Prove the deployed path with real journeys before planning performance optimization.

**Tech Stack:** Go/OpenTelemetry, Gin, gRPC, Ent/Atlas/PostgreSQL, Envoy, Next.js 16, Prometheus, Alloy, Loki, Tempo, Grafana, Bash.

**Spec:** `docs/superpowers/specs/2026-10-07-go-backend-performance-design.md`

## Global Constraints

- Preserve the modular monolith, PostgreSQL authority, lease fencing, sandbox isolation, idempotency, and `go-common` boundary.
- `request_id` is one HTTP attempt; `correlation_id` is the durable cause; W3C `trace_id` and `span_id` are telemetry identities. Do not equate them.
- Envoy discards public `traceparent` and `tracestate`; only authenticated internal hops may propagate validated trace context.
- Keep metric labels and span names finite. Never put raw URLs, SQL, user/submission IDs, credentials, source, hidden cases, or arbitrary error text in labels or span attributes.
- Keep logs event-scoped: `timestamp`/`level`/`message`, cross-cutting request/correlation/trace/span IDs, and completion `duration_ms` are top-level; trusted `service`/`environment` are Loki labels. Put other event-specific fields in one bounded `metadata` object, omitted when empty. `message` is a short event description, never an object dump.
- Retain top-level numeric `duration_ms` on HTTP and judge-attempt completion records for unsampled investigations; use spans/histograms for phase durations and keep queue wait separate. Job/submission/attempt IDs, generation, verdict, and safe failure code/class are searchable `metadata` keys, never metric labels. Never carry an expired HTTP `request_id` into durable work or invent trace/span IDs.
- Trace sampling, exporter queues, log volume, retention, and disk use are bounded. Telemetry failure must not change business outcomes.
- Use current dashboards, metrics, and alerts as a base. Do not replace Tempo with Jaeger or add a second observability stack.
- Read `website/AGENTS.md` and the installed Next.js 16 guide in `website/node_modules/next/dist/docs/` before changing website code.
- Each task uses its listed focused tests during implementation; integration proofs run only in an isolated non-production environment.

## Review Focus

- Forged public trace headers: Envoy must replace them; the server must not attach a browser-selected parent trace (Task 4).
- Pre-migration durable rows: outbox events and judge jobs with null trace context must still deliver and finalize (Task 2).
- Retry after a long delay: the worker span must link to the origin rather than pretend that queue time was one active parent span (Task 3).
- Collector outage: exporter buffers remain bounded while requests, outbox delivery, and verdicts complete (Task 6).
- Missing metrics target: dashboards and alerts show missing/unknown instead of a healthy zero (Task 5).

## File map

| Unit | Files and responsibility |
| --- | --- |
| Shared log event helper | `../go-common/pkg/logger/{context,event}.go` reads active trace identity and writes the common event envelope; Judgify supplies event metadata |
| Durable trace carrier | `api/internal/platform/tracing/durable.go` validates/captures W3C context and constructs links; tests in `durable_test.go` |
| HTTP log boundary | `api/internal/infrastructure/{router,request_log_context}.go` keeps request identity and logger injection without duplicate public access logs or ambient session IDs; Envoy owns public completion records |
| Outbox persistence | `api/internal/platform/eventing/{event,outbox,store}.go`, `api/internal/ent/schema/outbox_event.go`, generated Ent and Atlas migration carry optional origin trace context |
| Judge persistence | `api/internal/ent/schema/judge_job.go`, `api/internal/submission/{domain,adapters/driven/db}`, `api/internal/ports/submission.go` carry the same optional context through intake and claim |
| Runtime spans and logs | `api/internal/infrastructure/event_delivery*.go`, `api/internal/judge/{durable_worker,durable_execution,log_context,event_log}.go`, `api/internal/realtime/hub_publisher.go`, and `api/internal/platform/observability/catalog.go` add code-owned spans and one bounded judge event-metadata object |
| Website signals | `website/instrumentation.ts` or the installed Next.js 16 equivalent, `website/lib/api/client.ts`, and a bounded browser-vitals adapter own website-side signals |
| Ops views | `ops/stack/*`, `ops/provisioning/*`, `ops/observability/*`, and `scripts/verify/observability*.sh` own target coverage, dashboards, alerts, and operator proof |

---

### Task 0: Normalize the HTTP log boundary

**Files:** Modify `api/internal/infrastructure/router.go` and `request_log_context_test.go`; create `api/internal/infrastructure/request_logger_context.go` and focused tests for its middleware. Review `api/internal/infrastructure/gateway_identity.go` and direct-private HTTP entrypoints for access-log coverage. Reuse the shared `go-common/pkg/logger.Event` helper for direct-request completion logs.

**Interfaces:** Preserve `middlewares.Correlation()` and request logger injection into `context.Context`; keep `request_id` only on the live HTTP request and `correlation_id` as the durable cause. Stop emitting a second API completion log for public requests already logged by Envoy. Keep a documented completion-log owner for private direct HTTP traffic. Remove ambient `session_id`; attach `user_id` only where a specific event needs it. Continue rejecting raw credentials and token IDs.

| HTTP path | Completion evidence owner |
| --- | --- |
| Public request through Envoy | Envoy access log |
| Direct API request in legacy/development mode | API completion log |
| Private process health and metrics probes | Health/target metrics; no per-probe access log |

- [ ] Add focused middleware tests for request/correlation context, logger injection, absent ambient session/user ID, public completion ownership, and private direct-request coverage. Check that no request query or body appears in logs.
- [ ] Run `cd api && go test ./internal/infrastructure -run 'RequestLog|GatewayIdentity|RequestLogger' -count=1`; expect the new assertions to fail first.
- [ ] Implement the API-side boundary without changing correlation or auth behavior. Review public and private routes against the access-log owner table.
- [ ] Run the focused tests and `cd api && go test ./internal/infrastructure -count=1`; expect pass.
- [ ] Commit this self-contained log-boundary change.

### Task 1: Durable W3C context helper

**Files:** Create `api/internal/platform/tracing/durable.go` and `durable_test.go`.

**Interfaces:** Produce `CaptureTraceparent(ctx context.Context) string` and `OriginLink(traceparent string) (trace.Link, bool)`. `CaptureTraceparent` returns empty when the context has no valid sampled span; `OriginLink` accepts only a bounded valid W3C value and never copies baggage. Tasks 2 and 3 consume these functions.

- [ ] Write `TestCaptureTraceparent` and `TestOriginLink` for sampled/unsampled, missing, malformed, oversized, and forged-looking values. Assert no baggage or ID is returned as a metric label.
- [ ] Run `cd api && go test ./internal/platform/tracing -run 'TestCaptureTraceparent|TestOriginLink' -count=1`; expect the new tests to fail before implementation.
- [ ] Implement the two functions using the existing OpenTelemetry propagator and trace API. A valid sampled origin is a link target, not a parent for a new durable attempt.
- [ ] Run the same focused command; expect pass. Run `cd api && go test ./internal/platform/tracing -count=1`; expect pass.
- [ ] Commit only the helper and focused tests.

### Task 2: Persist optional origin context with durable work

**Files:** Modify `api/internal/platform/eventing/{event,outbox,store}.go`, `api/internal/ent/schema/{outbox_event,judge_job}.go`, `api/internal/submission/domain/job.go`, `api/internal/ports/submission.go`, `api/internal/submission/adapters/driven/db/{intake_writer,job}.go`, associated focused tests, generated Ent files, and one additive Atlas migration under `api/internal/ent/migrate/migrations/`.

**Interfaces:** Add optional `Traceparent string` to `eventing.Event` and optional `OriginTraceparent string` to the judge job domain/lease. Add nullable immutable `traceparent` to outbox rows and `origin_traceparent` to judge jobs, max 55 characters, with validation through Task 1. When no explicit value is supplied, append/intake captures the sampled span from `ctx`; malformed explicit values are dropped. Both Ent and raw SQL append/intake paths write the value. `messageFromRecord` maps an outbox value to the existing `commonoutbox.Metadata.Attributes` entry with key `traceparent`; consumers extract it from that bounded attribute. No `go-common` API change is needed. Existing rows decode as empty and continue normally. Keep `CorrelationID`/`OriginCID` unchanged.

- [ ] Write focused tests in `api/internal/platform/eventing/outbox_test.go`, `store_test.go`, and submission DB tests: valid context survives Ent and raw SQL round trips; malformed context is dropped without rejecting business work; null legacy rows deliver/claim; existing idempotent replay does not rewrite origin context.
- [ ] Run `cd api && go test ./internal/platform/eventing ./internal/submission/adapters/driven/db -run 'Traceparent|LegacyTrace|ReplayTrace' -count=1`; expect failure before the schema/mapping changes.
- [ ] Add the nullable fields and wiring. Regenerate Ent with `make generate-ent` and create one additive Atlas migration using `make migrate-diff name=durable_trace_context`; inspect SQL and `atlas.sum` before committing. Keep every non-telemetry transaction and index unchanged.
- [ ] Run the focused tests, `scripts/verify/migrations.sh --check`, and `scripts/verify/generated.sh`; expect pass. Run `scripts/verify/migrations.sh --integration` against an isolated database; expect old rows and new rows to process.
- [ ] Commit the schema, migration, generated code, mapping, and focused tests together.

### Task 3: Link API, outbox, and judge execution spans

**Files:** Modify `api/internal/infrastructure/event_delivery*.go`, `api/internal/judge/{durable_worker,durable_execution,log_context}.go`, `api/internal/infrastructure/worker/{runtime,metrics}.go`, `api/internal/realtime/hub_publisher.go`, `api/internal/platform/observability/catalog.go`, `api/internal/platform/tracing/tracer.go`; create `api/internal/judge/event_log.go` and focused Go tests.

**Interfaces:** Name spans with code-owned operations such as `outbox.claim`, `outbox.deliver`, `judge.claim`, `judge.attempt`, `judge.compile`, `judge.case_execution`, `judge.finalize`, `db.query`, and `realtime.publish`. The existing Gin span covers WebSocket handshake; add one bounded publish span per selected business event, not per connection/message. `ObserveDatabase` creates a class-only child span from its existing `StatementObservation`, without SQL text or arguments. Start a new worker attempt trace with `OriginLink` from Task 1 and emit one linked attempt per retry. Keep only `correlation_id` as cross-cutting judge context in `api/internal/judge/log_context.go`. `go-common/pkg/logger.FromContext` attaches active `trace_id`/`span_id` when the line is written, and `go-common/pkg/logger.Event` emits top-level numeric `duration_ms` and exactly one optional `metadata` object. Judgify supplies code-owned, bounded `job_id`/`attempt_id`, `submission_id` on start/terminal/retry/failure, and generation, attempt ordinal, judge spec, verdict, or failure code only on relevant transitions. The normal unsampled path still has correlation logs.

- [ ] Add focused tests for linked origin on first claim and retry, null origin fallback, cancellation, infrastructure failure, and early terminal verdict. Add DB observer and realtime publish tests proving finite names and absent SQL/payload. Assert lease/result transitions stay unchanged, cross-cutting IDs and numeric completion `duration_ms` remain top-level, one bounded `metadata` object contains the right judge fields, and no duplicate JSON key or secret appears.
- [ ] Run `cd api && go test ./internal/judge ./internal/infrastructure/worker ./internal/infrastructure ./internal/realtime ./internal/platform/observability ./internal/platform/tracing -run 'Trace|Span|Lease' -count=1`; expect the new assertions to fail before implementation.
- [ ] Instrument only operation boundaries, never each testcase or WebSocket message. Propagate current API span context into outbox/judge creation using Task 2. Keep exporter errors off the verdict and transaction paths.
- [ ] Run the focused tests and `cd api && go test ./internal/judge ./internal/infrastructure/worker ./internal/infrastructure ./internal/realtime ./internal/platform/observability ./internal/platform/tracing -count=1`; expect pass.
- [ ] Commit spans, log enrichment, and focused tests.

### Task 4: Website and gateway visibility

**Files:** Modify `deploy/envoy/envoy.yaml.tmpl`, `website/lib/api/client.ts`, `website/lib/api/client.test.ts`, `website/e2e/performance.spec.ts`, and the installed Next.js 16 instrumentation entrypoint; create `website/app/telemetry/vitals/route.ts` and a small browser-vitals adapter with focused tests under `website/lib/observability/`.

**Interfaces:** Server-side website spans use service name `judgify-website` and forward valid trusted W3C context on internal API calls. Envoy's access log keeps `request_id`/`correlation_id` and numeric `duration_ms` at the top level and emits method, bounded route, status, upstream, flags, and byte counts in one nested `metadata` object. A 1% sampled browser adapter sends only a finite route key, one of `LCP`/`INP`/`CLS`/`TTFB`/`client_error`, and a bounded numeric value to same-origin `/telemetry/vitals`; the route rejects bodies over 1 KiB, unknown keys, and non-finite values, then emits a structured `web.vital` log for Loki. The route has a bounded per-process admission budget so a malicious caller cannot turn it into unlimited log volume. Where a server response exists, its validated request/correlation ID can be shown in an error diagnostic but is never part of RUM. Public browser requests cannot choose Envoy's trace parent. Keep the existing API client error/CID contract.

- [ ] Read `website/AGENTS.md` and the installed Next.js 16 instrumentation guide. Write website tests for server outbound propagation, missing context, rejected external origin, browser-vitals sampling/route/body bounds, admission budget, and no sensitive fields. Add a static Envoy contract asserting public `traceparent`/`tracestate` removal, top-level numeric `duration_ms`, and one nested `metadata` object in access logs.
- [ ] Run the focused website tests with `cd website && npm test -- lib/api/client.test.ts lib/observability` and the Envoy contract; expect new assertions to fail first.
- [ ] Implement the smallest supported Next.js instrumentation hook and bounded browser signal path through `/telemetry/vitals`; retain same-origin client rules. Update Envoy route precedence only if the current website fallback does not reach this route.
- [ ] Run focused tests, `cd website && npm run typecheck`, and Envoy config validation from the existing deployment verifier; expect pass.
- [ ] Commit website/gateway telemetry and tests.

### Task 5: Complete operator views and actionable alerts

**Files:** Modify `ops/stack/prometheus/prometheus.yml`, `ops/stack/alloy/config.alloy`, `ops/stack/grafana/provisioning/datasources/datasources.yml`, `ops/provisioning/prometheus-rules/judgify.yml`, `ops/provisioning/grafana-dashboards/judgify-service-overview.json`, focused dashboard/alert files in `ops/observability/`, relevant `ops/runbooks/*.md`, and `scripts/verify/observability_stack_test.sh`.

**Interfaces:** Prometheus sees API, worker, Envoy, website, and collector target health through private endpoints or a documented equivalent signal where a process has no Prometheus endpoint. Provision a Loki top-level `trace_id` derived link to Tempo and a Tempo-to-Loki trace/time-window link using the pinned Grafana version's supported datasource settings. Dashboard request/error rows expose the same navigation; durable-work views expose the linked worker trace or correlation fallback. Loki event searches parse nested `metadata` keys; no ID becomes a Prometheus or Loki label. Alerts cover missing targets, collector/export failure, gateway/website/API impact, DB pressure, judge/outbox lag, and durable dead state; every page has owner, dashboard, runbook, threshold/window, and receiver.

- [ ] Extend static configuration tests for required target jobs, `up == 0`/`absent` behavior, finite labels, Loki-to-Tempo and Tempo-to-Loki data links, and private-only listeners. Run `bash scripts/verify/observability_stack_test.sh`; expect the new assertions to fail first.
- [ ] Wire the minimum private exporters/scrape targets and collector self-metrics; update dashboards and alert rules without duplicating existing API/worker rules. Add runbook links and an explicit unknown state when a target disappears.
- [ ] Run `bash scripts/verify/observability_stack_test.sh` and `bash scripts/verify/observability.sh`; expect pass. In isolated ops stack, query `/api/v1/targets` and trigger one safe test alert through Alertmanager; record receiver evidence.
- [ ] Commit ops configuration, views, runbooks, and verification scripts.

### Task 6: Running end-to-end and outage proof

**Files:** Create `scripts/verify/observability_journey.sh` and `ops/observability/journey-proof.md`; modify `ops/contracts/alert-policy.md` only if actual receiver evidence changes its operational contract.

**Interfaces:** The script accepts non-production base URL and released test fixtures through environment variables; writes no credentials, source, hidden cases, or tokens to artifacts. Its report records commit, image versions, environment, sample policy, target health, IDs needed for operator lookup, and links/queries used. It exits nonzero if a required stage is absent.

- [ ] Write the script's failing contract checks: one read and one real submission to terminal state, sampled synchronous spans plus linked worker trace, unsampled correlation lookup, error/retry journey, browser Web Vitals, all required target health, and delivered test alert. Record an operator walkthrough that starts at a request/error row, opens Tempo's ordered spans and durations without copying IDs, opens filtered Loki logs, follows the async worker link, and verifies queue wait is separate from execution time. Add a collector-outage case that verifies normal product completion and bounded exporter behavior.
- [ ] Run `bash scripts/verify/observability_journey.sh` against an isolated stack; expect it to identify missing stages before the final wiring is complete. Do not use a readiness endpoint as a submission substitute.
- [ ] Complete only wiring defects identified by the proof in their owning files. Re-run the script and existing observability verifier; preserve the resulting redacted report in `ops/observability/journey-proof.md` with any unavailable stage explicitly marked unmeasured.
- [ ] Commit the proof script and redacted report. Request a whole-branch review against the spec before any production rollout.

## Handoff to performance work

After this plan passes and the observability coverage is proven, write separate plans for the performance baseline/profiling workstream and judge phase metrics. Select an API, DB, background, realtime, or judge optimization only after a report names a measured bottleneck. The current k6 final-spike template is not a capacity result.
