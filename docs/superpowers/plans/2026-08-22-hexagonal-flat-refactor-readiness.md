# Hexagonal Flat Architecture Refactor Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Safely migrate Judgify's API to the approved domain-first Hexagonal Flat architecture while preserving its public behavior and production safety controls.

**Architecture:** Establish root port/constant catalogs and import-boundary tests first; then migrate a small pilot slice, followed by one domain at a time. Each domain becomes `domain`, `usecase`, and flat `adapters/driver` and `adapters/driven` packages. Cross-domain flows move to `internal/application`; process mechanics move to infrastructure/platform.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, GoMock, Testify, native `testing`, `httptest`, existing architecture tests.

**Spec:** `docs/superpowers/specs/2026-08-22-hexagonal-flat-architecture-design.md`

## Global Constraints

- Preserve every HTTP route, JSON field, status code, authorization requirement, and browser-session behavior unless a compatibility change is separately approved.
- Internal domain/use-case/port IDs use `uuid.UUID`; HTTP DTO IDs remain strings.
- Keep adapter packages flat by feature; do not introduce `dto`, `mapper`, `builder`, `entity`, `service`, or `repository` directories.
- Keep one root `ports` package and one root `constant` package, with exactly one source file per domain catalog.
- Domain packages must not import Ent, Gin, adapters, DI, infrastructure, `ports`, or another business domain.
- Do not commit from the present dirty worktree. Create an isolated worktree before execution.

> **Execution exception (2026-08-22):** A clean worktree was created, but the
> current source tree contains untracked architecture work absent from that
> worktree. The user explicitly authorized direct execution in this dirty
> worktree. Preserve unrelated changes and do not create commits here.

---

## Milestone Map

| Milestone | Deliverable | Gate |
|---|---|---|
| 0 | Clean baseline, isolated worktree, executable architecture test baseline | focused architecture + package tests |
| 1 | Root `ports` and `constant` catalogs plus dependency guards | architecture tests |
| 2 | Pilot domain migrated end-to-end | HTTP, DB integration, domain/use-case tests |
| 3 | Identity and Problem migration | authentication and authoring contracts |
| 4 | Submission, Judge, and cross-domain intake migration | durability and worker contracts |
| 5 | Contest, Cultivation, Material, Notification migration | respective domain contracts |
| 6 | Shared-package rehoming and cleanup | full architecture/import checks |
| 7 | Regression/release verification | full Go, contract, migration, topology checks |

### Task 1: Establish a clean executable baseline

**Files:**
- Read: `README.md`, `Makefile`, `api/go.mod`, `api/internal/architecture/*.go`
- Create: `docs/superpowers/baseline/2026-08-22-hexagonal-flat-baseline.md`
- Test: `api/internal/architecture/*_test.go`

**Produces:** A clean-worktree execution point, a record of current failures, and the exact focused test commands used by later milestones.

- [ ] **Step 1: Create an isolated worktree and record its commit SHA**

Run: `git worktree add ../judgify-hexagonal-flat -b refactor/hexagonal-flat`

Expected: a clean worktree on a dedicated branch; do not alter the current dirty checkout.

- [ ] **Step 2: Run the existing focused architecture suite**

Run: `cd ../judgify-hexagonal-flat/api && go test ./internal/architecture`

Expected: capture every failing test and missing fixture before refactor work begins.

- [ ] **Step 3: Restore or document test fixtures before changing architecture rules**

Verify `api/internal/architecture/testdata/known_violations.txt` and the path assumptions in `runtime_roles_test.go` exist in the clean worktree. If absent from the branch, add them from the repository's intended baseline rather than weakening tests.

- [ ] **Step 4: Save the baseline report**

Record commit SHA, commands, pass/fail results, and any pre-existing failures in `docs/superpowers/baseline/2026-08-22-hexagonal-flat-baseline.md`.

- [ ] **Step 5: Commit**

Run: `git add docs/superpowers/baseline/2026-08-22-hexagonal-flat-baseline.md && git commit -m "docs: record hexagonal refactor baseline"`

