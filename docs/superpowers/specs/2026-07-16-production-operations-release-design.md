# Judgify Production Operations and Release Design

**Date:** 2026-07-16  
**Status:** Draft for written review  
**Deployment phase:** Single production host with explicit seams for additional API and worker instances

## 1. Outcome

Judgify ships as a reproducible, observable, recoverable product on one machine. Acknowledged submissions and committed admin commands survive process crashes. API and judge execution are isolated into separate processes. Schema changes are explicit. Health, logs, metrics, backups, load tests, crash tests, and rollback evidence are release requirements rather than post-deployment additions.

The one-host phase is not presented as high availability: loss of the host can interrupt service. It still provides strong data durability through PostgreSQL and encrypted off-host backups, bounded failure domains, fast process recovery, and contracts that do not require redesign when another host is introduced.

## 2. Release artifacts and process roles

### 2.1 One Go binary

The backend produces one immutable versioned binary:

```text
judgify api
judgify judge-worker
judgify migrate
```

The release manifest records:

- Git commit and semantic release version.
- Go version and go-common version.
- Database migration checksum/head.
- HTTP OpenAPI and WebSocket schema versions.
- Sandbox image names and immutable digests.
- A checksum-bound explicit set of complete worker capability tuples. Each
  tuple names the judge-spec and supervisor protocols, runtime/profile,
  compiler/bootstrap, command IDs, image/artifact identities, checker/version,
  and capability hash. The set is an allow-list for worker bootstrap/probes;
  it is never inferred by combining runtime and checker lists.
- Next.js build ID and asset manifest hash.
- Deterministic source epoch (normally commit time) and supported configuration schema version. Wall-clock packaging time is recorded outside the binary so a rebuild from the same inputs remains byte-reproducible.

### 2.2 `judgify api`

The API process owns:

- HTTP REST endpoints.
- WebSocket upgrade and local hub.
- Authentication, session, authorization runtime, and admin commands.
- Domain command/query handlers.
- Lightweight durable outbox/inbox dispatchers that do not access Docker.
- Contest lifecycle reconciliation/scheduling through idempotent commands.
- Notification projections.
- Leased security-email delivery for reset/account-security messages; provider I/O never runs in request transactions.
- Liveness, readiness, and metrics endpoints.

It never mounts or accesses the Docker socket and never compiles or executes contestant code.

### 2.3 `judgify judge-worker`

The judge worker owns:

- PostgreSQL judge-job claim, lease, heartbeat, retry, and dead-state processing.
- Bounded fair scheduler and admission controls.
- Compiler/runner orchestration.
- Disposable sandbox lifecycle.
- Per-attempt/per-test persistence and transactional terminal finalization.
- Worker-local health and metrics bound to a private interface.

It is the only Judgify process with sandbox runtime permission. It does not expose product HTTP routes or accept browser traffic.

`judge_worker_instances` contains process/boot identity, build/lifecycle state, `claims_enabled`, capacity, probe summary, `started_at`, and `last_heartbeat_at`; it does not encode compatibility as independent arrays or broad min/max ranges. Normalized `judge_worker_capabilities` rows belong to the current instance/boot and each advertise one complete tuple of judge-spec protocol, runtime key/profile version, sandbox image digest, artifact-manifest version/checksum, supervisor protocol version, checker key/version, and capability-profile hash, plus passing probe/smoke evidence. A tuple is usable only while its exact boot heartbeat is fresh and claims-enabled. Stale or claims-disabled rows are never evidence of capacity or compatibility.

The claim query joins the immutable judge spec to one exactly equal normalized capability row and its fresh current boot; it never derives an untested Cartesian product from separately advertised lists. Enabling claims is guarded by the worker-local atomic gate plus durable row update; disabling closes the local gate before drain.

Activating a new emitted tuple requires one fresh, ready, claims-enabled worker with that exact passing capability row and a golden smoke judgement built from the same complete tuple. The successful audited gate writes `activated_judge_capabilities` with the tuple, release, actor/reason, smoke-evidence checksum, status, and activation time. After activation, a transient worker outage does not make the whole API globally unready: judge admission becomes degraded/alerting, while the durable queue may continue accepting submissions only below explicit validated `max_ready_depth`, `max_oldest_ready_age`, and minimum free-disk safety thresholds. Submission intake returns retryable `503 judge_capacity_unavailable` when its complete target tuple has no active record or any admission cap is reached; unrelated API/read routes remain ready. A stale heartbeat can never activate a capability or claim work.

