# Judgify Production Execution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the approved single-tenant, single-host Judgify production product and reusable go-common foundation without losing acknowledged work, weakening sandbox boundaries, or replacing the current technology stack.

**Architecture:** Judgify remains a modular monolith built as one Go artifact with separate `api`, `judge-worker`, and `migrate` runtime roles. PostgreSQL is authoritative for domain state, jobs, outbox/inbox, sessions, authorization revisions, audit, and notifications; local caches, wakeups, schedulers, and WebSocket hubs are disposable accelerators. Delivery is divided into dependency-ordered plans whose checkpoints always leave a testable compatibility state.

**Tech Stack:** Go 1.26.5, Gin, Ent, PostgreSQL, Casbin v3 with `github.com/casbin/ent-adapter v1.4.0`, `coder/websocket` hidden by go-common, Docker sandbox, Next.js 16.1.6, React 19.2.3, TypeScript 5, Tailwind CSS 4, Base UI/shadcn, next-themes, and existing Framer Motion.

## Global Constraints

- Deployment is single tenant and one physical host initially.
- Keep the current Go, Gin, Ent, PostgreSQL, Casbin, Docker, Next.js, React, Tailwind, Base UI/shadcn, and Framer Motion stack.
- Do not add Redis, Kafka, SSE, a service mesh, a second frontend framework, or another animation runtime to Judgify production.
- PostgreSQL is the durable source of truth; local queue, cache, hub, and wakeup state may always be discarded.
- API processes never access the Docker socket; only the `judge-worker` role executes contestant code.
- Browser authentication uses same-origin secure cookies and CSRF protection; no production token or profile is stored in `localStorage`.
- Browser realtime uses one native WebSocket connection per tab; no final SSE or polling path remains.
- Product-authored copy is Vietnamese and must come from one typed text catalog.
- Every production route supports light, dark, and system themes.
- Generic correlation is owned by go-common; Judgify adds bounded domain identifiers such as `submission_id`, `contest_id`, and `user_id`.
- Development logging defaults to `DEBUG`; production defaults to `INFO`; every level redacts secrets, source code, hidden tests, policy secrets, and lease tokens.
- All schema changes use `expand -> backfill -> switch -> verify -> contract in a later release`.
- Rootless Docker and cgroup v2 capability probes are mandatory production gates.
- Initial recovery objectives are RPO at most five minutes and RTO at most sixty minutes.
- Preserve the existing dirty Judgify worktree. Do not reset, discard, overwrite, or commit user changes.
- The user explicitly requested no commits. Replace every commit checkpoint with a diff/test review checkpoint.

---

## Approved Design Inputs

- `docs/superpowers/specs/2026-07-16-judgify-production-master-design.md`
- `docs/superpowers/specs/2026-07-16-go-common-production-library-design.md`
- `docs/superpowers/specs/2026-07-16-identity-authentication-authorization-design.md`
- `docs/superpowers/specs/2026-07-16-problem-submission-judge-design.md`
- `docs/superpowers/specs/2026-07-16-realtime-contest-notifications-design.md`
- `docs/superpowers/specs/2026-07-16-cultivation-material-admin-design.md`
- `docs/superpowers/specs/2026-07-16-frontend-design-system-data-design.md`
- `docs/superpowers/specs/2026-07-16-production-operations-release-design.md`

## Baseline Evidence Recorded on 2026-07-18

| Surface | Command | Observed result |
|---|---|---|
| go-common | `cd ../go-common && go test ./...` | Exit 0 |
| Judgify API | `cd api && go test ./...` | Exit 0 |
| Frontend lint | `cd website && npm run lint` | Exit 1: 27 errors and 7 warnings |
| Frontend production build | `cd website && npm run build` | Remained at “Creating an optimized production build ...” for several minutes and was interrupted; no pass claim |

The lint failures include unescaped authored text, synchronous state updates in effects, explicit `any`, the recursive SSE callback, and unused declarations. These are baseline defects and must be eliminated by the frontend foundation tasks rather than suppressed with blanket ESLint disables.

