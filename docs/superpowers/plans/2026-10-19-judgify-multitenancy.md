# Judgify Multi-Tenancy Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement this plan task-by-task. Do not run parallel edits across shared Ent schemas, migrations, tenant context, or router composition.

**Goal:** Add secure tenant isolation to Judgify with global identity, tenant-scoped business data and authorization, Tông môn/Owner/Member flows, and FE tenant selection.

**Architecture:** Use shared PostgreSQL tables with non-null `tenant_id` on tenant-owned records. Resolve a typed tenant scope from the authenticated account’s memberships, apply it to Ent transactions and RLS, then pass it to scoped repositories, Casbin, workers, realtime, storage, and client cache keys. Initialize a fresh database with Judgify Community and the bootstrap Owner; do not import records from the current database.

**Tech Stack:** Go, Ent, PostgreSQL, Casbin via `go-common`, Gin, Next.js/React, TypeScript.

**Spec:** `docs/superpowers/specs/2026-10-19-judgify-multitenancy-design.md`

## Global Constraints

- Account, username, credentials, federated identities, authentication sessions, and recovery contacts remain global.
- Every tenant-owned row has a non-null `tenant_id`; tenant-owned foreign references must remain within the same tenant.
- Empty tenant filter means all memberships only on explicitly supported read-only list APIs; writes and authorization require one concrete tenant.
- Each account must retain at least one active tenant membership.
- An Owner cannot leave a tenant while retaining Owner status, and every tenant must retain at least one active Owner.
- Tenant Owners may change the display name once per rolling 365 days; the first rename is available immediately and the stable ID/slug never change.
- The configured bootstrap administrator owns Judgify Community; new registrations join Community as Member.
- A new Tông môn starts empty; its creator becomes Owner and may invite existing accounts as Members.
- Use PostgreSQL RLS with transaction-local scope and a non-owner application database role without `BYPASSRLS`.
- Keep authorization cache keys tenant-scoped and revision-aware.
- Use a clean database; do not retain or backfill current database rows.
- Do not commit changes.

## Review Focus

- **Empty scope on a write or authorization check:** reject before touching tenant data; the API must not treat it as a wildcard.
- **Member leaves or is removed from their final tenant:** return a conflict and preserve the membership.
- **Owner leaves, is removed, or is demoted:** require transfer/demotion first and preserve at least one active Owner per tenant.
- **Cross-tenant parent ID, cursor, export, or bulk ID:** do not return or mutate another tenant’s records.
- **Connection reuse, async work, realtime, or client switching:** prior tenant scope and cached data must not leak into the next operation.

---

### Task 1: Build the tenant data inventory and schema foundation

**Files:**
- Create: `docs/superpowers/baseline/2026-10-19-tenant-data-inventory.md`
- Create: `api/internal/ent/schema/tenant.go`
- Create: `api/internal/ent/schema/tenant_membership.go`
- Create: `api/internal/ent/schema/tenant_invitation.go`
- Create: `api/internal/ent/schema/tenant_role.go`
- Create: `api/internal/ent/mixin/tenant_owned.go`
- Modify: `api/internal/ent/entc.go`
- Modify: `api/internal/ent/schema/*.go` for every row classified as tenant-owned in the inventory
- Generate: `api/internal/ent/generate/**`
- Create: `api/internal/ent/migrate/migrations/20261019090000_tenant_foundation.sql`

**Interfaces:**
- Produces `Tenant`, `TenantMembership`, `TenantInvitation`, and `TenantRole` Ent entities.
- `TenantMembership` stores active participation, not one fixed role. User-to-role grants are Casbin grouping rules inside the tenant domain.
- Produces a tenant-owned mixin with required `tenant_id` and a tenant edge; tenant-specific indexes and composite constraints remain explicit in each schema.
- The inventory classifies every current schema as global, tenant-owned, or mixed/cross-cutting and names the tenant field or reason it stays global.

- [ ] **Step 1: Classify every Ent schema and business child/projection table** in the inventory. Include contest, problem, submission/judging, materials, cultivation/profile, notification, audit, outbox, realtime, and operations tables.
- [ ] **Step 2: Add tenant registry, membership, invitation, and tenant role schemas** with stable UUID IDs, unique slug, membership state without a role column, tenant-scoped role definitions, invitation lifecycle, and uniqueness on `(tenant_id, user_id)` / active invitation target.
- [ ] **Step 3: Add tenant ownership fields and same-tenant relation constraints** to every inventory row marked tenant-owned. Convert uniqueness that should be local to `(tenant_id, existing_key)`.
- [ ] **Step 4: Generate Ent code** with `go generate ./internal/ent` from `api` and update the migration checksum.
- [ ] **Step 5: Add schema migration SQL** for tenant-owned columns, constraints, indexes, and RLS enablement. Since the target database is new, the schema migration must not import current database rows.

