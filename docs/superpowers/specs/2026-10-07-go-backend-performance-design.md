# Judgify system observability and Go backend performance

Date: 2026-10-07

## Intent and scope

Make a user journey diagnosable across Judgify, then use that evidence to improve the performance of its entire Go backend. Operators must be able to move from a user-visible symptom to the responsible website, gateway, API, database, outbox, worker, judge, realtime, or infrastructure stage using metrics, traces, and correlated logs. Preserve the current modular monolith, PostgreSQL authority, sandbox boundaries, business outcomes, and `go-common` boundary. An optimization is selected only after measurements identify a bottleneck.

This is an umbrella design with independently reviewable workstreams: (1) end-to-end observability coverage and operator proof, (2) performance baseline and profiling, (3) judge phase visibility, and (4) targeted changes to a measured bottleneck. Each workstream must produce a usable result on its own and will receive a separate implementation plan. Website work is limited to telemetry for server rendering, outbound calls, browser experience, and errors. The existing Tempo, Prometheus, Loki, Alloy, and Grafana stack remains the backend for traces, metrics, logs, and dashboards.

## Current evidence

- `api/internal/platform/observability/catalog.go` already defines bounded HTTP and DB latency, judge queue/worker/sandbox, outbox, WebSocket, campaign, contest lag, and process metrics. `ops/observability/dashboard-judge.json` displays queue and worker health. There is no judge phase or attempt duration histogram in the metric catalog.
- `deploy/envoy/envoy.yaml.tmpl` configures OpenTelemetry tracing. `api/internal/platform/tracing/tracer.go` installs OTLP providers for API and worker, and the API has Gin/gRPC instrumentation. `ops/stack/alloy/config.alloy` forwards traces to Tempo and syslog to Loki. The current Go paths inspected do not show explicit spans across durable outbox/judge work or persisted `traceparent` in those records. A configured exporter or provider alone does not prove that a full submission trace exists.
- `ops/stack/prometheus/prometheus.yml` scrapes API and worker metrics, while Grafana provisions Prometheus, Loki, and Tempo. Existing alert rules cover API, judge, outbox, realtime, and other risks. The checked-in scrape config does not include website or Envoy targets, so their operational coverage needs an explicit source or documented reason. Collector health and alert delivery also need running proof.
- `website/e2e/performance.spec.ts` captures browser Web Vitals during acceptance runs. No production browser telemetry pipeline was found in the inspected website paths. Existing `X-Correlation-ID` in product contracts is useful for incident search, but it is not a trace ID.
- `api/internal/judge/durable_execution.go` runs a fenced lease through plan read, compile, case execution, and finalization. `api/internal/infrastructure/worker/metrics.go` is the worker-owned adapter for judge metric callbacks. `api/internal/infrastructure/worker/production_runtime.go` already bounds worker concurrency and local backlog.
- `api/internal/platform/eventing/store.go` already caps outbox claim size; worker and API configuration bound delivery concurrency and batches. The realtime metrics distinguish delivered, dropped, and rate-limited messages. Contest rerating already uses bounded batches and shadow state. These contracts must not be replaced by generic worker or cache abstractions.
- `test/load/submissions.js` sends real authenticated submissions, waits for terminal states, and checks loss, duplicates, and acknowledgement p99. `ops/performance/k6/scenarios/contest-final-spike.js` is a template whose spike calls a readiness path. These two tools must be described accurately in reports; a readiness spike is not evidence of judge throughput. The normal and contest workload models also include browsing, submission status reads, leaderboard polling, and authenticated writes.
- `ops/contracts/performance-strategy.md` states the workload hypotheses and reporting rules. `ops/performance/report-template.md` provides a report structure. Existing targets remain hypotheses until measured on production-like data and capacity.
- `api/internal/platform/processhealth/server.go` serves only private live, ready, and metrics routes. No controlled API or worker profiler is configured in the current code.

## Approaches considered