### 2.4 `judgify migrate`

The migration command:

- Acquires a PostgreSQL advisory lock.
- Verifies migration checksums and current schema head.
- Applies pending forward migrations in order.
- Records application/migration compatibility metadata.
- Runs post-migration verification queries.
- Exits nonzero on drift, partial application, incompatible version, or failed validation.

Runtime processes use DML-only database credentials and never call Ent schema creation. Casbin's Ent adapter is initialized without runtime DDL.

## 3. Single-host production topology

```text
Internet
   |
TLS reverse proxy
   |-- /        -> Next.js production server
   |-- /api/*   -> judgify api
   `-- /ws      -> judgify api WebSocket

Private host network
   |-- PostgreSQL + durable volume
   |-- judgify judge-worker
   |-- rootless Docker + delegated cgroup v2 sandbox runtime
   `-- backup agent -> encrypted off-host destination
```

Operational rules:

- Only TLS ports are public. PostgreSQL, worker metrics, and Docker are private.
- API, Next.js, worker, PostgreSQL, and backup tasks run under distinct identities.
- The API identity cannot invoke Docker.
- Production worker readiness requires rootless Docker with delegated cgroup v2 enforcement for the worker identity. A rootful or merely “most restrictive available” runtime is development-only and cannot satisfy production readiness; membership in a host root-equivalent Docker group is forbidden for the production worker.
- Sandbox containers use no network, non-root user, read-only root filesystem, bounded executable tmpfs, PID/CPU/memory/file/output limits, dropped capabilities, `no-new-privileges`, and an approved seccomp/AppArmor profile.
- Sandbox images are pinned by digest. `latest` is forbidden in release configuration.
- PostgreSQL data, logs needed for recovery, and backup staging have explicit filesystem quotas and monitoring.
- Local Forge files, caches, and WebSocket state are accelerators and are not part of disaster recovery.

## 4. Application lifecycle

### 4.1 Shared lifecycle primitive

`go-common/pkg/common/lifecycle` provides the reusable ownership model:

```go
type Component interface {
    Name() string
    Run(context.Context) error
    Shutdown(context.Context) error
}
```

Executable `main` creates the sole `signal.NotifyContext` and passes that parent context to the lifecycle runner. The reusable library never imports or captures operating-system signals. The implementation establishes:

- One caller-owned cancellation root.
- Deterministic component registration and start ordering.
- Error propagation: an unexpected critical component exit cancels the application.
- Reverse-order shutdown with per-component and total deadlines.
- Panic containment at component boundaries with stack logging and process failure for corrupted critical state.
- No hidden `context.Background()` loops or goroutines without an owner.
- No component catches OS signals independently.
- No library code calls `Fatal`, `os.Exit`, or bypasses deferred cleanup.

### 4.2 API start order

1. Load, normalize, and validate typed configuration.
2. Initialize structured logger and build metadata.
3. Connect PostgreSQL and verify server version, migration head, and required extensions.
4. Load authorization policy revision and construct the initial enforcer.
5. Construct repositories, application services, outbox consumers, health registry, HTTP router, and WebSocket hub through explicit dependencies.
6. Reconcile overdue contest lifecycle commands and the API-owned outbox/notification/security-email work. Judge leases are recovered only by `judgify judge-worker`.
7. Start dispatchers/schedulers.
8. Start HTTP listener with readiness still false.
9. Mark ready after every critical dependency and policy revision is confirmed.

### 4.3 API shutdown order

1. Mark not ready and stop accepting new connections.
2. Stop new WebSocket upgrades; notify/close existing sockets with code `1001`.
3. Stop claiming outbox, notification-campaign operations, and security-email work and drain in-flight bounded batches.
4. Gracefully drain HTTP requests within the request deadline.
5. Stop contest scheduler and background components.
6. Flush metrics/log buffers with a deadline.
7. Close database and remaining resources.

### 4.4 Worker lifecycle

