# Judgify Production Operations and Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the completed product as reproducible API/worker/migrate roles on one host with observable lifecycle, hardened ingress, encrypted off-host recovery, and retained evidence for load, crash, rollout, and rollback.

**Architecture:** One Go artifact exposes explicit roles. A reverse proxy terminates TLS and forwards to the API; PostgreSQL and worker are private. API and worker use DML-only credentials and lifecycle-managed components. Schema migration, backup, restore, health, metrics, deployment switching, and rollback are explicit commands/runbooks rather than startup side effects.

**Tech Stack:** Go 1.26.5, PostgreSQL, rootless Docker+cgroup v2, Docker Compose/systemd-compatible process definitions, TLS reverse proxy, GitHub Actions, go-common lifecycle/health/logger, Next.js standalone production output, and shell verification scripts.

## Global Constraints

- Initial topology is one physical host and is not advertised as host-level HA.
- One Go binary supports `api`, `judge-worker`, `migrate`; `all` is local development only.
- Only migrate role has DDL privileges.
- API never receives Docker socket access.
- Worker starts claims-disabled and enables only exact smoke-tested capability rows.
- Configuration validates before dependency initialization and contains no checked-in production secret.
- Production logs JSON to stdout at `INFO`; component-scoped debug is authenticated, audited, and time-bounded.
- Metrics never use user/submission/job/event/CID as labels.
- Rootless Docker+cgroup v2 is a required production gate.
- Expand/backfill/switch/verify/contract preserves binary rollback.
- Backups are encrypted and stored off-host; restore is rehearsed into a blank environment.
- RPO ≤ 5 minutes and RTO ≤ 60 minutes.
- No commits or production deployment without explicit later user authorization.

---

## Required Execution Order

The numeric task ID is authoritative: **1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11**. Task sections are grouped in this document for runtime and release review; an implementer must jump to and finish the next numeric ID before starting a later one.

### Task 1: Build One Explicit `judgify` Command with Lifecycle-Owned Roles

**Files:**
- Modify: `api/cmd/judgify/main.go`
- Create: `api/cmd/judgify/api.go`
- Modify: `api/cmd/judgify/judge_worker.go`
- Create: `api/cmd/judgify/migrate.go`
- Create: `api/cmd/judgify/all.go`
- Create: `api/cmd/judgify/main_test.go`
- Modify: `api/cmd/server/main.go`
- Modify: `api/internal/infrastructure/initialized.go`
- Modify: `api/internal/infrastructure/run.go`
- Modify: `api/internal/infrastructure/server.go`
- Modify: `api/internal/di/container.go`

**Interfaces:**

```go
type Role string

const (
    RoleAPI Role = "api"
    RoleJudgeWorker Role = "judge-worker"
    RoleMigrate Role = "migrate"
    RoleAll Role = "all"
)

func Run(ctx context.Context, role Role, config Config) error
```

- [ ] **Step 1: Write role/import/lifecycle tests**