### Task 2: Build root catalog foundations and import guards

**Files:**
- Create: `api/internal/ports/{identity,problem,submission,contest,cultivation,material,notification,judge,platform}.go`
- Create: `api/internal/constant/{identity,problem,submission,contest,cultivation,material,notification,judge,platform}.go`
- Modify: `api/internal/architecture/dependencies_test.go`, `api/internal/architecture/runtime_roles_test.go`
- Test: `api/internal/architecture/*_test.go`

**Produces:** Empty-but-valid root catalog package layout and enforceable layer-direction rules before behavior moves.

- [ ] **Step 1: Write architecture tests that reject forbidden domain imports**

Add rules asserting packages below each business domain's future `domain/` directory cannot import `internal/ports`, `internal/ent`, `github.com/gin-gonic/gin`, `internal/di`, `internal/infrastructure`, any `/adapters/`, or another business domain prefix.

- [ ] **Step 2: Run the new tests and verify they fail against a deliberate fixture**

Add a temporary test package fixture containing an illegal `internal/ent` import; run `go test ./internal/architecture -run TestDomainLayerDependencies`; confirm the failure names the fixture, then remove the fixture after validating the detector.

- [ ] **Step 3: Add root catalog source files with package documentation**

Each `ports/*.go` file uses `package ports`; each `constant/*.go` file uses `package constant`. Add package comments declaring that exported names must be domain-prefixed and root files may not import adapters, Gin, Ent, or infrastructure.

- [ ] **Step 4: Re-run architecture tests**

Run: `cd api && go test ./internal/architecture`

Expected: PASS, with no new baseline violations.

- [ ] **Step 5: Commit**

Run: `git add api/internal/{ports,constant,architecture} && git commit -m "refactor: add hexagonal catalog guardrails"`

### Task 3: Migrate one pilot vertical slice

**Files:**
- Modify: one low-risk existing catalog domain selected from `api/internal/problem` or `api/internal/material`
- Create: `<pilot>/domain/*.go`, `<pilot>/usecase/*.go`, `<pilot>/adapters/driver/http/*.go`, `<pilot>/adapters/driven/db/*.go`
- Modify: `api/internal/di/<pilot>.go`, `api/internal/infrastructure/router.go`
- Test: pilot domain unit, HTTP, DB integration, and architecture tests

**Consumes:** Root catalog conventions from Task 2.

**Produces:** A verified reference implementation for all subsequent domains.

- [ ] **Step 1: Select the pilot with no cross-domain write workflow**

Choose either a Problem taxonomy slice (tag/difficulty) or a Material category slice. Do not select authentication, submission intake, contest rating, or notification campaign as the pilot.

- [ ] **Step 2: Write failing domain and use-case tests**

Create table-driven tests for the aggregate's state/validation and use-case tests using a fake root port. The tests must compile without Gin or Ent imports in the domain/use-case packages.

- [ ] **Step 3: Move the pilot entity/rule code into `domain` and I/O orchestration into `usecase`**

Replace old `core/entity` and `core/service` imports for the pilot only. Keep HTTP behavior unchanged.

- [ ] **Step 4: Split HTTP and DB code into flat feature files**

Use `<feature>_handler.go`, `<feature>_dto.go`, `<feature>_mapper.go` in HTTP; use `<feature>_repository.go`, `<feature>_mapper.go`, `<feature>_builder.go` in DB. Keep DTO IDs as strings and internal IDs as `uuid.UUID`.

- [ ] **Step 5: Add adapter contract tests**

Use `httptest` to assert unchanged route/status/JSON behavior; use the existing PostgreSQL integration harness to assert Ent mapping and writes.

- [ ] **Step 6: Run the pilot gate**

Run focused domain tests, the pilot HTTP tests, DB integration tests, and `go test ./internal/architecture`.

- [ ] **Step 7: Commit**

Run: `git add api/internal/<pilot> api/internal/{ports,constant,di,infrastructure,architecture} && git commit -m "refactor(<pilot>): migrate pilot to hexagonal flat layout"`