Startup loads the release's explicit capability tuple allow-list, verifies database schema, sandbox image digests, Docker capability, free disk/tmpfs, and performs one probe/smoke path for each tuple before readiness. The worker registers its boot claims-off during probing; platform activation is an audited, later operator action, never an input to decide which tuple to probe. Shutdown stops new claims, keeps heartbeating in-flight leases, waits for bounded execution, persists final results when possible, releases leases it cannot finish, removes disposable containers, and closes the database. A hard deadline terminates remaining containers; expired leases make jobs reclaimable.

## 5. Configuration and secrets

Configuration is typed, versioned, validated before side effects, and sourced from config files plus explicit environment overrides. Validation includes:

- Required endpoints, credentials, key files, and allowed origins.
- Positive timeouts, queue limits, pool sizes, and retention intervals.
- HTTP timeout ordering and maximum body/header sizes.
- Database TLS policy and connection budget shared by API, worker, migration, and operations.
- Access/refresh token issuer, audience, lifetimes, key IDs, and cookie policy.
- Sandbox digest, limits, language runtimes, and output caps.
- Backup destination, encryption, retention, and freshness thresholds.

Startup errors name the invalid field without printing secret values. Logs redact authorization headers, cookies, tokens, password/reset/OAuth codes, private keys, database credentials, contestant source, and hidden testcase data.

Secrets are mounted/read from restricted files or the host secret mechanism, not committed YAML. JWT signing supports active and previous key IDs for rotation. Key, OAuth, email, database, and backup credentials have documented rotation procedures.

## 6. HTTP and connection hardening

The HTTP server configures:

- Read-header, read, write, idle, and graceful shutdown timeouts.
- Maximum headers and request body limits by endpoint class.
- Exact trusted proxy configuration.
- Exact CORS origin allowlist and correct credential behavior.
- Correlation middleware, recovery, security headers, CSRF, authentication, authorization, and route-scoped rate limits.
- Outbound dependency timeouts, idempotency-aware retry, bounded concurrency, and circuit breakers around dependencies rather than around the entire inbound API.

Production responses use an enforcing, integration-tested header policy:

- TLS responses set `Strict-Transport-Security: max-age=31536000; includeSubDomains` after all covered hosts are HTTPS-only, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, and `X-Frame-Options: DENY` as legacy defense.
- `Permissions-Policy` denies camera, microphone, geolocation, payment, USB, and other unused capabilities; any later exception is route-scoped and reviewed.
- CSP sets `default-src 'self'`, `base-uri 'none'`, `object-src 'none'`, `frame-ancestors 'none'`, `form-action 'self'`, `script-src`/`style-src` to self plus per-response Next.js nonces or reviewed immutable hashes, `connect-src` to the exact same-origin HTTPS API and WSS `/ws`, `img-src` to self/data plus the content-render policy's closed HTTPS image-origin allowlist, and `font-src 'self' data:`. Production forbids `unsafe-eval`, unrestricted `unsafe-inline`, wildcard origins, and arbitrary frames/embeds.
- The nonce is generated per response and propagated through the Next.js render path without entering logs or caches. Shared `api/internal/platform/contentrender` HTML contains no scripts, event attributes, inline style, unsafe URL, or node that violates this CSP.

Header/CSP tests cover normal, redirect, error, auth, admin, rendered-content, and WebSocket-connect pages in production build mode. Policy changes deploy report-only first with bounded reports, then become enforcing only after clean critical journeys.

The WebSocket server applies the limits, origin policy, authentication, quotas, heartbeat, backpressure, and shutdown semantics in the realtime design. HTTP and WebSocket share the same session and authorization authority.

## 7. Database and migration policy

### 7.1 Migration discipline

- Atlas/Ent migration files and checksum are committed and immutable after release.
- A clean PostgreSQL instance must migrate from zero to head in CI.
- A representative previous production schema must upgrade to head in CI.
- Schema drift against generated Ent expectations fails CI and readiness.
- Destructive/locking changes include measured query/lock impact and a rollback-safe rollout.
- Seed/reference data uses idempotent versioned migrations or explicit seed commands, not application startup.

### 7.2 Expand–migrate–contract

Breaking changes use separate releases:

1. **Expand:** add nullable/new tables/indexes and backward-compatible write paths.
2. **Migrate:** backfill in bounded resumable batches with progress and verification.
3. **Switch:** deploy readers/writers using the new representation and observe.
4. **Contract:** remove legacy fields/paths only after no compatible release needs them.