Assert unknown role exits non-zero, `all` rejects prod, signal context exists only in main, components return errors instead of Fatal, API dependency graph has no Docker import/socket, worker has no HTTP product router, migrate has no long-running component, and shutdown is reverse/bounded/idempotent.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./cmd/judgify ./internal/infrastructure -count=1`

Expected: FAIL because roles are not explicit and current bootstrap owns long-running work incorrectly.

- [ ] **Step 3: Implement command and builders**

`main` parses role/config, creates `signal.NotifyContext`, calls `Run`, logs one terminal error, flushes logger, and exits. Role builders construct only their needed components and register them with go-common lifecycle. No module starts a goroutine during DI construction.

- [ ] **Step 4: Preserve compatibility entry point**

`cmd/server/main.go` delegates to `judgify api` behavior with a deprecation notice for local scripts; remove it only after deployment consumers switch.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./cmd/judgify ./internal/infrastructure -count=100
go test -race ./cmd/judgify ./internal/infrastructure -count=10
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 6: Make CI Reproducible and Supply-Chain Aware

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `.github/workflows/integration.yml`
- Create: `.github/workflows/release-evidence.yml`
- Modify: `Makefile`
- Create: `scripts/verify/go.sh`
- Create: `scripts/verify/frontend.sh`
- Create: `scripts/verify/supply_chain.sh`
- Create: `scripts/verify/contracts.sh`

**Interfaces:**
- Produces isolated jobs for Go, frontend, migration, sandbox, integration, security, and release evidence.

- [ ] **Step 1: Write workflow/static script tests**

Assert pinned action versions/permissions, no broad write token in PR jobs, deterministic Go/npm installs, cache keys include lockfiles/tool versions, no hidden local `replace` in release, generated/API/schema drift gates, explicit integration tags, artifact retention, concurrency cancellation, and required job dependency graph.

- [ ] **Step 2: Run local script checks**

Run:

```bash
bash scripts/verify/go.sh
bash scripts/verify/frontend.sh
bash scripts/verify/contracts.sh
```

Expected: scripts expose current frontend/contract/migration blockers until all child plans land.

- [ ] **Step 3: Implement Go/frontend gates**

Go: format, vet, lint, test, targeted race, coverage ratchet, integration, fuzz seeds, `govulncheck`, module tidy/drift, generated files. Frontend: clean install, lint, typecheck, coverage, build, browser, bundle/route scans.

- [ ] **Step 4: Implement supply-chain evidence**

Generate SBOM for Go/npm/images, scan dependencies/images/secrets, check licenses, record source/build/tool versions and checksums, sign/provenance only through approved release environment. Release build consumes tagged go-common and has no filesystem `replace`.

- [ ] **Step 5: Verify**

Run:

```bash
make verify
bash scripts/verify/supply_chain.sh
git diff --check
```

Expected after functional plans: every local equivalent exits 0. Do not commit.

### Task 7: Add Structured Metrics, Health, and Time-Bounded Debug Control

**Files:**
- Modify: `api/internal/infrastructure/logger.go`
- Create: `api/internal/platform/metrics/registry.go`
- Create: `api/internal/platform/metrics/http.go`
- Create: `api/internal/platform/metrics/metrics_test.go`
- Create: `api/internal/platform/health/registry.go`
- Create: `api/internal/platform/health/http.go`
- Create: `api/internal/platform/health/health_test.go`
- Create: `api/internal/application/workflow/debuglevel/workflow.go`
- Create: `api/internal/application/workflow/debuglevel/workflow_test.go`
- Modify: `api/internal/infrastructure/router.go`

**Interfaces:**

```go
type DebugLevelCommand struct {
    Component string
    Level string
    Duration time.Duration
    Reason string
    IdempotencyKey uuid.UUID
}
```

- [ ] **Step 1: Write log/metric/health/debug tests**

Assert every log has service/version/environment/component/message/CID where available; secret/high-cardinality fields rejected/redacted; level semantics; production info default; expensive debug lazy; metrics label allowlist; health deadline/concurrency; ready false on DB/schema/authz mismatch/worker capability; debug capability+recent reauth+reason+max duration+automatic revert+audit.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/platform/metrics ./internal/platform/health \
  ./internal/application/workflow/debuglevel -count=1
```

Expected: FAIL because platform adapters/workflow do not exist.

- [ ] **Step 3: Implement low-cardinality metrics**

Expose request count/duration by route template/status class, DB pool/query class, job depth/age/state, worker capacity/lease/finalization, outbox/replay lag, WS connections/drop/close, email/campaign/operation counts, contest projection/rating lag, and process runtime. Never label with IDs/CID/user-supplied content.

- [ ] **Step 4: Implement role-aware health**

Liveness proves event loop/process. API readiness requires DB, compatible schema, authz desired=applied, bounded critical component state. Worker readiness additionally requires fresh current boot, claims state, exact passing capability, Docker/cgroup probe, disk thresholds. Static `/ping` remains diagnostic only.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/platform/metrics ./internal/platform/health \
  ./internal/application/workflow/debuglevel -count=100
