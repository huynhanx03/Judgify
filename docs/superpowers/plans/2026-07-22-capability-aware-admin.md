# Capability-Aware Admin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a revision-consistent effective-capability endpoint and make the complete admin frontend capability-aware without logging users out for transient failures.

**Architecture:** The authorization runtime precomputes a catalog-filtered capability projection inside each immutable revision view. The frontend stores that snapshot in AuthContext and applies a centralized route/navigation/action policy while the backend remains authoritative.

**Tech Stack:** Go, Gin, Casbin runtime, Ent-backed authorization snapshot, Next.js 16, React 19, TypeScript, Node test runner.

## Global Constraints

- Strict red-green-refactor TDD; observe every new test fail for the expected missing behavior.
- Only a final HTTP 401 clears authentication; network and 5xx failures preserve session state.
- Never return raw Casbin rules, wildcard grants, subjects, role keys, or out-of-catalog capabilities.
- Keep UI copy in `website/constants/text.ts` and preserve light/dark tokens.
- Do not modify contest or query repositories. Do not commit.

---

### Task 1: Effective Capability Projection

**Files:**
- Modify: `api/internal/identity/core/dto/authorization.go`
- Modify: `api/internal/identity/ports/authorization.go`
- Modify: `api/internal/identity/core/service/authorization.go`
- Test: `api/internal/identity/core/service/authorization_test.go`

**Interfaces:**
- Produces: `EffectiveCapabilities(ctx context.Context, userID uuid.UUID) (*dto.EffectiveCapabilitiesResponse, error)`.
- Produces: `CapabilityGrant{Resource string, Actions []string}` and `EffectiveCapabilitiesResponse{Revision int64, Capabilities []CapabilityGrant}`.

- [ ] Write service tests that request the teacher, multi-role, and protected-superadmin projections and assert catalog-only stable grants with no `*` values.
- [ ] Run `go test ./internal/identity/core/service -run 'TestEffectiveCapabilities' -count=1` and observe missing-method failures.
- [ ] Add the DTO and reader port, precompute projections in `newAuthorizationView`, and return a deep copy from the current healthy view.
- [ ] Mutate a role policy through the existing revision-fenced service in a test and assert the next read returns the new revision and grants.
- [ ] Run the focused service tests until green and refactor projection helpers without changing behavior.

### Task 2: Protected Self-Capability HTTP Boundary

**Files:**
- Modify: `api/internal/identity/adapters/driver/http/authorization.go`
- Modify: `api/internal/identity/adapters/driver/http/handler.go`
- Modify: `api/internal/identity/di/di.go`
- Create: `api/internal/identity/adapters/driver/http/authorization_test.go`

**Interfaces:**
- Consumes: `EffectiveCapabilities` from Task 1.
- Produces: authenticated `GET /authorization/me` with no resource permission middleware.

- [ ] Write handler tests proving a canonical authenticated UUID is passed to the reader and missing principal returns unauthorized.
- [ ] Write a route test proving `/authorization/me` is mounted outside the `role:read` group while `/authorization/catalog` stays capability protected.
- [ ] Run the focused HTTP tests and observe missing handler/route failures.
- [ ] Extend `AuthorizationHandler`, derive the UUID with `authctx.UserID`, and mount the route in the protected identity group.
- [ ] Run the focused HTTP tests until green.

### Task 3: Any-Of Authorization Middleware for Role Management Reads

**Files:**
- Modify: `../go-common/pkg/common/http/middlewares/authorization.go`
- Test: `../go-common/pkg/common/http/middlewares/authorization_test.go`
- Modify: `api/internal/identity/adapters/driver/http/handler.go`

**Interfaces:**
- Produces: generic `PermissionRequirement{Resource, Action string}` and `RequireAnyPermission(...PermissionRequirement) gin.HandlerFunc`.

- [ ] Write middleware tests for first-grant allow, later-grant allow, all-denied 403, unauthenticated 401, evaluation error 500, and empty configuration fail-closed.
- [ ] Run the focused go-common middleware test and observe the missing API failure.
- [ ] Implement one principal validation followed by short-circuit authorization checks; never allow on evaluation error.
- [ ] Use the any-of guard for role/catalog reads (`role:read` or `authorization:manage`) while preserving exact mutation guards.
- [ ] Run go-common and identity HTTP tests until green.

### Task 4: Frontend Capability Model and Failure-Safe Session Bootstrap