An application release declares the migration range it supports. The API refuses readiness when the schema is older than its minimum or newer than its tested maximum.

### 7.3 UUID convergence and legacy migration history

UUID is the canonical identifier target across Ent schemas, PostgreSQL primary/foreign keys, Go/OpenAPI contracts, frontend route/types, and WebSocket topics. The existing committed migration history contains bigint-era tables and cannot be assumed compatible with the generated UUID model.

Before choosing a path, release preflight inventories every migration/checksum, table/column type, sequence, foreign key, row count, and configured/deployed database; captures and verifies a restorable backup when any database exists; and records whether any environment contains authoritative user or production-like data. Ambiguous ownership or unknown deployment history blocks release and requires explicit user/operator resolution.

Only when inventory proves that no authoritative deployment/data exists and the user explicitly approves may the unreleased migration history be replaced with one generated UUID baseline. The replacement regenerates the Atlas checksum and must pass empty-database zero-to-head, Ent schema-diff, seed, and representative query tests. A released migration is never rewritten under this exception.

If any authoritative data exists, destructive reset/history replacement is forbidden. Migration follows expand–migrate–switch–contract:

1. Add UUID shadow columns/tables and a durable mapping table with unique `(entity, legacy_id)` and unique `(entity, uuid)`. Generate a UUIDv7 exactly once with `INSERT ... ON CONFLICT DO NOTHING`, then read/reuse the persisted row on every retry; public UUIDs are not derivable from enumerable legacy integers. New rows also use UUIDv7. Back up and checksum the complete mapping before any foreign-key switch.
2. Deploy backward-compatible dual writes and reads that understand both representations without exposing bigint through new public contracts.
3. Backfill in bounded foreign-key order, parent before child, with resumable cursors, lock/statement limits, and explicit orphan handling.
4. Verify row counts, uniqueness, non-null coverage, mapping checksums, referential integrity, and domain invariants before switching reads/writes and UUID constraints.
5. Retain bigint columns/tables and compatibility code throughout the measured rollback window. Remove them only in a later contract migration after N/N-1 rollback and backup/restore rehearsal succeed.

Application startup and readiness never call Ent/Atlas auto-migration. Only `judgify migrate` running an approved, checksum-verified plan may change schema.

### 7.4 Connection and query operations

- Connection pools are budgeted across all processes and tuned from load evidence, not copied defaults.
- Slow-query thresholds and query-shape metrics identify endpoint/module without logging values.
- User-provided filters and sorts use endpoint allowlists.
- Growing feeds use cursor pagination and supporting composite indexes.
- Long backfills, cleanup, and retention run in bounded batches with statement/lock timeouts.
- Database transactions never include Docker, network email, WebSocket writes, or unbounded computation.

## 8. Continuous integration

### 8.1 Reproducibility

- Go version is identical across go-common modules, Judgify modules, CI, and release builder; the target is the repository-declared Go 1.26.5 toolchain.
- Judgify consumes a tagged go-common version. A local `go.work` may aid development, but release `go.mod` contains no `replace ../../go-common`.
- Dependency lock files, sandbox Dockerfiles, and image digests are committed.
- Build commands run in a clean checkout without developer-global state.
- Go/API, Next.js, and every sandbox image produce SPDX or CycloneDX SBOMs plus signed build-provenance attestations tied to source commit, dependency locks, builder identity, and immutable artifact digest. Release/deploy verifies signatures and provenance before activation.

### 8.2 Go gates

Every pull request runs:

- Formatting and generated-code consistency.
- `go vet` and configured `golangci-lint` with zero accepted production errors.
- Unit tests for all stable packages.
- Race tests for lifecycle, auth runtime, outbox/job claim, scheduler, WebSocket hub, and concurrency primitives.
- Integration tests against an isolated PostgreSQL service using explicit integration selection; ordinary unit tests never start Docker implicitly.
- Migration zero-to-head, previous-to-head, and schema drift checks.
- HTTP/WebSocket contract tests.
- Secret scanning covers tracked history, generated artifacts, container build contexts, and release bundles; test fixtures use unmistakably non-production values.
- Dependency, SAST, license, SBOM, and container/image scans enforce the documented severity/exception policy. Exceptions are owner/expiry/audit bound, never a permanent “ignore all”.
- Artifact signature and build-provenance verification.