1. **Recommended: close observability gaps, measure each backend path, then change one measured bottleneck.** Reuse the current stack and workload tools, link synchronous and durable work with safe trace context, add bounded judge phase measurements and controlled profiling, then select one API/DB/background/realtime/judge optimization with before/after proof. This preserves clean module boundaries and makes regressions attributable.
2. **Copy Begroup's cache, batching, bitmap, and concurrency patterns across modules immediately.** These help specific read-heavy or large-set workloads, but may add stale reads, memory pressure, lock contention, and hidden failure modes where Judgify already has durable correctness rules.
3. **Raise worker counts and pool sizes first.** This can improve an identified saturation point, but without pressure evidence may simply move the bottleneck to PostgreSQL, sandbox CPU, or network and reduce total throughput.

## End-to-end observability contract

The September gateway/observability design in `docs/superpowers/specs/2026-09-13-envoy-gateway-observability-authz-design.md` defines the identity and privacy contract. This spec builds on it: `request_id` identifies one HTTP attempt, `correlation_id` identifies its durable cause, and W3C `trace_id`/`span_id` identify telemetry spans. These values are never interchangeable. The trusted Envoy edge owns public request and trace context; website and API propagate only validated context on trusted internal hops. An asynchronous consumer keeps the durable correlation ID and starts a linked trace when continuing the original trace as a child would misrepresent a long queue delay or retry. Sampling must be bounded and documented; unsampled work remains discoverable by correlation ID in structured logs.

| Stage | Trace or correlation boundary | Required operational view |
| --- | --- | --- |
| Browser and website | Capture production navigation/error/Web Vitals with bounded route names; instrument server rendering and outbound API calls. Relate a browser error to a server response's request/correlation ID where available; do not trust browser-supplied trace context at the public edge. | Route-level experience, server errors, rendering latency, and dependency failures |
| Envoy and authorization Check | Gateway request span and Check outcome with the same trusted request context; bounded route and outcome names | Traffic, upstream failures, Check latency/denials, and target health |
| Go API and PostgreSQL | HTTP root and selected use-case/DB child spans with code-owned operation names; preserve existing query-class metrics | Route p50/p95/p99, errors, connection pressure, query latency, locks, and database host pressure |
| Durable outbox, campaigns, rating, and judge jobs | Persist only minimal validated trace context with durable correlation; create consumer spans or links on claim, retry, publish, and terminal transition. Legacy records without trace context still process. | Backlog age/depth, attempts, retry/dead state, batch latency, worker capacity, and end-to-end completion |
| Sandbox and realtime | Parent judge spans cover compile/case/cleanup, without source or hidden case data. Trace WebSocket handshake and selected publish paths; avoid per-message spans at normal volume. | Phase latency, sandbox CPU/memory/OOM, connection counts, delivery/drop/close outcomes |
| Telemetry pipeline and infrastructure | Keep collector, scrape, and storage paths private; observe their own availability and dropped/export-failed data. | Host/container CPU, memory, disk, restarts, DB health, scrape status, collector lag/failure, retention budget |

Every critical journey has a Grafana entry point that links a symptom panel to relevant Tempo traces and Loki records. Provision a system overview plus focused gateway/website, API/DB, judge, background, realtime, and telemetry health views; extend existing dashboards where possible. Alerts cover user-impacting error/latency, absent required targets, judge/outbox age, worker unavailability, durable dead-letter state, sandbox OOM, database pressure, and telemetry ingestion failure. Each actionable alert names an owner, runbook, threshold/window, dashboard, and notification route. A missing telemetry target must be visible as unknown or failed, never mistaken for zero errors.

The operator drilldown is explicit: select a request from a Grafana panel or correlated Loki record, open its sampled Tempo trace, see the ordered service/span timeline with each duration, and open logs filtered to that trace and time window. The trace must distinguish gateway, Check, website/API, DB, and relevant synchronous work. A durable submission continues through a linked worker trace with queue wait, judge phase, retry, and terminal timing shown separately; a long queue delay is not rendered as CPU time in one span. For an unsampled request, the same entry point offers a correlation-ID log search and durable state/timing evidence, clearly indicating that no span waterfall exists. No view claims to contain logs for steps that were never logged.