## File and Ownership Map

| Area | Existing directories retained | New directories introduced |
|---|---|---|
| Shared library | `../go-common/pkg` | `../go-common/pkg/correlation`, `../go-common/pkg/security/authentication`, `../go-common/pkg/security/authorization`, `../go-common/pkg/common/lifecycle`, `../go-common/pkg/common/health`, `../go-common/pkg/common/grpc`, `../go-common/pkg/common/websocket` |
| Application kernel | `api/cmd/server`, `api/internal/infrastructure`, `api/internal/di` | `api/cmd/judgify`, `api/internal/platform/audit`, `api/internal/platform/contentrender`, `api/internal/platform/operation`, `api/internal/platform/transaction`, `api/internal/application/query`, `api/internal/application/workflow` |
| Identity | `api/internal/identity`, `api/internal/authctx` | `api/internal/platform/eventing`, `api/internal/platform/transaction`, `api/internal/platform/audit` |
| Problem/submission | `api/internal/problem`, `api/internal/submission`, `api/internal/judge` | `api/internal/platform/contentrender`, `api/internal/application/workflow/admincommand` |
| Realtime/contest | `api/internal/contest`, `api/internal/contest/store/hub.go` | `api/internal/platform/eventing`, `api/internal/platform/realtime`, `api/internal/notification` |
| Cultivation/material/admin | `api/internal/cultivation`, `api/internal/material`, `website/modules/admin` | `api/internal/cultivation/core/service`, `api/internal/material/core/service`, `api/internal/platform/operation`, `api/internal/application/workflow/admincommand` |
| Frontend | `website/app`, `website/modules`, `website/services`, `website/types` | `website/i18n`, `website/lib/api`, `website/lib/realtime`, `website/contexts/session-context.tsx`, `website/contexts/realtime-context.tsx` |
| Operations | `Dockerfile`, `Makefile`, `.github/workflows` | `deploy`, `ops/runbooks`, `scripts/verify`, `ops/evidence` |

Generated Ent files in `api/internal/ent/generate` are regenerated from schema tasks. They are never edited by hand.

## Execution Dependency Graph

```text
Wave 0 baseline + ownership manifest
  ├── go-common security/correlation/lifecycle/HTTP
  │     ├── Judgify process roles and platform transaction
  │     ├── Identity sessions + authorization
  │     └── go-common WebSocket
  ├── frontend contract/theme/text foundation
  │     └── frontend real-data slices
  └── migration inventory
        ├── problem/submission/judge durability
        ├── outbox/realtime/notifications
        ├── contest correctness
        └── cultivation/material/admin

All functional waves
  └── operations proof: migration, crash, load, restore, rollout, rollback
```

## Plan Index and Exit Gates

| Order | Plan | Entry gate | Exit gate |
|---:|---|---|---|
| 0 | This master plan, Tasks 1–3 | Approved specs | Reproducible baseline, dirty-worktree manifest, architecture checks |
| 1 | `2026-07-18-go-common-production-library-implementation-plan.md` | go-common baseline green | Stable shared APIs, race/fuzz/security checks green |
| 2 | `2026-07-18-identity-access-implementation-plan.md` | security/correlation/lifecycle APIs exist | Cookie sessions, recovery, atomic Casbin revisions, admin access API |
| 3 | `2026-07-18-problem-submission-judge-implementation-plan.md` | transaction/lifecycle plus migration inventory | Durable intake, exact capabilities, safe sandbox, transactional finalization |
| 4 | `2026-07-18-cultivation-material-admin-implementation-plan.md` | identity/onboarding, solved-transition, and content-renderer contracts | Durable trait offers, exact rewards, revisions, shared operation plane, audited admin workflows |
| 5 | `2026-07-18-realtime-contest-notifications-implementation-plan.md` | WebSocket primitive, transaction-bound outbox, and shared operation plane | Replayable realtime, persisted notifications, correct contests/rating |
| 6 | `2026-07-18-frontend-product-implementation-plan.md` | API contracts become available by slice | No runtime mocks/SSE/localStorage auth; complete responsive themed product |
| 7 | `2026-07-18-production-operations-release-implementation-plan.md` | functional waves green | Production topology, observability, backup/restore, load/crash/rollback evidence |