Coverage release floors are:

- Judge runner, scheduler, job state machine, scoring, auth session rotation, and authorization policy mutation: at least 90% meaningful branch coverage.
- Other core application services and stable go-common packages: at least 80%.
- Generated Ent code, trivial DTOs, and external adapters are excluded only through documented rules.

### 8.3 Frontend gates

- TypeScript type check and ESLint.
- Generated API/event contract is current.
- Production Next.js build.
- Unit/component tests for shared primitives and critical state reducers.
- End-to-end auth, submission, contest, notification, and admin workflows.
- Accessibility checks and keyboard smoke tests.
- Light/dark visual regression for representative pages.
- Route bundle and Web Vitals budgets.

### 8.4 Sandbox gates

Each sandbox image passes a golden corpus covering:

- Accepted, wrong answer, compile error, runtime error, time limit, memory limit, and output limit.
- Nonzero exits, signals, fork bomb, background daemon, filesystem abuse, oversized files/output, marker-like output, and infinite loops.
- Network denial and read-only filesystem guarantees.
- All supported language/compiler versions and deterministic image provenance.

## 9. Structured logging

### 9.1 Levels

- **DEBUG:** development diagnostics, lease decisions, sampled protocol detail; disabled or sampled in production.
- **INFO:** process lifecycle, successful high-value commands, contest transitions, judge terminal summaries, deployment/build identity.
- **WARN:** degraded but recovered behavior, retries, client abuse limits, stale projections, nearing capacity.
- **ERROR:** request/component failure requiring investigation, exhausted retries, dead jobs, failed persistence, broken invariants.

Routine 4xx validation/authorization outcomes are not logged as errors. Panics at protected boundaries are errors with stack traces. Application components return terminal errors to `main`; libraries do not call `Fatal`.

### 9.2 Generic context enrichment

`logger.FromContext(ctx)` in go-common automatically enriches only generic process metadata and the validated correlation ID:

```text
service, process_role, environment, version, instance_id
cid
```

HTTP adapters add method and route-template fields; generic job/event adapters may add bounded `job_id`, `event_id`, attempt, and consumer fields. Judgify uses `logger.WithFields` for `user_id`, `session_id`, `submission_id`, `problem_id`, `contest_id`, and other domain identifiers. go-common never hardcodes those Judgify concepts. Lease tokens and their hashes are secrets and are never logged at any level.

Logs use route templates, error classes, and stable enum values. They never use source code, testcase input/output, token, raw query, arbitrary user text, full URL query, or high-volume payload as fields. Correlation IDs received from clients are length/character validated; invalid IDs are replaced with UUIDv7 values.

### 9.3 Log shape and sampling

Production logs are JSON to stdout/stderr with UTC timestamp, level, message, fields, error class, and stack only when appropriate. Repeated retry/connection noise is sampled while first occurrence, state transition, and final failure are retained. Security and immutable domain audit logs are not probabilistically sampled.

## 10. Metrics and health

### 10.1 Metrics

Metrics avoid user, submission, job, event, or correlation IDs as labels.

Core metric families include:

- HTTP request count/duration/in-flight by method, route template, status class.
- WebSocket active connections, upgrades, subscriptions, messages, reconnects, drops, coalesces, stable protocol/quota errors, and close reason class by bounded anonymous/authenticated principal class where useful.
- Realtime committed-outbox-to-send latency by bounded event family/outcome, replay/resync requests, cursor gaps, authorization closes, and session-revocation commit-to-socket-close latency.
- Database pool usage/wait, transaction duration, errors, and slow queries by operation class.
- Judge job depth, oldest-ready age, claim latency, lease expiry, retry/dead counts, completion duration, verdict class.
- Sandbox acquire/start/cleanup duration, active capacity, failures, OOM, timeout, output-limit events.
- Outbox ready age/depth, publish latency, retry/dead count; inbox duplicate count.
- Contest lifecycle lag, scoring latency/conflicts, freeze projection lag, rating job state.
- Notification creation/delivery/read latency and unread projection lag; campaign operation age, recipient pending/created/skipped/failed counts, batch duration, resume, and cancellation by bounded outcome/audience class.
- Auth login outcome class, password-login issuance recovery, refresh rotation/reuse/recovery, active/revoked sessions, and authorization decision/error class.
- Security-email ready/leased depth, oldest-ready age, send duration, request-to-sent latency, attempts by kind/outcome, lease recovery, sent/dead/cancelled totals, and ciphertext-erasure failures. Labels never contain recipient, reset token, delivery ID, or provider-message ID.
- Judge-worker current-boot heartbeat freshness, claims-enabled/ready state, normalized exact-capability rows and probe outcomes, activated-tuple drift by bounded component class, judge-admission degraded/rejected state, and exact claim rejection by unsupported tuple component.
- Process CPU, RSS, goroutines, file descriptors, disk space/inodes, and build info.