**Files:**
- Modify: `website/constants/api/identity.ts`
- Modify: `website/constants/authorization.ts`
- Modify: `website/types/auth.ts`
- Create: `website/lib/auth/capabilities.ts`
- Create: `website/services/authorization.service.ts`
- Modify: `website/lib/api/client.ts`
- Modify: `website/contexts/auth-context.tsx`
- Test: `website/tests/admin-capability-contract.test.mjs`
- Test: `website/tests/session-failure-contract.test.mjs`

**Interfaces:**
- Produces: `can(resource, action)`, `canAny(requirements)`, `hasAnyCapability`, `capabilityStatus`, and `refreshCapabilities` on AuthContext.

- [ ] Write pure capability tests for exact matching, any matching, empty snapshots, and revision-preserving normalization.
- [ ] Write transport tests proving refresh 401 clears credentials while refresh network/5xx rejects without clearing.
- [ ] Run the tests and observe missing module/state failures.
- [ ] Add the endpoint/type/service, pure indexed capability snapshot, and failure classifier.
- [ ] Refactor API refresh so only HTTP 401 returns authentication-invalid; propagate transient failures.
- [ ] Refactor AuthProvider to keep profile authentication independent from capability loading and expose retry.
- [ ] Run frontend focused tests, lint, and typecheck until green.

### Task 5: Central Admin Route, Navigation, Dashboard, and Error States

**Files:**
- Modify: `website/constants/admin-navigation.ts`
- Create: `website/lib/auth/admin-policy.ts`
- Create: `website/modules/admin/admin-access-state.tsx`
- Modify: `website/app/admin/(dashboard)/layout.tsx`
- Modify: `website/modules/admin/admin-sidebar-nav.tsx`
- Modify: `website/app/admin/(dashboard)/page.tsx`
- Modify: `website/constants/text.ts`
- Test: `website/tests/admin-capability-contract.test.mjs`

**Interfaces:**
- Consumes: Task 4 capability helpers.
- Produces: `adminRouteAllowed(pathname, capabilityIndex)`, filtered nav sections, and explicit forbidden/unavailable states.

- [ ] Write pure tests for root-any, exact list/create/edit requirements, role-read-or-manage, categories, longest-route matching, filtered nav, and dashboard cards.
- [ ] Run tests and observe missing policy helpers.
- [ ] Implement the centralized policy and attach requirements to navigation/card definitions.
- [ ] Guard the dashboard layout without redirecting authenticated denied/error states; add centralized 403 and retry copy.
- [ ] Fetch and render only permitted dashboard stats and hide empty sidebar sections.
- [ ] Run focused tests, lint, and typecheck until green.

### Task 6: Capability-Aware Admin Actions

**Files:**
- Create: `website/modules/admin/admin-resource-actions.tsx`
- Modify: `website/modules/admin/data-table-shell.tsx`
- Modify: CRUD pages under `website/app/admin/(dashboard)` for contests, difficulties, elements, levels, materials, material categories, problems, ranks, rarities, tags, and traits.
- Modify: `website/app/admin/(dashboard)/users/page.tsx`
- Modify: `website/app/admin/(dashboard)/roles/page.tsx`
- Test: `website/tests/admin-action-capability-contract.test.mjs`

**Interfaces:**
- Produces: reusable exact update/delete controls and `DataTableShell.canCreate`.

- [ ] Write contract tests requiring every CRUD page to bind its code-owned resource and forbidding unguarded create/edit/delete controls.
- [ ] Run tests and observe unguarded action failures.
- [ ] Add the reusable row actions and create visibility prop, then migrate generic pages.
- [ ] Guard contest publish with `contest:update`, user create/update/delete with the matching user action plus `authorization:manage`, and role policy editing with `authorization:manage`.
- [ ] Refresh the current capability snapshot after successful role-policy or user-role changes.
- [ ] Run focused tests, lint, and typecheck until green.

### Task 7: Verification

**Files:**
- Verify all files changed by Tasks 1–6.

- [ ] Run `go test ./pkg/common/http/middlewares -count=1` and `go test -race ./pkg/common/http/middlewares -count=1` in `go-common`.
- [ ] Run `go test ./internal/identity/... -count=1`, focused `-race`, and `go vet ./internal/identity/...` in `Judgify/api`.
- [ ] Run `npm run verify` and `npm run build` in `Judgify/website`.
- [ ] Run `git diff --check` in both repositories and inspect the scoped diff for secrets, wildcard leakage, hard-coded UI copy, and contest/query repository changes.