Frontend work is interleaved after each backend slice rather than postponed until all backend work finishes. The order column describes contract dependency, not a rule that UI integration waits until the end.

## Spec Coverage Matrix

| Approved design area | Implementing plan/tasks |
|---|---|
| go-common maturity/taxonomy/compatibility | go-common Tasks 1, 14–16 |
| password, JWT, OAuth, authorization snapshot | go-common Tasks 2–5 |
| correlation, logging, HTTP, gRPC | go-common Tasks 6, 8–10 |
| lifecycle and health | go-common Task 7 |
| WebSocket envelope/Hub/ACL/backpressure/drain | go-common Tasks 11–13 |
| cache/database/MQ/Forge/CDC/unsafe/time/IDs | go-common Tasks 14–15 |
| UnitOfWork, audit, transaction-bound outbox | identity Task 1 |
| identity security schema/session cache | identity Tasks 2–3 |
| recoverable login/registration/refresh | identity Tasks 4–5 |
| recovery contact/security email | identity Task 6 |
| OAuth state/callback ticket/finalize | identity Task 7 |
| profile definition/value revisions | identity Task 8 |
| resource/action registry, Casbin revision/invariants | identity Tasks 9–10 |
| cookie/session/recovery/access APIs and migration | identity Tasks 11–12 |
| shared authored-content renderer | problem/judge Task 1 |
| problem/testset/runtime/checker revisions | problem/judge Tasks 2–3 |
| submission/spec/job/attempt/judgement state | problem/judge Task 4 |
| atomic intake and exact worker claim | problem/judge Tasks 5–6 |
| rootless supervisor, hostile staging, fresh cases/checkers | problem/judge Tasks 7–9 |
| finalization/solve transition/rejudge/query/worker switch | problem/judge Tasks 10–12 |
| trait catalog/offers/onboarding | cultivation/material/admin Tasks 1–2 |
| ledger/reward/ranking projection | cultivation/material/admin Tasks 3–4 |
| material revisions/search/views | cultivation/material/admin Tasks 5–6 |
| operation plane/admin queries/workflows/audit | cultivation/material/admin Tasks 7–9 |
| exact legacy allocation and complete admin APIs | cultivation/material/admin Tasks 10–11 |
| outbox dispatch/inbox and serialized replay | realtime/contest/notification Tasks 1–2 |
| WebSocket gateway and persisted notifications | realtime/contest/notification Tasks 3–4 |
| notification campaigns | realtime/contest/notification Task 5 |
| contest lifecycle/content/scoring/freeze/rating | realtime/contest/notification Tasks 6–9 |
| API/event catalog/SSE removal/crash-load proof | realtime/contest/notification Tasks 10–12 |
| frontend baseline/theme/catalog | frontend Tasks 1–3 |
| UUID/API/client/session foundations | frontend Tasks 4–6 |
| one WebSocket provider and shells | frontend Tasks 7–8 |
| public/problem/auth/profile/contest/material/ranking/notification routes | frontend Tasks 9–11 |
| complete admin and accessibility/performance/no-mock gates | frontend Tasks 12–13 |
| roles/config/migrations/topology/HTTP drain | operations Tasks 1–5 |
| CI/supply chain/metrics/health/SLO | operations Tasks 6–8 |
| backup/restore/load/crash/rollout/rollback/runbooks | operations Tasks 9–11 |

Every explicit initial-release exclusion in the specs remains excluded. No plan task silently adds multi-tenancy, Redis/Kafka runtime coordination, microservices, interactive judging, plagiarism detection, MFA/passkeys, or another frontend/runtime framework.

### Task 1: Freeze a Non-Destructive Baseline