Keep trace attributes and metric labels bounded: no raw URL, SQL, user ID, submission ID, source code, test data, token, or unbounded error text. Domain IDs may appear only in access-controlled structured logs needed to investigate a specific operation. Configure explicit sampling, exporter queue bounds, retention, and disk budgets. Telemetry outages must not change authorization, verdicts, durable delivery, or request completion. A trace is an investigative sample; metrics and durable records remain the source for rates, counts, and correctness.

The final structured-log field contract is event-scoped. This `metadata` object layout supersedes the flat event-field examples in the September gateway design:

| Event | Top-level fields | `metadata` object |
| --- | --- | --- |
| Every line | `timestamp`, `level`, `message`; trusted Loki labels `service`, `environment` | Omit the object when no event-specific fields exist |
| Cross-cutting identity | `request_id` only during an HTTP attempt; `correlation_id` when a durable cause exists; `trace_id`/`span_id` only with valid span context | None of these IDs is duplicated inside `metadata` |
| HTTP completion | Cross-cutting IDs and numeric `duration_ms` | `method`, bounded `route`, `status`; upstream, response flags, and byte counts where needed |
| Judge routine diagnostic | `correlation_id`; `trace_id`/`span_id` if available | `job_id`, `attempt_id` |
| Judge start, terminal, retry, or failure | Cross-cutting IDs; numeric `duration_ms` when an attempt completed | `job_id`, `attempt_id`, `submission_id`; `generation`, attempt ordinal, `judge_spec_id`, `verdict`, `failure_code`, and queue wait only on events that explain them |

The judge metadata remains searchable in structured logs; it is not a metric label or ambient field on every line. Keep one top-level numeric `duration_ms` per completed HTTP request and completed judge attempt: Tempo computes durations for retained spans, while logs still identify an individual operation after trace expiry or export failure. Phase timing lives in spans and histograms. Queue wait and submission-to-terminal latency are separate quantities and must never be inferred from HTTP or attempt `duration_ms`. Keep `metadata` bounded and omit it when empty; never emit two `metadata` keys in one JSON line. Review log volume per journey as well as telemetry storage.

`message` is a short, stable human-readable description of the event, never a serialized object. `level` expresses severity; `metadata` contains only the bounded fields needed to explain and search that event. Error logs use stable failure codes and safe error classes in `metadata`, not raw source, testcase output, credentials, or arbitrary error strings. Do not log a whole domain/request object. Log only identities available at the current boundary: do not carry an expired HTTP `request_id` into durable work, invent `trace_id`/`span_id` outside an active span, or duplicate an ID inside `metadata`. Envoy owns public HTTP completion logs; API and worker logs describe their own events. Preserve coverage for private requests that bypass Envoy without creating a second public completion record.

## Backend performance evidence contract

Use the checked-in `normal-1k-ccu`, `contest-1k-ccu`, `contest-final-spike`, and soak workload models as distinct profiles. A profile must exercise its declared HTTP reads/writes and background effects, not replace business traffic with health checks. Record API p50/p95/p99 by route template, 4xx/429/5xx, DB query duration and pool pressure, host/container CPU and throttling, process heap/goroutines, outbox age and delivery rate, WebSocket delivery/drop/close counts, judge arrival/completion/queue metrics, and contest projection/rating lag when the profile triggers them. Current pool telemetry includes open/in-use/idle connections and cumulative `DBStats.WaitCount`; derive its increase over the run, and add wait-duration instrumentation only if that count and query latency cannot explain a suspected pool bottleneck. A run that omits a required behavior is labelled partial and cannot prove the profile's capacity target.

Each report records the exact commit and images, CPU/memory limits, database size and connection limits, dataset revision, traffic mix, request rate, concurrency, and load-generator location. Extend `ops/performance/report-template.md` with DB, outbox, realtime, and resource observations so the whole backend is reported alongside judge results. Name the bottleneck evidence rather than attributing latency from a single aggregate graph. Compare repeated runs on the same environment; do not interpret run-to-run noise as a performance gain.