### 10.2 Health endpoints

- `/health/live` verifies only that the process and critical loops are responsive. It does not fail because PostgreSQL is temporarily unavailable.
- `/health/ready` verifies PostgreSQL, compatible migration head, loaded authorization revision, required dispatchers, activated judge capability/admission safety, and process-specific dependencies. A transient absence of fresh workers is reported as degraded and alerts but does not globally fail unrelated API readiness until admission safety caps require judge submissions to close.
- `/metrics` is private or authenticated.
- Worker health is bound to a private interface and reports current boot heartbeat freshness, claims-enabled gate, database lease capability, exact normalized capability-tuple count/checksums, tuple-specific probe/smoke status, capacity, and drain state.

Dependency checks have short timeouts and cannot exhaust the resource they measure. Readiness goes false before drain and on known incompatible policy/schema state.

## 11. Service objectives and alert thresholds

For the single-host launch phase:

- Monthly API availability objective: 99.5%, excluding announced maintenance; host-level HA requires the later multi-host phase.
- Healthy-host API target: 99.9% successful non-user-error responses.
- Public/read API p95 at agreed load: at most 300 ms; command API p95 excluding judge execution: at most 500 ms.
- Durable submission acknowledgement: zero acknowledged submissions lost.
- Under normal queue depth, judge-job claim p95: at most 2 seconds.
- Submission terminal commit to WebSocket event p95: at most 1 second.
- Notification commit to connected-client event p95: at most 2 seconds.
- Session/account revocation commit to local socket close p95: at most 2 seconds; crash recovery remains bounded by durable invalidation and the 30-second session-cache limit.
- Password-reset request commit to successful provider handoff p95 under healthy-provider conditions: at most 60 seconds; dead deliveries or sustained oldest-ready age above 60 seconds alert.
- Outbox oldest-ready age during steady state: below 5 seconds.
- Authorization policy DB revision to runtime application: below 5 seconds, with fail-closed readiness when stale.
- Backup recovery point objective: at most 5 minutes.
- Full service recovery time objective from validated off-host backup: at most 60 minutes on prepared replacement capacity.

Alerts use sustained windows and burn-rate logic where applicable. Immediate pages are reserved for data-loss risk, migration incompatibility, database unavailable, queue lease collapse, sustained absence of a compatible worker approaching admission/data-safety limits, sandbox escape indicators, signing-key compromise, backup freshness breach, or disk exhaustion. Security-email dead/ciphertext-erasure failures, sustained delivery age, realtime cursor-gap/resync storms, and revocation-close latency have actionable warning/page thresholds without secret-bearing labels.

## 12. Backup and disaster recovery

### 12.1 Backup policy

- Continuous WAL archival or an equivalent mechanism keeps worst-case committed-data loss within 5 minutes.
- Encrypted daily base backups are copied off the production host.
- Retention target: 7 daily, 5 weekly, and 12 monthly recovery points.
- Backup encryption keys are stored separately from backup data and production database credentials.
- Backup reports include start/end time, bytes, WAL position, checksum, destination, and verification result.
- A stale or failed backup makes operations readiness degraded and triggers an alert.

Hidden tests, source code, account data, audit logs, sessions, policies, outbox/inbox, and judge history are all protected as sensitive backup data. Local cache, active socket state, disposable sandbox contents, and Forge acceleration data are excluded.

### 12.2 Restore verification

At least monthly, automation restores the latest backup into an isolated database, applies checksum/integrity checks, runs migration compatibility and representative domain queries, and records measured RPO/RTO. Quarterly, an operator performs a full replacement-host drill including API, worker, Next.js, sandbox images, keys, DNS/TLS routing, and post-restore reconciliation.