**Files:**
- Create: `docs/superpowers/baseline/2026-07-18-worktree-status.txt`
- Create: `docs/superpowers/baseline/2026-07-18-test-baseline.md`
- Create: `scripts/verify/baseline.sh`
- Create: `scripts/verify/baseline_test.sh`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: current dirty Judgify worktree and clean go-common worktree.
- Produces: a reviewable file manifest and one read-only baseline command used by every later plan.

- [x] **Step 1: Write the baseline-script contract test**

Create `scripts/verify/baseline_test.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test -x "$repo_root/scripts/verify/baseline.sh"
"$repo_root/scripts/verify/baseline.sh" --check-tools
```

- [x] **Step 2: Run the test and verify the script is missing**

Run: `bash scripts/verify/baseline_test.sh`

Expected: non-zero with `scripts/verify/baseline.sh` missing or not executable.

- [x] **Step 3: Implement the bounded baseline runner**

Create `scripts/verify/baseline.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

check_tools() {
  command -v git >/dev/null
  command -v go >/dev/null
  command -v npm >/dev/null
}

if [[ "${1:-}" == "--check-tools" ]]; then
  check_tools
  exit 0
fi

check_tools
git -C "$repo_root" status --short
git -C "$repo_root/../go-common" status --short
(cd "$repo_root/../go-common" && go test ./...)
(cd "$repo_root/api" && go test ./...)
(cd "$repo_root/website" && npm run lint)
(cd "$repo_root/website" && npm run build)
```

Add only generated test/build outputs—not source, specs, or baseline evidence—to `.gitignore`. Mark both scripts executable through the filesystem mode.

- [x] **Step 4: Capture the current state without overwriting user files**

Run:

```bash
git status --short > docs/superpowers/baseline/2026-07-18-worktree-status.txt
```

Write `docs/superpowers/baseline/2026-07-18-test-baseline.md` with the exact four observed outcomes from the table above, including every current frontend lint file and rule category. Do not describe the interrupted build as a failure or success; record it as non-terminal.

- [x] **Step 5: Verify and review the checkpoint**

Run:

```bash
bash scripts/verify/baseline_test.sh
git diff -- scripts/verify .gitignore
git status --short -- docs/superpowers/baseline scripts/verify .gitignore
```

Expected: tool check exits 0; diff contains no source deletion or unrelated worktree change. Do not commit.

### Task 2: Add Architecture and Generated-Code Guards

**Files:**
- Create: `api/internal/architecture/dependencies_test.go`
- Create: `api/internal/architecture/runtime_roles_test.go`
- Create: `api/internal/architecture/testdata/known_violations.txt`
- Create: `scripts/verify/generated.sh`
- Create: `scripts/verify/generated_test.sh`
- Create: `scripts/verify/testdata/generated/stable/api/go.mod`
- Create: `scripts/verify/testdata/generated/stable/api/internal/ent/generate.go`
- Create: `scripts/verify/testdata/generated/stable/api/internal/ent/generate/value.txt`
- Create: `scripts/verify/testdata/generated/drift/api/go.mod`
- Create: `scripts/verify/testdata/generated/drift/api/internal/ent/generate.go`
- Create: `scripts/verify/testdata/generated/drift/api/internal/ent/generate/value.txt`
- Modify: `../go-common/pkg/database/widecolumn/client_test.go`
- Modify: `Makefile`

**Interfaces:**
- Consumes: module ownership rules from the master design.
- Produces: CI-detectable rejection of new cross-domain repository/Ent imports, API Docker access, and stale generated files; an explicit strict mode rejects every retained architecture violation.

- [x] **Step 1: Write failing architecture tests**

Create table-driven tests that parse imports beneath `api/internal` with `golang.org/x/tools/go/packages` and assert:

```go
var forbidden = []struct {
    fromPrefix string
    imports    []string
}{
    {"github.com/huynhanx03/judgify/internal/identity", []string{"/internal/problem/", "/internal/submission/", "/internal/contest/", "/internal/cultivation/", "/internal/material/"}},
    {"github.com/huynhanx03/judgify/internal/problem", []string{"/internal/identity/adapters/", "/internal/submission/adapters/", "/internal/contest/adapters/"}},
    {"github.com/huynhanx03/judgify/internal/submission", []string{"/internal/identity/adapters/", "/internal/problem/adapters/", "/internal/contest/adapters/"}},
    {"github.com/huynhanx03/judgify/internal/contest", []string{"/internal/identity/adapters/", "/internal/problem/adapters/", "/internal/submission/adapters/"}},
}
```

The runtime-role test scans imports reachable from the API role and rejects `github.com/docker/docker` and direct access to `internal/judge/executor`. Both tests compare findings against an exact sorted baseline in `testdata/known_violations.txt`: default mode fails on any added, removed, or reordered finding so debt can only change under review; `JUDGIFY_ARCH_STRICT=1` fails whenever the baseline is non-empty.

- [x] **Step 2: Run the focused tests and record existing violations**

Run: `cd api && go test ./internal/architecture -run 'Test(ModuleDependencies|APIRoleHasNoDocker)' -count=1`

Expected: FAIL with an explicit sorted list because the known-violation baseline does not exist yet. Store that exact list in `testdata/known_violations.txt` and the baseline document. This is a ratchet, not a suppression: new violations fail immediately, removed violations also require deleting their baseline rows, and strict mode remains red until the runtime split removes all rows.

- [x] **Step 3: Implement generated drift verification**

Create `scripts/verify/generated.sh` to:

1. copy the current list and checksums of `api/internal/ent/generate`;
2. run `go generate ./internal/ent`;
3. compare the new list/checksums;
4. print changed paths and exit non-zero on drift;
5. leave generated output in place for review rather than silently restoring it.

The shell test creates a temporary fixture repository with a stable generator and a drifted generator, asserting exit 0 and non-zero respectively.

- [x] **Step 4: Add explicit Make targets**

Add:

```make
.PHONY: test-api test-common test-web verify-generated verify-architecture verify

test-api:
	cd api && go test ./...

test-common:
	cd ../go-common && go test ./...

test-web:
	cd website && npm run lint && npm run build

verify-generated:
	bash scripts/verify/generated.sh

verify-architecture:
	cd api && go test ./internal/architecture -count=1

verify-architecture-strict:
	cd api && JUDGIFY_ARCH_STRICT=1 go test ./internal/architecture -count=1

verify: test-common test-api test-web verify-generated verify-architecture
```

- [x] **Step 5: Verify and review the checkpoint**

Before the complete go-common suite is accepted, replace its Scylla testcontainer's early banner-only readiness check with a bounded `cqlsh` success probe. The existing integration test is the regression test: preserve the reproduced connection-reset output in the baseline evidence, change only the readiness strategy, and verify the focused test before rerunning `make test-common`.

Run:

```bash
bash scripts/verify/generated_test.sh
make test-common
make test-api
git diff -- Makefile api/internal/architecture scripts/verify
```

Expected: script fixture tests, the baseline-ratcheted architecture guard, and existing Go suites pass. `make verify-architecture-strict` remains explicitly red until the process-role task removes every baseline row. Do not commit.

### Task 3: Inventory Migration and Identifier Reality

**Files:**
- Create: `api/internal/ent/migrate/inventory/inventory.go`
- Create: `api/internal/ent/migrate/inventory/inventory_test.go`
- Create: `api/cmd/judgify/main.go`
- Create: `api/cmd/judgify/inventory.go`
- Create: `api/cmd/judgify/inventory_test.go`
- Create: `docs/superpowers/baseline/2026-07-18-migration-inventory.md`

**Interfaces:**
- Consumes: live Ent schema definitions and ordered SQL migrations.
- Produces:

```go
type Report struct {
    MigrationFiles       []MigrationFile
    DeclaredTables       []string
    EntTables            []string
    IdentifierMismatches []IdentifierMismatch
    MissingMigrations    []string
    DestructiveSQL       []Finding
}

func Inspect(ctx context.Context, migrationFS fs.FS, schemas []ent.Interface) (Report, error)
func (r Report) Validate() error
```

- [x] **Step 1: Write failing inventory tests**

Fixture tests must prove detection of:

- bigint migration ID versus UUID Ent ID;
- an Ent table absent from migration history;
- a dropped table/column;
- stable ordered output independent of filesystem order;
- malformed SQL returning a typed error rather than being skipped.

- [x] **Step 2: Run the focused test**

Run: `cd api && go test ./internal/ent/migrate/inventory -count=1`

Expected: FAIL because `Inspect` and `Report.Validate` do not exist.

- [x] **Step 3: Implement the read-only inventory command**

The parser accepts the PostgreSQL constructs present in the checked-in migrations, records unknown statements as bounded diagnostics, and never connects to or mutates a database. The command:

```text
judgify inventory --format markdown
judgify inventory --format json
```

returns exit 1 when `Validate` finds mismatched identifiers, missing migration coverage, or destructive history that lacks an explicit annotation.

Create `cmd/judgify/main.go` as the single command entry point and keep `cmd/server/main.go` unchanged in this task. The operations plan later adds `api`, `judge-worker`, and `migrate` subcommands and then turns the legacy server command into a compatibility delegate; this task must not create two `main` functions in one package.

- [x] **Step 4: Generate and inspect the repository report**

Run:

```bash
cd api
go run ./cmd/judgify inventory --format markdown
```

Save the exact output to `docs/superpowers/baseline/2026-07-18-migration-inventory.md`. This step is read-only; it does not choose reset versus additive UUID migration.

- [x] **Step 5: Select the migration branch using evidence**

Use the report and deployment/data evidence:

- if no authoritative deployment or data exists, document the evidence and require explicit user approval before replacing unreleased bigint history with a UUID baseline;
- otherwise select additive UUID shadow columns, one durable UUIDv7 mapping row per legacy ID, bounded resumable backfill, compatibility writes, read switch, and later contract migration.

No destructive SQL is written until that decision is recorded.

- [x] **Step 6: Verify and review the checkpoint**

Run:

```bash
cd api
go test ./internal/ent/migrate/inventory -count=1
go test ./...
git diff -- cmd/judgify internal/ent/migrate/inventory ../docs/superpowers/baseline
```

Expected: inventory tests and the API suite pass; report contains no invented production-data claim. Do not commit.

## Cross-Plan Review Checkpoints

After each task:

1. Run its focused red/green test cycle.
2. Run the owning module’s complete test suite.
3. Run `go test -race` for modified concurrent Go packages.
4. Inspect `git diff --check`.
5. Inspect path-scoped `git diff` and `git status`; never stage or commit.
6. Confirm no existing user change outside the task path was altered.

After each plan:

1. Re-run `make verify` or record the exact pre-existing frontend blocker if the frontend plan has not removed it yet.
2. Run the plan-specific integration and security suites.
3. Update `docs/superpowers/baseline/implementation-status.md` with task IDs, commands, exit codes, and remaining gates.
4. Review the approved spec section-by-section and map every acceptance statement to test evidence.

## Product Completion Gate

The work is not called complete until all child-plan checkboxes and these independent proofs are green:

- zero acknowledged submissions lost across injected crash boundaries;
- fresh non-root sandbox and cgroup per testcase, with hostile corpus contained;
- desired and applied authorization revisions remain atomic and fail closed;
- WebSocket replay is authorized, contiguous, bounded, and REST-resynchronizable;
- every production frontend route uses a real API and centralized Vietnamese copy;
- light, dark, and system themes plus keyboard/reduced-motion behavior pass browser checks;
- migration-from-zero, upgrade, generated drift, race, vulnerability, contract, and schema checks pass;
- encrypted off-host backup restore meets RPO/RTO targets;
- load, soak, rollout, and binary rollback rehearsals produce retained evidence.