go test -race ./internal/platform/metrics ./internal/platform/health -count=10
```

Expected: all exit 0. Do not commit.

### Task 8: Encode SLOs, Alerts, Dashboards, and Capacity Thresholds

**Files:**
- Create: `ops/observability/slos.yaml`
- Create: `ops/observability/alerts.yaml`
- Create: `ops/observability/dashboard-api.json`
- Create: `ops/observability/dashboard-judge.json`
- Create: `ops/observability/dashboard-product.json`
- Create: `scripts/verify/observability.sh`
- Create: `scripts/verify/observability_test.sh`

**Interfaces:**
- Produces machine-validated objectives/thresholds tied to emitted metrics.

- [ ] **Step 1: Write observability contract tests**

Validate every alert metric exists, no forbidden label, every SLO has window/target/budget/query/runbook, thresholds are positive/ordered, and dashboard queries use bounded aggregation.

- [ ] **Step 2: Run contract test**

Run: `bash scripts/verify/observability_test.sh`

Expected: FAIL until manifests exist.

- [ ] **Step 3: Define initial objectives**

Include API availability/latency, submission acknowledgement, judge queue age/finalization, outbox/replay lag, WS reconnect/critical close, security email age/dead state, authorization revision mismatch, backup freshness, disk/FD/memory/cgroup pressure, and error budget burn. Thresholds match single-host measured capacity rather than invented multi-node claims.

- [ ] **Step 4: Verify**

Run: `bash scripts/verify/observability.sh`

Expected: manifests validate and every alert links to an existing runbook path. Do not commit.

### Task 9: Implement Encrypted Off-Host Backup and Blank-Environment Restore

**Files:**
- Create: `scripts/backup/postgres.sh`
- Create: `scripts/backup/verify.sh`
- Create: `scripts/restore/postgres.sh`
- Create: `scripts/restore/validate.sh`
- Create: `ops/backup/policy.md`
- Create: `ops/runbooks/database-restore.md`
- Create: `test/operations/backup_restore_test.sh`

**Interfaces:**
- Produces encrypted, checksummed, off-host database backups plus restore validation; test assets/source blobs are included according to their canonical storage policy.

- [ ] **Step 1: Write fixture backup/restore test**

Seed users/sessions/Casbin revisions/problems/submissions/jobs/outbox/replay/contest/rating/cultivation/material/notifications/audit. Backup, destroy test DB, restore blank, validate checksums/counts/invariants, reclaim expired leases, rebuild disposable projections where approved, and prove secrets are not printed.

- [ ] **Step 2: Run fixture test**

Run: `bash test/operations/backup_restore_test.sh`

Expected: FAIL until scripts exist.

- [ ] **Step 3: Implement backup**

Use consistent PostgreSQL backup plus WAL/incremental policy sufficient for five-minute RPO, compress, encrypt with an external key, checksum/sign metadata, upload to configured off-host target, enforce retention, and emit freshness/success metrics. A local file alone is not success.

- [ ] **Step 4: Implement safe restore**

Refuse when writers are active unless explicit disaster flag, restore into blank DB, apply only compatible migrations, rotate incident-exposed credentials, validate authorization desired/applied, submission/job/outbox/inbox/rating/ledger invariants, and keep traffic closed until validation passes.

- [ ] **Step 5: Measure recovery**

Run the full test with production-size fixture and retain start/end evidence. Expected: observed RPO ≤5 minutes and RTO ≤60 minutes. If not, tune process/capacity before accepting.

### Task 10: Automate Load, Soak, Crash, and Disk-Pressure Proof

**Files:**
- Create: `test/load/http.js`
- Create: `test/load/websocket.js`
- Create: `test/load/submissions.js`
- Create: `test/load/scenario.yaml`
- Create: `test/chaos/process.sh`
- Create: `test/chaos/postgres.sh`
- Create: `test/chaos/docker.sh`
- Create: `test/chaos/disk.sh`
- Create: `scripts/verify/resilience.sh`
- Create: `ops/runbooks/capacity-pressure.md`

**Interfaces:**
- Produces machine-readable pass/fail evidence and time-series resource samples.

- [ ] **Step 1: Write scenario assertions**

Encode 250 mixed HTTP requests/s, 1,000 WebSockets with reconnect/slow clients, 500 submissions at 50/s for ten seconds, four-hour soak, process kills at every durable boundary, PostgreSQL/Docker restart, and disk watermark behavior.

- [ ] **Step 2: Implement load generators**

Use bounded test accounts/data and validate business outcomes, not only status codes: no lost job, no duplicate finalization/reward/rating/notification, authorized topics only, replay convergence, latency/error/memory/FD/goroutine budgets.

- [ ] **Step 3: Implement chaos safety guards**

Scripts require explicit test environment marker and target IDs, print planned action, never run against production by default, and restore service before next scenario. Disk test uses a bounded test volume, not host root.

- [ ] **Step 4: Run and retain evidence**

Run: `bash scripts/verify/resilience.sh`

Expected: every threshold/invariant passes; failures produce timestamped artifact paths and leave no test process/container.

### Task 11: Implement N/N-1 Rollout, Rollback, and Operational Runbooks

**Files:**
- Create: `scripts/deploy/release.sh`
- Create: `scripts/deploy/rollback.sh`
- Create: `scripts/deploy/status.sh`
- Create: `scripts/deploy/smoke.sh`
- Create: `ops/runbooks/deploy.md`
- Create: `ops/runbooks/rollback.md`
- Create: `ops/runbooks/authentication-incident.md`
- Create: `ops/runbooks/authorization-stale.md`
- Create: `ops/runbooks/judge-queue.md`
- Create: `ops/runbooks/sandbox-incident.md`
- Create: `ops/runbooks/realtime-lag.md`
- Create: `ops/runbooks/contest-rating.md`
- Create: `ops/runbooks/security-email.md`
- Create: `ops/runbooks/disk-pressure.md`
- Create: `test/operations/rollout_test.sh`

**Interfaces:**
- Produces saved version switch on one host with old/new API ports and separately proved N/N-1 worker tuples.

- [ ] **Step 1: Write rollout-state test**

Assert schema expand first, new worker claims-disabled, exact N and N-1 tuple smoke rows, new worker enable, old worker claim disable/drain, new API secondary port/readiness/exact tuple activation, proxy atomic switch, old API drain, and rollback to old API plus separately proved previous tuple. Stale heartbeat or missing tuple must block switch.

- [ ] **Step 2: Run rollout test**

Run: `bash test/operations/rollout_test.sh`

Expected: FAIL until deploy scripts exist.

- [ ] **Step 3: Implement release and rollback scripts**

Scripts are idempotent, lock deployment, verify build/SBOM/config/migration/backup, apply only expand-compatible migrations, record release state, and stop on any missing evidence. Rollback never rolls database backward; it switches binary/proxy to a schema-compatible prior version and restores previous worker claims only after exact smoke.

- [ ] **Step 4: Write executable runbooks**

Every runbook contains symptoms, dashboard/query, safe containment, commands with expected output, escalation boundary, recovery validation, and incident/audit record. Prefer supported admin/operations commands over ad hoc SQL.

- [ ] **Step 5: Verify complete operations plan**

Run:

```bash
make verify
bash scripts/verify/topology.sh
bash scripts/verify/migrations.sh
bash scripts/verify/observability.sh
bash test/operations/backup_restore_test.sh
bash test/operations/rollout_test.sh
bash scripts/verify/resilience.sh
git diff --check
```

Expected: every command exits 0 with retained evidence. Do not deploy, commit, tag, or push.

## Acceptance Criteria Reference

- [ ] One artifact exposes explicit API/worker/migrate roles and dev-only all role.
- [ ] API cannot access Docker and runtime DB users cannot perform DDL.
- [ ] Configuration/secrets fail closed before dependency initialization.
- [ ] Topology exposes only hardened TLS ingress.
- [ ] CI proves code, migration, contracts, generated output, dependencies, and images.
- [ ] Structured logs/metrics/health follow CID, redaction, and cardinality rules.
- [ ] SLO/alerts link to emitted metrics and executable runbooks.
- [ ] Encrypted off-host backup restores into blank environment within RPO/RTO.
- [ ] Load/soak/crash/disk scenarios retain invariant/resource evidence.
- [ ] N/N-1 rollout and binary rollback pass without database rollback.
- [ ] No deployment, commit, tag, or push was performed.

### Task 2: Validate Configuration and Secret Sources Before Startup

**Files:**
- Modify: `api/pkg/settings/config.go`
- Modify: `api/internal/infrastructure/config.go`
- Modify: `api/config/local.yaml`
- Create: `api/config/example.production.yaml`
- Create: `api/internal/infrastructure/config_test.go`
- Create: `api/internal/infrastructure/secrets.go`
- Create: `api/internal/infrastructure/secrets_test.go`
- Modify: `.gitignore`

**Interfaces:**

```go
type SecretSource interface {
    Read(ctx context.Context, name string) ([]byte, error)
}