The restore runbook identifies the canonical backup, prevents split-brain writes, rotates credentials exposed during an incident, rebuilds disposable projections where safe, reclaims expired jobs, and validates submission/outbox/inbox invariants before reopening traffic.

## 13. Capacity and performance verification

The agreed initial model is approximately 10,000 accounts, 1,000 concurrent users, and bursts of 50 submission requests per second on one machine.

Release load scenarios include:

1. 1,000 WebSocket connections using the agreed authenticated/anonymous mix, closed public/private topic subscriptions, reconnect/principal-upgrade churn, per-IP anonymous abuse/reconnect storms, subscription-quota rejection, and slow-client cases. Private-topic denial and present-invalid-credential rejection are asserted; anonymous quotas cannot starve authenticated control paths or consume unbounded global handler/mailbox capacity.
2. 250 mixed HTTP requests per second across catalog, profile, contest, submission status, notification, and admin-safe reads.
3. A reference burst of 50 valid, in-quota submission commands per second for 10 seconds. The API must durably acknowledge all 500 commands, lose none, and keep memory bounded; completion latency is reported separately from acceptance latency. Stable admission rejection is permitted only outside this declared capacity envelope or for commands that violate quotas/validation.
4. Fair-scheduler contention between contest and regular lanes with per-user concurrency caps and aging.
5. Outbox/replay burst after a temporary dispatcher outage.
6. Four-hour soak with stable goroutine count, file descriptors, RSS trend, disk use, database connections, and queue age.

Judge throughput is measured per language and workload distribution. Worker count is selected from CPU/memory/sandbox evidence; it is not increased beyond host capacity merely to hide queueing. Admission control returns stable retry information before the host enters memory/disk collapse.

## 14. Crash and resilience verification

Automated fault tests terminate processes at each boundary:

- Before and after submission/job/outbox transaction commit.
- After job claim, during compile, during testcase execution, and before terminal finalize.
- After terminal finalize but before outbox dispatch.
- During notification consumer/campaign-recipient transactions and cultivation consumer transaction, including crash after a campaign batch commits but before its operation checkpoint response.
- During contest scoring, unfreeze, rejudge, and rating jobs.
- During authorization DB commit and runtime policy swap.
- During API/worker graceful drain and forced deadline.
- During PostgreSQL reconnect and temporary disk pressure.

Expected results:

- No acknowledged command disappears.
- Leases expire and jobs are reclaimable without duplicate terminal effects.
- Inbox/unique constraints prevent duplicate rewards, scores, ratings, and notifications.
- Runtime authorization never silently continues with a known stale revision.
- WebSocket clients replay or resync; the hub remains bounded.
- Recovery does not require editing database rows by hand.

## 15. Rollout and rollback

### 15.1 Release order

1. Produce immutable Go, Next.js, sandbox, contract, and migration artifacts; sign/record digests.
2. Verify off-host backup freshness and perform preflight capacity/config checks.
3. Apply backward-compatible expand migrations with `judgify migrate`.
4. Start worker N with claims disabled. It writes separate normalized capability rows for the exact N and N-1 complete tuples and must pass schema, decoder fixtures, sandbox probes, and golden smoke checks for each advertised tuple without leasing a job.
5. Close the old N-1 worker's claim gate and wait for its leases to drain or become safely releasable.
6. Enable worker N claims, require its fresh ready instance/boot row plus the exact passing N-1 capability row, and prove it can claim/finalize an N-1 queued fixture before any API switch.
7. Start API N on an alternate local port. New-version activation/emission stays false until the fresh claims-enabled worker exposes the exactly equal N tuple and its tuple-specific smoke evidence; persist that exact `activated_judge_capabilities` row before emission. Runtime worker loss after activation follows the degraded bounded-admission policy rather than globally failing unrelated API routes.
8. Switch reverse-proxy API/WebSocket traffic atomically and drain API N-1.
9. Activate the new Next.js build atomically and smoke critical user journeys.
10. Observe SLO/error/queue/outbox/security/email/realtime dashboards for the defined stabilization window.
11. Run contract/backfill verification; contract migrations occur in a later release.