For the submission portion, use the existing real `test/load/submissions.js` path with a non-production account and released problem fixtures. Record worker count, sandbox image, and source/runtime classes. Use at least one fast accepted solution, one compilation failure, and one case-heavy solution so a single verdict class does not hide the bottleneck. Test source and hidden case contents are never written to report artifacts.

For submissions, separate HTTP acknowledgement latency, queue wait, worker attempt time, and submission-to-terminal latency. Report p50/p95/p99 where samples permit, accepted/completed rates, peak queue depth, queue drain time, and lost or duplicate submissions. Correlate accepted IDs with terminal IDs; do not count a 202 acknowledgement as a completed verdict. The existing `ops/contracts/performance-strategy.md` thresholds are the pass criteria for matching workloads; an unmeasured profile has no pass claim.

The k6 `contest-final-spike` scenario continues to be labelled a template until it issues real authenticated submissions and observes completion. Prefer one canonical workload definition shared by the Node submission harness and k6 scenario rather than two conflicting claims of the same profile. A run is invalid if credentials, fixture revision, environment, or terminal observation are missing.

## API, database, background, and realtime diagnosis

Use the existing HTTP route-template and DB query-class histograms as the first diagnostic layer. For a slow API route, compare HTTP time with DB time, the change in pool `wait_count`, external dependency time where applicable, and CPU/heap profiles before adding a new metric. Add an operation-specific span or histogram only when the existing views cannot distinguish two plausible causes; its operation names and outcomes must be code-owned finite enums.

For outbox and other durable background work, retain the existing age, state, retry, and reconciliation metrics as the authority for backlog. Measure one complete delivery/reconciliation batch only if a profile shows lag despite available capacity. Preserve current claim limits, byte limits, transaction boundaries, and idempotent acknowledgement; a larger batch is not automatically faster if it lengthens locks or delays retries. The same rule applies to contest rerating and notification campaign batches, whose progress and partial-failure contracts must remain truthful.

For realtime, start with existing delivered/dropped/rate-limited/close counters and process pressure. A faster path must not turn bounded backpressure into unbounded buffering or weaken per-topic authorization. If fan-out is the measured bottleneck, compare delivery latency and slow-client drops at the same connection mix before changing queue or worker sizes. Loss of ephemeral messages and loss of durable events remain distinct outcomes.

## Judge phase instrumentation

Add a Judgify-owned histogram `judgify_judge_phase_duration_seconds` with a closed `phase` enum: `plan_read`, `compile`, `case_execution`, `finalize`, and `cleanup`; and a closed `outcome` enum: `success`, `error`, `cancelled`. A second histogram, `judgify_judge_attempt_duration_seconds`, records the whole `ExecuteLease` call with the same outcome enum. The phase metric records `case_execution` once per attempt for the aggregate case loop, not once per testcase. The bucket set must cover milliseconds through the configured maximum practical attempt duration; the existing HTTP/DB buckets ending at 30 seconds are insufficient for judge work.

The observer interface belongs beside the judge execution dependency contracts and has no Prometheus or OpenTelemetry import. The worker infrastructure adapter in `api/internal/infrastructure/worker/metrics.go` maps its fixed enums to the code-defined catalog. Instrumentation never receives source code, testcase input/output, user ID, submission ID, job ID, runtime key, or unbounded error text as a metric label. A missing observer is a no-op for development and focused tests; metrics failures never change a verdict, lease transition, retry, or finalization result.

Measure phase time at the existing operation boundaries, including failure and cancellation paths. The total attempt measurement begins after lease validation and ends after the attempt's cleanup path. A phase is recorded only when entered. The `cleanup` phase includes artifact release and sandbox/staging teardown where those costs are owned by this execution path; any cleanup that happens outside `ExecuteLease` must be named separately rather than silently counted as zero. The dashboard shows phase p95 and attempt p95 alongside existing queue depth, oldest ready age, capacity, and finalization outcomes. Do not infer queue wait from attempt duration.

## Profiling contract

