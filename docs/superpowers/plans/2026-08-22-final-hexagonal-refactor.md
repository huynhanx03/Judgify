# Final Hexagonal Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete Judgify's approved Hexagonal structure without changing externally observable behaviour.

**Architecture:** Keep root catalog and DI packages shared; make each business domain a layered module with `domain`, `contract`, `usecase`, and flat adapters. Test tooling is centralized under `internal/testutil`; PostgreSQL integration is runner-managed and isolated from unit tests.

**Tech Stack:** Go, Ent, PostgreSQL 16.14, GoMock, Testify, `httptest`, Docker/Testcontainers runner.

**Spec:** `docs/superpowers/specs/2026-08-22-hexagonal-flat-final-structure.md`

## Global Constraints

- Preserve HTTP, authorization, migrations, events, and UUID semantics.
- Keep one source file per domain in each root catalog.
- Never add domain-local `di`, `mocks`, `builder`, or `mapper` directories.
- Work directly in the authorized dirty worktree; preserve unrelated changes and do not commit.
- Run focused tests after every slice and `go test ./...` after every milestone.

---

### Task 1: Lock the final structural guardrails

**Files:**
- Modify: `api/internal/architecture/dependencies_test.go`
- Test: `api/internal/architecture/dependencies_test.go`

**Produces:** Tests that reject legacy `core`, domain-local DI/mocks, and nested adapter technical folders only after each relevant migration is complete.

- [ ] Add guards for completed domains first; do not mark Identity complete before its migration.
- [ ] Run `go test ./internal/architecture` and `git diff --check`.

### Task 2: Centralize generated mocks and shared fixtures

**Files:**
- Create: `api/internal/testutil/mocks/{identity,problem,submission,contest,cultivation,material,notification,judge,platform}.go`
- Modify: generated mock directives and usecase tests importing domain-local mocks
- Delete: `api/internal/<domain>/mocks/` after all consumers move

**Produces:** Test-only GoMock packages generated from `internal/ports/<domain>.go`, with compile-time contract assertions.

- [ ] Regenerate one domain mock file at a time from the root catalog.
- [ ] Update test imports and run that domain's usecase tests.
- [ ] Delete the old mock directory only after the domain gate passes.

### Task 3: Move Identity models and orchestration

**Files:**
- Move: `api/internal/identity/core/entity/*.go` to `api/internal/identity/domain/*.go`
- Move: `api/internal/identity/core/service/*.go` to `api/internal/identity/usecase/*.go`
- Create: `api/internal/identity/contract/identity.go`
- Modify: all Identity consumers, root ports, root DI, and Identity tests

**Interfaces:**
- `domain` exposes Identity entities and business errors.
- `contract` exposes Identity commands, queries, and results.
- `usecase` implements root service ports using domain values and contracts.

- [ ] Start with a failing architecture test that permits no `identity/core/entity` imports after the move.
- [ ] Move all entities as one package-preserving slice, then update imports to `identity/domain` with the `entity` alias where needed.
- [ ] Move services to `identity/usecase`, update DI/workflow/HTTP imports, and leave HTTP behaviour intact.
- [ ] Extract only types crossing usecase boundaries into `contract/identity.go`; keep pure entities in `domain/`.
- [ ] Run `go test ./internal/identity/... ./internal/application/... ./internal/infrastructure ./internal/architecture`.

### Task 4: Move local DI into the root composition package

**Files:**
- Move/merge: `api/internal/<domain>/di/*.go` into `api/internal/di/<domain>.go`
- Modify: `api/internal/di/container.go`, `api/internal/di/wire.go`, command bootstrap callers
- Delete: every `api/internal/<domain>/di/`

**Produces:** One root composition package that builds adapters, use cases, handlers, and workers without business code importing DI.

- [ ] Migrate a domain constructor into `internal/di/<domain>.go`.
- [ ] Update all callers to root DI and remove the old local DI package.
- [ ] Repeat per domain; run affected domain plus `cmd` and infrastructure tests after each move.

### Task 5: Replace legacy core DTOs with contracts and HTTP DTOs

**Files:**
- Create: `api/internal/<domain>/contract/<domain>.go`
- Move/replace: `api/internal/<domain>/core/dto/*.go`
- Create/modify: `api/internal/<domain>/adapters/driver/http/<feature>_dto.go`, `<feature>_mapper.go`
- Modify: `api/internal/ports/<domain>.go`, usecase, workflow, and HTTP packages

**Produces:** Root service ports accept contract commands/results; HTTP adapters exclusively own JSON DTOs.

- [ ] For one feature, write mapper tests that parse a string UUID request into a UUID contract command and format the result back to a string response.
- [ ] Change the port and usecase signatures to the contract type, preserving JSON output through the HTTP mapper.
- [ ] Repeat feature-by-feature; delete a legacy DTO file only when all imports are gone.
- [ ] Run feature HTTP, usecase, workflow, and architecture tests.

### Task 6: Finish flat adapter naming and remove core

**Files:**
- Modify: all business-domain HTTP adapter files and tests
- Delete: remaining `api/internal/<domain>/core/` directories
- Modify: `api/internal/architecture/dependencies_test.go`

**Produces:** No business `core` package; every adapter uses flat feature file naming.

- [ ] Rename handler files to `<feature>.go`; retain `<feature>_dto.go` and `<feature>_mapper.go`.
- [ ] Keep DB feature files as `<feature>.go`, `<feature>_builder.go`, and `<feature>_mapper.go`.
- [ ] Add an architecture assertion that no business-domain `core/` directory exists.
- [ ] Run `go test ./...`.

### Task 7: Establish one PostgreSQL integration runtime

**Files:**
- Create: `test/integration/runner/`, `test/compose.integration.yml`, `api/internal/testutil/postgres/`
- Modify: `Makefile`, PostgreSQL integration tests, contributor documentation

**Produces:** `make test-integration` starts exactly one production-pinned PostgreSQL container, migrates once, runs `go test -tags=integration`, and tears down once.

- [ ] Add `//go:build integration` to real PostgreSQL tests; leave SQLMock tests untagged.
- [ ] Use the digest pinned in root `docker-compose.yml`; do not use an unpinned PostgreSQL tag.
- [ ] Give each package an isolated schema/database from a runner-prepared template.
- [ ] Add `test-integration-keep` and `test-integration-down` targets for local reuse without CI reuse-by-name.
- [ ] Run the unit suite and the integration command independently.

### Task 8: Final verification and documentation

**Files:**
- Modify: architecture tests and contributor test documentation

- [ ] Run `git diff --check`.
- [ ] Run `go test ./...`.
- [ ] Run `make test-integration` when Docker is available.
- [ ] Confirm every root catalog has exactly one source file/domain and no domain has `core`, local `di`, or local `mocks`.