### Task 2: Add typed tenant context and transaction-local RLS scope

**Files:**
- Create: `api/internal/tenantctx/tenantctx.go`
- Create: `api/internal/infrastructure/tenant_context.go`
- Modify: `api/internal/ent/client.go`
- Modify: `api/internal/platform/transaction/ent_unit_of_work.go`
- Modify: `api/internal/infrastructure/router.go`
- Modify: `api/internal/infrastructure/bootstrap/**` where the runtime and migration DB roles are configured
- Modify: `deploy/compose.production.yml`
- Modify: `deploy/env.production.example`
- Modify: `deploy/README.md`

**Interfaces:**
- `tenantctx.Scope` represents `Tenant(uuid.UUID)` or `All([]uuid.UUID)`; constructors validate non-empty, unique canonical tenant IDs.
- `tenantctx.FromContext(context.Context) (Scope, bool)` returns only a previously validated scope.
- Middleware resolves a requested tenant from the authenticated principal’s active memberships. All scope is permitted only for GET/HEAD routes explicitly registered for aggregation.
- Ent unit-of-work setup sets transaction-local PostgreSQL scope before application queries. Nested service transactions join the existing scoped transaction.

- [ ] **Step 1: Define typed tenant scope and context APIs** with no stringly-typed wildcard and no implicit system bypass.
- [ ] **Step 2: Add a membership-backed scope resolver** that distinguishes one tenant from all active memberships and fails closed for unknown, inactive, or non-member IDs.
- [ ] **Step 3: Apply transaction-local RLS settings** before Ent queries in request transactions and worker transactions; clear scope automatically at transaction completion through PostgreSQL transaction-local settings.
- [ ] **Step 4: Configure runtime and migration database roles** so the runtime role is not the table owner and does not bypass RLS; migrations/bootstrap use the separate privileged path.
- [ ] **Step 5: Register scope middleware after authentication** and before protected business handlers. Keep public content explicitly scoped to Judgify Community.

### Task 3: Implement tenant lifecycle and membership invariants

**Files:**
- Create: `api/internal/tenant/contract/tenant.go`
- Create: `api/internal/tenant/domain/tenant.go`
- Create: `api/internal/tenant/usecase/tenant.go`
- Create: `api/internal/tenant/adapters/driven/db/tenant.go`
- Create: `api/internal/tenant/adapters/driver/http/tenant.go`
- Create: `api/internal/tenant/adapters/driver/http/tenant_dto.go`
- Create: `api/internal/di/tenant.go`
- Modify: `api/internal/di/container.go`
- Modify: `api/internal/di/identity.go`
- Modify: `api/internal/infrastructure/router.go`
- Modify: `api/internal/identity/usecase/account_provisioning.go`
- Modify: `api/internal/onboarding/registration.go`
- Modify: `api/internal/operator/bootstrapadmin/bootstrap.go`
- Modify: `api/internal/ports/identity.go`
- Create: `api/internal/ports/tenant.go`

**Interfaces:**
- Tenant endpoints list only the caller’s memberships, create an empty tenant with the caller as Owner, and manage existing-account invitations.
- Membership operations enforce at least one active membership per account, at least one active Owner role assignment per tenant, and no departure while retaining Owner status.
- Role assignments are separate from membership so a member may hold multiple tenant roles in the future.
- Registration atomically creates global identity plus Community Member membership. Bootstrap initialization idempotently creates Community and assigns the configured admin as Owner.

- [ ] **Step 1: Define tenant use-case commands and repository ports** for list, create, invite existing account, accept/decline invite, leave, transfer ownership, role assignment, and membership removal.
- [ ] **Step 2: Implement transactional tenant creation** that inserts tenant, creator membership, Owner/Member role definitions, baseline policies, and creator Owner role assignment in one transaction, with no copied business rows.
- [ ] **Step 3: Implement invitation acceptance and removal rules**; reject removal of a final membership or last Owner role assignment, and reject Owner departure until transfer/demotion.
- [ ] **Step 4: Integrate registration and bootstrap** so clean startup seeds one Judgify Community, its Owner/Member role definitions and policies, and configured bootstrap Owner membership/assignment idempotently.
- [ ] **Step 5: Expose tenant/membership HTTP endpoints** under the authenticated API router with response DTOs that use technical values `owner`/`member` and separate localized FE labels.
- [ ] **Step 6: Add Owner-only tenant rename** with a database-atomic 365-day cooldown, stable slug, and `next_name_change_at` in API responses.