### Task 4: Migrate Identity and Problem independently

**Files:**
- Modify: `api/internal/identity/**`, `api/internal/problem/**`, `api/internal/di/{identity,problem}.go`
- Modify: `api/internal/ports/{identity,problem}.go`, `api/internal/constant/{identity,problem}.go`
- Test: identity authentication/session tests; problem authoring/publication tests; HTTP and DB integration tests

**Consumes:** Pilot conventions from Task 3.

**Produces:** Pure Identity and Problem domains and stable cross-domain capabilities for later workflows.

- [ ] **Step 1: Define root port names before moving code**

Put cross-domain interfaces and public command/result types in `ports/identity.go` and `ports/problem.go`: `IdentityEligibility`, `IdentityCampaignAudience`, `ProblemJudgeSpecReader`, and other actual callers. Do not export broad `IdentityService` or `ProblemService` interfaces.

- [ ] **Step 2: Migrate Identity feature groups one commit at a time**

Move authentication, users, roles, sessions, recovery contacts, OAuth, and profile schema as separate commits. For every group, retain its HTTP DTO contract and add a use-case test with fake or GoMock root ports.

- [ ] **Step 3: Migrate Problem feature groups one commit at a time**

Move taxonomy, authoring, revision, publication, test cases, and immutable judge reads separately. Keep release-manifest and revision invariants covered by existing tests.

- [ ] **Step 4: Move domain constants deliberately**

Place only shared identifiers in `constant/identity.go` or `constant/problem.go`; keep typed lifecycle states in the relevant domain package.

- [ ] **Step 5: Run domain gates after each feature group**

Run the affected package tests, HTTP contract tests, DB integration tests, and architecture tests before each commit.

### Task 5: Migrate Submission and Judge with durable-flow gates

**Files:**
- Modify: `api/internal/submission/**`, `api/internal/judge/**`, `api/internal/application/submissionintake/**`
- Modify: `api/internal/ports/{submission,judge,identity,problem,contest,platform}.go`
- Modify: `api/internal/di/{submission,worker}.go`
- Test: intake, finalization, durable worker, sandbox capability, outbox, and architecture suites

**Consumes:** Identity/Problem root capability ports from Task 4.

**Produces:** Submission and Judge boundaries that preserve acknowledged-submission durability and API/worker isolation.

- [ ] **Step 1: Move cross-domain intake orchestration into `application/submissionintake`**

Its dependencies must be named `ports.IdentityEligibility`, `ports.ContestSubmissionReserver`, `ports.ProblemJudgeSpecReader`, `ports.SubmissionIntakeWriter`, and platform audit/outbox/transaction ports.

- [ ] **Step 2: Migrate Submission aggregate/use cases and DB adapter**

Keep idempotency, job reservation, finalization, rejudge, and solved-state tests green as each feature moves.

- [ ] **Step 3: Keep Judge as a dedicated worker component**

Judge imports only root Submission/Problem/Judge ports and its sandbox adapters. Do not move Docker/sandbox code into Submission.

- [ ] **Step 4: Run durability regression gates**

Run the focused submission and judge suites, outbox tests, worker role import tests, and existing failure/recovery tests.

### Task 6: Migrate Contest, Cultivation, Material, and Notification

**Files:**
- Modify: `api/internal/{contest,cultivation,material,notification}/**`
- Modify: `api/internal/ports/{contest,cultivation,material,notification}.go`
- Modify: `api/internal/constant/{contest,cultivation,material,notification}.go`
- Modify: `api/internal/di/{contest,cultivation,material,notification}.go`
- Test: each domain's existing unit/HTTP/DB tests plus cross-domain operation tests

**Consumes:** Stable root port catalog and workflow conventions.

**Produces:** Remaining domains in the same flat Hexagonal layout.

- [ ] **Step 1: Migrate Contest lifecycle, registration, content, standings, and rating by feature**

Move contest-aware submission reservation behind `ports.ContestSubmissionReserver`; do not import Problem or Submission implementations from Contest.

- [ ] **Step 2: Migrate Cultivation catalogs, progression, rewards, and ranking by feature**

