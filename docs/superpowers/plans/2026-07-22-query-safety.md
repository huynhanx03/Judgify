# Query Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make all 19 dynamic Ent repositories whitelist query columns, preserve supported virtual filters, and always bound result queries.

**Architecture:** Add domain-neutral split filter/window helpers in go-common, then use generated Ent field constants at every Judgify repository boundary. Problem/tag/material translate their known virtual keys into typed Ent predicates before the generic physical-column helper.

**Tech Stack:** Go, Ent, go-common DTO/query helpers, google/uuid, go-sqlmock, testify.

## Global Constraints

- Do not commit.
- Do not edit contest correctness behavior, frontend, submission, judge, or realtime code.
- Unknown filter/sort keys must never reach generated SQL.
- Nil and empty query options must use the default bounded window.

---

### Task 1: go-common split query contracts

**Files:**
- Modify: `/Users/jerry/nhanhmc/coding/go-common/pkg/database/ent/request.go`
- Modify: `/Users/jerry/nhanhmc/coding/go-common/pkg/database/ent/request_test.go`

**Interfaces:**
- Produces: `ApplyQueryFilters(*dto.QueryOptions, *sql.Selector, ...string)` and `ApplyQueryWindow(*dto.QueryOptions, *sql.Selector, ...string)`.
- Preserves: `ApplyQueryOptions`, now composed from the split helpers and bounded for nil options.

- [ ] Add failing selector tests for nil options and split helper whitelist/window behavior.
- [ ] Run `go test ./pkg/database/ent -count=1` and confirm the nil-options test fails because no LIMIT is emitted.
- [ ] Implement the two split helpers and compose `ApplyQueryOptions` from them.
- [ ] Re-run the package tests and keep all existing operator/security tests green.

### Task 2: problem/material behavioral regressions

**Files:**
- Create: `/Users/jerry/nhanhmc/coding/Judgify/api/internal/problem/adapters/driven/db/query_safety_test.go`
- Create: `/Users/jerry/nhanhmc/coding/Judgify/api/internal/material/adapters/driven/db/query_safety_test.go`

**Interfaces:**
- Problem supports physical `difficulty_id` and virtual `tag_ids`.
- Tag supports virtual `element_id`.
- Material supports physical `category_id` and virtual `search`.

- [ ] Add a sqlmock query-capturing test for `ProblemRepository.FindPublished` with `does_not_exist`, empty options, `difficulty_id`, and `tag_ids`.
- [ ] Add a sqlmock query-capturing test for `MaterialRepository.FindPublic` with `search` and `category_id`.
- [ ] Run both adapter packages and confirm SQL contains the unknown/virtual column names or lacks LIMIT before implementation.

### Task 3: domain-owned virtual predicates

**Files:**
- Create: `/Users/jerry/nhanhmc/coding/Judgify/api/internal/problem/adapters/driven/db/query_options.go`
- Create: `/Users/jerry/nhanhmc/coding/Judgify/api/internal/material/adapters/driven/db/query_options.go`
- Modify: problem repositories `problem.go`, `tag.go`, `difficulty.go`
- Modify: material repositories `material.go`, `material_category.go`

**Interfaces:**
- Consumes: common split query helpers from Task 1.
- Produces: typed Ent predicates for `tag_ids`, `element_id`, and material `search`.

- [ ] Define per-repository physical allowlists from generated `Field*` constants.
- [ ] Parse string, UUID, `[]string`, and JSON-decoded `[]any` UUID filter values without converting malformed input to SQL.
- [ ] Apply virtual predicates, filter before Count, and window only before All.
- [ ] Run the problem/material query-safety tests until green.

### Task 4: remaining repository allowlists

**Files:**
- Modify: `/Users/jerry/nhanhmc/coding/Judgify/api/internal/contest/adapters/driven/db/contest.go`
- Modify: eight cultivation repository files containing `ApplyFilters`
- Modify: five identity repository files containing `ApplyFilters`

**Interfaces:**
- Consumes: `ApplyQueryFilters` and `ApplyQueryWindow`.
- Produces: an explicit generated-field allowlist at every remaining dynamic repository.

- [ ] Add an AST architecture regression that rejects two-argument legacy `ApplyFilters`/`ApplySort` calls.
- [ ] Run the architecture test and confirm all current legacy callsites fail it.
- [ ] Convert each repository to generated field constants, filtered Count, and always-on bounded window.
- [ ] Run `rg`/architecture tests and confirm all 19 repositories are covered.

### Task 5: verification

**Files:**
- Verify only; format modified Go sources mechanically.

- [ ] Run focused unit tests for go-common query helpers and Judgify problem/material/cultivation/identity/contest adapters.
- [ ] Run focused `go test -race` for the modified adapter packages.
- [ ] Run focused `go vet` for the modified adapter packages.
- [ ] Run `git diff --check` in both repositories and report any unrelated pre-existing failures separately.
