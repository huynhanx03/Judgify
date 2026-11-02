# Unified Find Method Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Standardize collection and filtered-read method names on `Find...` throughout Judgify.

**Architecture:** Standardize query methods across ports/usecases/adapters and collection-query routes. Use `By...` for actual filters and `For...` for audience/visibility scope. Keep request/response payload contracts unchanged.

**Tech Stack:** Go, TypeScript/React.

**Spec:** `docs/superpowers/specs/2026-11-01-unified-find-methods-design.md`

## Global Constraints

- `Find...` names collection and filtered/paginated reads; `Get` names singular reads; write methods remain `Create`, `Update`, and `Delete`.
- Public/admin visibility is determined by trusted service authorization, not by a request-supplied scope flag.
- Use `/find` for collection query routes; keep HTTP verbs, JSON fields, response shapes, and UI search copy unchanged.
- Preserve business commands including `Submit`, `Publish`, `Archive`, and `Register`.
- Preserve and update existing route-contract tests and mocks; do not add or run tests unless requested.

## Review Focus

- Public/admin visibility remains unchanged: preserve distinct `FindForPublic`/`FindForAdmin` paths where their authorization and filters differ.
- Query routes use `/find`; resource paths for singular reads and writes and serialized contracts remain unchanged.
- Mock call names remain aligned with renamed methods, including recorder method strings.
- Query method identifiers and collection query paths are standardized; similarly named request/response types and UI search labels remain intact.
- Go and website callers use the same renamed query methods end-to-end.

---

### Task 1: Rename Go query methods end-to-end

**Files:** Go sources under `api/internal`, excluding generated Ent code; includes ports, use cases, DB/HTTP adapters, and test mocks.

**Interfaces:** Preserve every existing signature except the method identifier. Rename `List...`, `Page...`, and `Search...` query methods to `Find...`, including bare `List`, `Page`, and `Search` methods as `Find`. Use `By...` for actual filters and `For...` for caller/visibility scope.

- [x] Rename interface declarations, implementations, selectors/call sites, and mock methods/recorder names together.
- [x] Keep query/request/response types, HTTP methods, JSON, and user-visible strings unchanged; standardize query routes on `/find`.
- [x] Run `gofmt` on modified Go files.

**Check:** Search Go declarations and calls to confirm no query method remains named `List...`, `Page...`, or `Search...`; exclude types, local variables, tests that describe text search, and HTTP routes.

### Task 2: Rename website service methods and callers

**Files:** `website/services/**/*.ts` and their direct TypeScript/React call sites.

**Interfaces:** Rename corresponding list/page/search service methods to `find...`, update collection query URLs to `/find`, and retain payloads, response types, and UI copy.

- [x] Rename website service method declarations and call sites.
- [x] Keep user-facing search terminology unchanged; use `/find` for collection query URLs.

**Check:** Search website service method declarations/calls for stale list/page/search verbs; inspect the diff to confirm only identifiers and references changed.

### Task 3: Review the full rename diff

**Files:** All files changed by Tasks 1–2.

- [x] Inspect changed identifiers and verify public/admin query scope remains explicit.
- [x] Verify no unrelated dirty worktree changes were reverted or staged.

**Check:** `git diff --check` and a focused search for the old method identifiers. No tests or commits are included in this task.