func Load(ctx context.Context, path string, source SecretSource) (Config, error)
func (c Config) Validate(role Role) error
```

- [ ] **Step 1: Write config matrix and secret-redaction tests**

Cover required issuer/audience/key IDs, cookie origin/domain, CSRF/HMAC/AEAD/refresh peppers, DB URLs/role separation, API/worker timeouts, queue/lease/capacity, WebSocket bounds, Docker profiles, backup target, invalid env/zero/negative/overflow, dev-only options in prod, unknown keys, file permissions, and error/log redaction.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/infrastructure -run 'Test(Config|Secret)' -count=1`

Expected: FAIL because current config permits incomplete runtime setup.

- [ ] **Step 3: Implement layered configuration**

Load checked-in non-secret defaults → environment-specific file → environment variables/secret files. Production secret values are referenced by names/paths, not present in YAML. Reject unknown config keys and validate the selected role before logger/dependency connection.

- [ ] **Step 4: Verify repository secret hygiene**

Run:

```bash
git grep -n -I -E '(BEGIN .*PRIVATE KEY|client_secret: [^$]|password: [^$]|refresh_pepper: [^$])'
```

Expected: no real secret; examples use explicit placeholder references.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/infrastructure -run 'Test(Config|Secret)' -count=50
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 3: Make Migration an Explicit, Reproducible Role