Dual API slots on one host reduce process deployment interruption but do not provide host HA. Capacity checks ensure the overlap window cannot starve PostgreSQL or the worker. The release compatibility manifest names and proves the exact N and N-1 capability tuples accepted by worker N. API N either emits a complete tuple also accepted by worker N-1 during the rollback window or keeps the new tuple feature-gated until rollback no longer depends on N-1; ambiguity fails preflight.

### 15.2 Rollback rules

- Application rollback is permitted while the expanded schema remains backward compatible.
- The proxy switches back to the ready previous API, the current process drains, and the previous Next.js build is reactivated.
- Before API switch, rollback disables worker N claims, drains it, and may re-enable worker N-1. After API N has emitted jobs, API rolls back first; worker N remains for any N-only queued/in-flight jobs until they drain unless the compatibility manifest proves worker N-1 can claim them. Worker rollback waits for leases or lets them expire and never force-commits unknown work.
- Database down migrations are not used as an automatic rollback for destructive changes. Restore is reserved for verified data corruption/loss scenarios.
- A release is automatically halted or rolled back on migration mismatch, readiness failure, sustained 5xx/SLO burn, authorization revision failure, growing outbox/job lease failure, sandbox security regression, or data invariant violation.

## 16. Operational runbooks

Versioned runbooks cover:

- PostgreSQL unavailable/saturated and migration mismatch.
- Disk/inode pressure and WAL/archive backlog.
- Judge queue age, expired leases, dead jobs, and sandbox unavailable.
- Outbox backlog and duplicate/dead consumer events.
- Contest scoring/freeze/rating incident and emergency end/cancel.
- Authorization policy lockout and signing-key compromise.
- Session abuse, account compromise, and credential rotation.
- WebSocket connection storm and slow-client pressure.
- Backup failure, restore drill, and host replacement.
- Release rollback and forward-fix decision.

Each runbook specifies symptoms, dashboards/queries, safe containment, commands with expected output, escalation boundary, recovery validation, and required audit/incident record. Commands that mutate product state use supported admin/operations commands rather than ad hoc SQL whenever possible.

## 17. Release acceptance checklist

A production release is accepted only with evidence that:

- Source, generated code, migrations, contracts, and dependency locks are clean and reproducible.
- Go and frontend CI gates pass at the defined coverage and quality floors.
- Clean and upgrade migrations pass; schema checksum matches.
- UUID convergence preflight records the approved empty/unreleased-baseline or authoritative-data expand path; zero-to-head/upgrade, mapping checksum, FK/order, rollback-window, and schema-drift evidence match that decision.
- Golden sandbox security/correctness corpus passes for every enabled language image.
- SBOMs, secret/dependency/container scans, signed provenance, and artifact/image signature verification pass for the exact deployed digests.
- Load, soak, backpressure, and crash tests meet budgets with no acknowledged data loss.
- Latest off-host backup is verified and the last restore drill is within policy.
- API/worker readiness, dashboards, alerts, and runbooks match the release fields/metric names.
- Worker N/N-1 claims-disabled tuple-specific smoke, durable fresh current-boot heartbeat, exact normalized-tuple claim join, prevention of untested Cartesian capabilities, claims handoff before API switch, stale-heartbeat rejection, exact activated-capability persistence, no-capability intake rejection, post-activation degraded bounded admission at every depth/age/disk threshold, and rollback compatibility gates pass.
- In-app campaign dry-run/hashed-confirmation/schedule/batch/resume/cancel crash tests, including commit/lost-response recovery and transient-pending versus terminal-failed recipients, plus low-cardinality operation metrics prove bounded work, no duplicate recipients, honest partial-cancel/failure state, nonce redaction, and no email/push side effect.
- Durable reset-email and committed-realtime/revocation metric families emit expected low-cardinality series and their alert tests fire without secret-bearing labels.
- The authenticated/anonymous WebSocket mix passes public-topic replay, private-topic denial, invalid-credential no-downgrade, per-IP reconnect-abuse, quota fairness, and bounded `principal_class` metric-series gates.
- Light/dark/system, accessibility, and critical user/admin journeys pass on the release build.
- Rollout and rollback artifacts are available and compatible with the expanded schema.
- No release dependency uses a local path replacement, mutable sandbox tag, committed secret, runtime auto-migration, or unaudited critical admin mutation.