### Task 4: Make Casbin policy and runtime tenant-scoped

**Files:**
- Modify: `api/internal/ent/schema/casbin_rule.go`
- Modify: `api/internal/ent/schema/authorization_state.go`
- Modify: `api/internal/identity/usecase/authorization.go`
- Modify: `api/internal/identity/usecase/authorization_reconciler.go`
- Modify: `api/internal/identity/usecase/authorization_validation.go`
- Modify: `api/internal/identity/adapters/driven/db/authorization_policy.go`
- Modify: `api/internal/identity/adapters/driver/http/authorization.go`
- Modify: `api/internal/identity/contract/authorization.go`
- Modify: `api/internal/ports/identity.go`
- Modify: `api/internal/di/identity.go`
- Create: `api/internal/ent/migrate/migrations/20261019100000_tenant_authorization.sql`
- Reuse: `go-common/pkg/security/authorization/casbin/ScopedRuntimeCache`

**Interfaces:**
- Policy lookup and mutation receive a validated tenant scope.
- Tenant policy domains use canonical `tenant:<uuid>` values. Global system policy remains a separate domain.
- Durable desired/applied revision, audit, outbox, reconciliation, and L1 cache behavior are keyed per authorization domain.

- [ ] **Step 1: Extend persisted authorization state and policy rules** to partition tenant policies and revisions by scope without changing global system-role semantics.
- [ ] **Step 2: Update startup snapshot loading and reconciliation** to load/publish each domain independently and use tenant-scoped runtime cache entries.
- [ ] **Step 3: Require concrete scope for policy mutations and authorization checks**; reject empty/all scope before Casbin evaluation or persistence.
- [ ] **Step 4: Seed tenant-scoped Owner and Member role definitions and policy templates** separately from global system roles, with no cross-tenant role inheritance.
- [ ] **Step 5: Update authorization HTTP DTOs and API permissions** so an Owner can manage only the selected tenant’s policies.

### Task 5: Scope problem and material domains

**Files:**
- Modify: `api/internal/problem/**`
- Modify: `api/internal/material/**`
- Modify: `api/internal/cache/**`
- Modify: all generated Ent files regenerated from Task 1

**Interfaces:**
- Tenant-owned use cases obtain scope from typed context; repositories cannot run a tenant-owned query without it.
- All-scope reads expand only to the active membership IDs and return the tenant ID with each row.
- Background job/outbox/realtime/file/cache identifiers carry tenant scope end to end.

- [ ] **Step 1: Scope problem and material repositories** including slug lookup, search, revisions, test assets, tags/categories, exports, and public reads (public reads use Community).
- [ ] **Step 2: Implement opt-in all-membership aggregation only on documented problem/material GET collections.** Include each row's tenant ID and constrain to the active membership set.

### Task 6: Scope contest, submission, and judging domains

**Files:**
- Modify: `api/internal/contest/**`
- Modify: `api/internal/submission/**`
- Modify: `api/internal/judge/**`
- Modify: `api/internal/realtime/**` for contest/judge subscriptions

- [ ] **Step 1: Scope contest repositories and handlers** including registrations, content revisions, announcements, clarifications, standings, ratings, and direct-ID lookups.
- [ ] **Step 2: Scope submission and judge execution** including intake, idempotency, judge jobs/results, retries, and worker claims.
- [ ] **Step 3: Scope contest projections and scoring** including attempts, result tables, rating projections, and replay/rebuild paths.
- [ ] **Step 4: Carry tenant scope through contest and judging realtime events**; no event topic may publish to members of another tenant.

### Task 7: Scope profile, cultivation, and notifications

**Files:**
- Modify: `api/internal/cultivation/**`
- Modify: `api/internal/notification/**`
- Modify: `api/internal/identity/**` for tenant profile attributes only
- Modify: `api/internal/problem/**` for per-tenant solve/profile projections

- [ ] **Step 1: Scope profile attributes and public profile projections** while keeping the global User identity unchanged.
- [ ] **Step 2: Scope cultivation/onboarding state** including traits, progression, solve records, stats, rewards, rankings, and per-tenant privacy settings.
- [ ] **Step 3: Scope notifications and campaigns** by tenant and recipient; preserve platform-global notifications as an explicit global record class.
- [ ] **Step 4: Scope user-facing list aggregation** only on documented read endpoints and label each result with its tenant.

### Task 8: Scope cross-cutting audit, events, workers, and caches