**Files:**
- Modify: `api/internal/ent/client.go`
- Modify: `api/internal/infrastructure/ent.go`
- Create: `api/internal/ent/migrate/runner.go`
- Create: `api/internal/ent/migrate/runner_test.go`
- Create: `api/internal/ent/migrate/schema_drift_test.go`
- Modify: `api/cmd/judgify/migrate.go`
- Create: `scripts/verify/migrations.sh`
- Create: `scripts/verify/migrations_test.sh`

**Interfaces:**

```go
type MigrationOptions struct {
    DryRun bool
    Target string
    LockTimeout time.Duration
    StatementTimeout time.Duration
}

func Apply(ctx context.Context, db *sql.DB, options MigrationOptions) (Report, error)
```

- [ ] **Step 1: Write migration lock/history/role tests**

Test migration from zero, upgrade from every retained production version, checksum mismatch, concurrent migrator advisory lock, timeout/cancel, dry-run, runtime DML role cannot DDL, adapter startup performs no DDL, schema drift, generated Atlas checksum, and destructive SQL policy.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test -tags=integration ./internal/ent/migrate -count=1
```

Expected: new runner/drift tests fail.

- [ ] **Step 3: Implement migrate-only DDL**

API/worker clients receive only migrated schemas and validate compatible schema version. `migrate` obtains advisory lock, checks migration checksums, applies ordered SQL with bounded timeouts, records result/build identity, and exits. No runtime `Schema.Create` call remains.

- [ ] **Step 4: Implement CI script**

Script starts disposable PostgreSQL only in explicit integration job, applies from zero, loads each upgrade fixture, applies latest, runs Ent schema diff, and fails on unapproved destructive/DDL drift.

- [ ] **Step 5: Verify**

Run:

```bash
bash scripts/verify/migrations_test.sh
cd api
go test -tags=integration ./internal/ent/migrate -count=20
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 4: Define the Hardened Single-Host Runtime Topology