Expose Go CPU, heap, goroutine, mutex, and block profiles for the API and judge-worker roles through explicitly enabled, loopback-bound diagnostic listeners in a non-production environment. Each listener is absent by default, is not mounted on the public API or normal private health server, and is not published by Docker Compose. Startup rejects a non-loopback bind address. Profile capture has a bounded duration and stops cleanly with the process lifecycle. Profile artifacts are kept outside the repository and reviewed for sensitive data before sharing.

Continuous profiling is a later decision. Add it only if repeated investigations show that on-demand profiles cannot capture the problem and its CPU/storage overhead is measured. Begroup's Pyroscope package is a reference for supported profile types, not a dependency to import into Judgify.

## Optimization decision rules

After the baseline, make one change at a time and compare the same workload, fixture, machine limits, and worker count. Every change must preserve HTTP contracts, authorization, data freshness, durable delivery, judge result equivalence, ordering rules, resource limits, and retry semantics.

| Measured bottleneck | Candidate technique | Required proof before adoption |
| --- | --- | --- |
| Repeated immutable read-model fetches dominate API time | Bounded in-process cache; add Redis only if multi-instance sharing is itself measured as necessary | Show repeated identical reads, a stable version/key, invalidation or bounded TTL with jitter, memory cap, and a correct DB fallback; never cache authorization decisions |
| Repeated immutable judge metadata reads dominate DB time | Batch related reads or cache only verified metadata by release identity and checksum | Show repeated identical reads; prove immutable identity, bounded memory, and no retention of source or hidden case payloads |
| DB round trips dominate a bounded operation | Chunk/batch SQL within driver parameter and transaction limits | Compare query count, lock time, p95, memory, and rollback behavior; no empty or oversized batches |
| CPU time dominates a Go-owned phase | Profile-guided local algorithm/data representation change | CPU profile names the hot path and the replacement produces identical outputs for the released fixtures |
| Sandbox or host resources saturate | Tune existing worker concurrency or scheduling limits | Demonstrate throughput and queue drain improvement without increased OOM, CPU throttling, disk pressure, or verdict errors |
| Realtime fan-out saturates at a known connection mix | Bounded queue or fan-out tuning | Show reduced delivery latency without unbounded memory, increased slow-client drops, or altered topic authorization |

Roaring Bitmap is not a default backend optimization: no Judgify hot path has yet demonstrated a large integer-set membership bottleneck that outweighs conversion and synchronization costs. Multi-layer cache is not a default judge optimization: durable plan integrity and test secrecy have precedence over a speculative cache hit. Cheap prefiltering is allowed only when it cannot remove an otherwise valid result; unlike Begroup's geographic filter, many judge and authorization checks cannot be approximated safely. Testcase execution remains sequential unless a separate design proves ordering, early-stop, resource-limit, and verdict equivalence under parallel execution. Any parallel batch uses an explicit concurrency limit and cancellation policy.

## Boundaries and failure behavior

`go-common` owns generic bounded metric primitives and the reusable structured-log event helper. Its logger reads `correlation_id` and active `trace_id`/`span_id` from context when a line is written, and its event helper places optional numeric `duration_ms` and event-specific fields under one `metadata` object. Judgify owns event messages, safe business metadata, product metric names, fixed phase labels, benchmark fixtures, diagnostic access policy, cache keys and invalidation contracts, and optimization decisions. Judgify currently consumes the local `go-common` module through a `replace` directive; a tagged module release is a separate integration step.

Profiling and metrics are diagnostic side paths. They must not acquire business authority, write to the database, log credentials, source or hidden test data, or block a request or job when a collector is unavailable. Existing memory, disk, sandbox, and backpressure limits remain enforced. A process that cannot start its optional diagnostic listener fails startup with an explicit error only when profiling was deliberately enabled; the normal disabled path is unaffected.

## Verification and rollout