Keep reward ledger and reconciliation operation semantics under use cases; preserve immutable reward/projection behavior.

- [ ] **Step 3: Migrate Material category, article, production, view, and lifecycle features**

Keep render/publish behavior behind platform ports and retain HTTP contracts.

- [ ] **Step 4: Migrate Notification inbox and campaign features**

Move campaign orchestration that needs Identity audience selection to `application/notificationcampaign`; remove any core-service import of a DB adapter.

- [ ] **Step 5: Run focused gates after each feature commit**

Run affected package tests, integration tests, `go test ./internal/architecture`, and the relevant frontend contract tests.

### Task 7: Rehome shared components and delete legacy layout

**Files:**
- Move: `api/internal/onboarding/**` to `api/internal/application/onboarding/**`
- Move: `api/internal/profile/**` to `api/internal/application/query/profile/**`
- Move: `api/internal/realtime/**` to `api/internal/platform/realtime/**`
- Move: `api/internal/runtime/**` to `api/internal/infrastructure/runtime/**`
- Move: `api/internal/authctx/**` to `api/internal/platform/authctx/**`
- Move: `api/internal/operator/**` to `api/internal/infrastructure/operator/**`
- Remove after migration: old `core`, domain-local `ports`, `constant`, `di`, `mocks`, and technical subdirectories

**Produces:** One canonical layout with no duplicate legacy paths.

- [ ] **Step 1: Migrate one shared package at a time with import rewriting**

For each move, update all Go imports with `goimports`, run the package tests, then run architecture tests before moving the next package.

- [ ] **Step 2: Replace domain-local mocks with test-only fakes or generated root-port mocks**

Put reusable test fixtures in `internal/testutil`; keep specialized fakes beside their consumer tests.

- [ ] **Step 3: Delete legacy directories only after zero imports remain**

Run: `rg 'internal/(identity|problem|submission|contest|cultivation|material|notification)/(core|ports|constant|di|mocks)' api --glob '*.go'`

Expected: no production imports of legacy paths.

- [ ] **Step 4: Commit each independently testable rehome**

Use `refactor(application): move onboarding workflow`, `refactor(platform): move realtime delivery`, and analogous scoped commit messages.

### Task 8: Final verification and documentation

**Files:**
- Modify: `docs/BACKEND_GUIDE.md`, `README.md`, `CONTRIBUTING.md`
- Modify: `scripts/verify/go.sh`, `scripts/verify/contracts.sh` if paths need updating
- Test: full Go, contract, migration, topology, and generation gates

**Produces:** Documentation and CI enforce the new architecture rather than the old layout.

- [ ] **Step 1: Update backend documentation with the target tree and dependency rules**

Document root `ports`/`constant`, flat-by-feature naming, UUID boundary rule, and the prohibition on technical artifact folders.

- [ ] **Step 2: Update generation and verification paths**

Ensure mock generation, Ent generation, architecture tests, and verification scripts reference the new directories.

- [ ] **Step 3: Run the full verification set**

Run: `make verify`, focused integration/contract checks, migration verification, and topology checks appropriate to the isolated worktree environment.

- [ ] **Step 4: Review import graph and deletion list**

Run `go list -deps` for API and worker commands; confirm API has no Docker/sandbox dependencies and worker has no Gin/product router dependencies.

- [ ] **Step 5: Commit documentation and verification updates**

Run: `git add README.md CONTRIBUTING.md docs scripts api && git commit -m "docs: document hexagonal flat architecture"`

## Plan Review

- Spec coverage: all approved root catalogs, flat file rules, UUID policy, DI/infrastructure responsibilities, cross-domain workflows, test placement, and legacy rehomes map to Tasks 1–8.
- Scope check: the work is intentionally staged; the pilot and each domain milestone produce independently testable software.
- Type consistency: all internal ID-bearing port commands/results use `uuid.UUID`; HTTP-only DTOs preserve string identifiers.
- Worktree safety: the plan begins in a new worktree because the current checkout has unrelated uncommitted changes.