**Files:**
- Modify: `docker-compose.yml`
- Modify: `docker/sandbox/Dockerfile`
- Create: `docker/api/Dockerfile`
- Create: `docker/website/Dockerfile`
- Create: `deploy/compose.production.yml`
- Create: `deploy/env.production.example`
- Create: `deploy/reverse-proxy.conf`
- Create: `deploy/postgres/init-roles.sql`
- Create: `scripts/verify/topology.sh`
- Create: `scripts/verify/topology_test.sh`

**Interfaces:**
- Produces private PostgreSQL, isolated worker, API, Next.js, and TLS proxy services with explicit health/dependency rules.

- [ ] **Step 1: Write static topology tests**

Parse Compose files and assert API has no Docker socket/device/privileged/capability escalation, worker alone has rootless Docker endpoint and bounded resources, PostgreSQL is not publicly exposed, secrets use files/env references, immutable image digests for sandbox profiles, read-only filesystems where possible, tmpfs limits, restart policy, health checks, and separate DML/DDL DB users.

- [ ] **Step 2: Run topology test**

Run: `bash scripts/verify/topology_test.sh`

Expected: FAIL against current topology.

- [ ] **Step 3: Implement multi-stage reproducible images**

Pin builder/runtime bases by digest, build Go with module verification and VCS metadata, run as non-root, and generate Next standalone output. Images contain no source secrets/dev tools. Sandbox images contain only exact runtime/compiler manifest and non-root user.

- [ ] **Step 4: Implement ingress/private network**

TLS proxy exposes 443 only, exact host, security headers, request/body limits, WebSocket upgrade, trusted proxy address, drain/timeout settings. API/PostgreSQL/worker management ports remain private.

- [ ] **Step 5: Verify**

Run:

```bash
bash scripts/verify/topology.sh
docker compose -f deploy/compose.production.yml config
```

Expected: config validates and static security assertions pass. Do not deploy or commit.

### Task 5: Harden HTTP, Proxy Trust, WebSocket, and Graceful Drain

**Files:**
- Modify: `api/internal/infrastructure/router.go`
- Modify: `api/internal/infrastructure/server.go`
- Modify: `api/internal/platform/realtime/gateway.go`
- Create: `api/internal/infrastructure/http_security_test.go`
- Create: `api/internal/infrastructure/shutdown_test.go`
- Modify: `deploy/reverse-proxy.conf`

**Interfaces:**
- Consumes go-common HTTP/WebSocket/lifecycle.
- Produces exact origin/proxy/body/header/deadline/drain behavior.

- [ ] **Step 1: Write ingress and shutdown tests**

Cover exact production origin, disallowed/no CORS headers, wildcard+credentials rejection, proxy spoofing, forwarded IP only from exact proxy, host validation, security headers, body/header/URL limits, request deadline, WebSocket upgrade, readiness off before drain, stop intake, HTTP drain, socket 1001 close, worker claim drain, and bounded terminal cancellation.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/infrastructure -run 'Test(HTTPSecurity|Shutdown)' -count=1`

Expected: FAIL until complete hardening/drain order exists.

- [ ] **Step 3: Implement validated middleware/server order**

Order: recovery → correlation/logger → host/proxy/security headers → body/deadline → CORS/CSRF → session authentication → authorization → handlers. WebSocket uses the same exact origin/trusted peer/session policy at upgrade.

- [ ] **Step 4: Implement role shutdown sequence**

API: readiness false → stop new HTTP/WS → drain HTTP → stop event/security-email/realtime components → close DB/cache/logger. Worker: claims disabled durable+local → drain/cancel leases/sandboxes → heartbeat final state → close DB/logger.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/infrastructure -run 'Test(HTTPSecurity|Shutdown)' -count=100
go test -race ./internal/infrastructure -count=10
```

Expected: all exit 0. Do not commit.

## Final Acceptance Gate

After Task 11, re-evaluate every checkbox in **Acceptance Criteria Reference**. The plan exits only when every topology, migration, observability, backup, resilience, rollout, and rollback item has retained evidence.