1. Inventory each stage in the table against deployed signals and record missing trace, metric, log, dashboard, alert, and retention links. Complete the end-to-end telemetry path before treating a performance report as system-wide evidence.
2. Prove one normal read and one real submission journey from website/Envoy through API and, for the submission, durable queue and worker to terminal result. In Grafana, follow a sampled trace across synchronous spans and the linked async trace, then find the same durable correlation in logs. Exercise an error, retry, and unsampled journey; verify no source/test data or secret appears. Show website Web Vitals and rendering errors alongside gateway/API impact without accepting forged browser trace context.
3. Prove Prometheus target status for required services, collector/export failure visibility, dashboard data links, and delivery of one safe test alert. Stop Alloy/Tempo briefly in a non-production environment and confirm bounded loss/recovery of telemetry while the product journey completes. Record actual storage and exporter overhead.
4. Record baseline runs for normal practice, contest steady state, contest final spike, and soak where the environment can support each profile. Exercise declared HTTP and background behaviors. Use the existing real submission harness for the submission portion. Preserve raw evidence and completed reports without secrets; label any unavailable profile as unmeasured rather than passing.
5. Add judge phase and attempt metrics with fixed-label checks for success, infrastructure failure, cancellation, and early terminal verdicts. Confirm one attempt emits one total observation and only entered phases emit phase observations. Existing outbox, realtime, and HTTP/DB metrics remain comparable with the baseline.
6. Verify API and worker profilers cannot bind non-loopback addresses, are absent when disabled, and shut down with their process. Capture profiles under representative workloads and compare CPU/memory overhead with profiling disabled.
7. Run the same workloads after instrumentation. If measurement overhead changes p95 or CPU beyond run-to-run variation, reduce sample volume or implementation cost before using the data for optimization decisions.
8. Select one measured bottleneck for a separately reviewable optimization change. Publish before/after evidence, business-result equivalence, durability and authorization checks, and rollback criteria. Do not declare a capacity increase from a template k6 run.

## Completion criteria

- The observability workstream demonstrates browser/website, gateway, API/DB, durable background, judge, realtime, infrastructure, and telemetry-pipeline views with a sampled end-to-end journey, an unsampled correlation lookup, target health, and an alert reaching its receiver.
- From a sampled request in Grafana, an operator can open the span timeline and its filtered logs without manually copying identifiers; for a durable submission the linked worker trace is reachable from that investigation. The displayed step durations reconcile with separate queue and terminal latency measures. An unsampled request shows the correlation-based fallback explicitly.
- The baseline and profiling workstream delivers reports for the declared API, DB, realtime, background, and judge workload behaviors, clearly marking unmeasured ones. Reports state workload, environment, error/loss counts, and uncertainty; they state the saturation point only when a run reaches one safely.
- An operator can separate HTTP, database, queue, judge phase/attempt, and terminal latency for a real submission workload without exposing per-request IDs in metric labels. The judge dashboard retains its queue and sandbox view and adds phase and attempt latency.
- Non-production API and worker profiles identify CPU/heap/goroutine pressure while normal processes expose no profiling listener.
- A targeted optimization is a separate workstream selected from the measured bottleneck. Its own completion requires a comparable before/after report and evidence of no regression in authorization, data freshness, verdict, durable delivery, or bounded backpressure behavior.
- Existing worker capacity, lease, retry, finalization, sandbox, outbox, realtime, and `go-common` boundaries remain authoritative.

## Begroup references

- `begroup/be-promo/internal/services/coupon/service.go`: layered reads from RAM, Redis, and DB.
- `begroup/be-promo/internal/services/coupon/caching.go`: cache expiry jitter and Redis batch read.
- `begroup/be-promo/internal/stores/account.go`: SQL chunking by parameter budget.
- `begroup/be-dispatch-core/internal/services/filtering/initial.go`: cheap filtering before more expensive routing.
- `begroup/be-promo/internal/services/campaign/bitmap/manager.go`: compressed integer-set representation, considered only for a measured matching workload.
- `begroup/be-dispatch-core/cmd/server/server.go` and `begroup/belabs/pyroscope/pyroscope.go`: on-demand and continuous Go profiling examples.

These are techniques to evaluate against Judgify's measured bottleneck, not dependencies or code to copy verbatim.