**Files:**
- Modify: `api/internal/platform/audit/**`
- Modify: `api/internal/platform/eventing/**`
- Modify: `api/internal/realtime/**`
- Modify: `api/internal/cache/**`
- Modify: `api/internal/infrastructure/event_delivery_*.go`
- Modify: `api/internal/judge/**` for background worker tenant payloads

- [ ] **Step 1: Add tenant scope to audit and business outbox events** while keeping platform-only events explicitly global.
- [ ] **Step 2: Propagate scope through scheduled jobs, event delivery, realtime tickets/topics, object-storage keys, and cache keys.**
- [ ] **Step 3: Verify async handlers revalidate tenant membership/ownership of the referenced parent before reading or mutating records.**
- [ ] **Step 4: Audit direct IDs, cursors, exports, bulk operations, search, and worker retries** for cross-tenant access or leakage.

### Task 9: Add the frontend tenant experience

**Files:**
- Create: `website/contexts/tenant-context.tsx`
- Create: `website/services/tenant.service.ts`
- Create: `website/types/tenant.ts`
- Create: `website/modules/layout/tenant-selector.tsx`
- Create: `website/modules/tenant/create-tenant-dialog.tsx`
- Create: `website/modules/tenant/membership-management.tsx`
- Modify: `website/components/app-providers.tsx`
- Modify: `website/modules/layout/header.tsx`
- Modify: `website/modules/layout/mobile-nav-panel.tsx`
- Modify: `website/lib/api/client.ts`
- Modify: `website/contexts/realtime-context.tsx`
- Modify: page services and query owners under `website/services/**`, `website/hooks/**`, and `website/modules/**` according to the endpoint inventory
- Modify: `website/i18n/catalog.vi.ts` and the corresponding locale catalog(s)
- Consult: `website/node_modules/next/dist/docs/` before changing Next.js-specific routing/provider behavior, per `website/AGENTS.md`

**Interfaces:**
- `TenantContext` exposes memberships, `activeTenantId`, `isAllTenants`, `selectTenant(id)`, and `refreshMemberships()`.
- API client sends one selected tenant ID for tenant-specific requests; all mode is sent only by collection calls that opt into aggregation.
- User-facing roles display as Tông chủ and Đệ tử; stored/API values remain Owner and Member.

- [ ] **Step 1: Add tenant API contracts and service methods** for current memberships, create tenant, invite/accept, leave, transfer, and membership management.
- [ ] **Step 2: Add tenant context below session/auth providers** and restore selection only if it remains in the user’s current membership list.
- [ ] **Step 3: Add responsive tenant selector to desktop and mobile navigation** with current tenant, All, and Create Tông môn actions.
- [ ] **Step 4: Add Owner invite/member management** using existing-account discovery and invitation lifecycle.
- [ ] **Step 5: Add tenant rename UI** that permits the first rename immediately, then shows the localized remaining time until `next_name_change_at`.
- [ ] **Step 6: Thread tenant scope through every query/mutation service**; clear or namespace cached data and realtime subscriptions when selection changes.
- [ ] **Step 7: Add All view only to supported read-only lists** and display each row’s tenant. Disable tenant-owned mutations while All is selected.
- [ ] **Step 8: Add Vietnamese and English labels/errors** for tenant, Tông môn, Tông chủ, Đệ tử, invite lifecycle, name cooldown, and tenant-selection errors.

### Task 10: Initialize and build the clean deployment

**Files:**
- Modify: `api/internal/ent/migrate/migrations/**`
- Modify: `api/internal/infrastructure/bootstrap/**`
- Modify: `deploy/compose.production.yml`
- Modify: `deploy/env.production.example`
- Modify: `deploy/README.md`
- Modify: `ops/README.md` if it owns first-install instructions

- [ ] **Step 1: Verify migration chain creates a clean database** with all tenant-owned fields, same-tenant constraints, RLS policies, and separate runtime/migration roles.
- [ ] **Step 2: Verify bootstrap idempotence and seed cardinality** for Community, bootstrap Owner membership, Owner/Member policies, and required Community defaults.
- [ ] **Step 3: Build backend** with `cd api && go build ./...`.
- [ ] **Step 4: Build frontend** with `cd website && npm run build`.
- [ ] **Step 5: Inspect all migration diffs and deployment configuration** to confirm no import/backfill of the old database and no runtime RLS bypass role.

## Dependency order

Tasks 1 and 2 define the schema and trusted scope. Task 3 builds on them. Task 4 uses tenant IDs and validated context. Tasks 5 through 8 isolate every backend data path; tenant creation must remain disabled in production until they are complete. Task 9 consumes the complete API contract and therefore follows backend isolation. Task 10 closes the clean-install path and builds both applications.
