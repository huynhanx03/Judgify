# Judgify Multi-Tenancy Design

**Status:** Approved for planning; membership rules clarified by user on 2026-10-19  
**Date:** 2026-10-19

## Goal

Make Judgify support multiple independently owned communities (tenants) while keeping one global account identity per person. Each tenant owns and isolates its business data, authorization assignments, and runtime views. Users can belong to multiple tenants and switch between them.

The initial tenant is **Judgify Community**. New registrations join it as **Member**. The configured bootstrap administrator becomes its **Owner**. A user can create a new tenant (shown in Vietnamese as a **Tông môn**); its creator becomes **Owner/Tông chủ**. Owners invite existing Judgify accounts as **Member/Đệ tử**. New tenants start with an empty business workspace.

The application will run against a new, clean database. Existing database contents are not preserved or backfilled. Schema migrations remain necessary to create the new database structure and seed its initial state.

## Decisions

- Use a pooled PostgreSQL database and shared tables, with a required `tenant_id` on every tenant-owned row.
- Keep account identity global: users, usernames, credentials, federated OAuth identities, authentication sessions, and recovery contacts do not belong to a tenant.
- Keep per-tenant profile and product state separate even when the same global user participates in several tenants.
- Keep global system roles separate from tenant roles. Membership records participation only; tenant role definitions and Casbin user-to-role assignments are separate so a member can gain additional roles later. Casbin policy domains identify a tenant; assignments never grant access in another tenant.
- Tenant context is selected by the client but validated by the backend against the authenticated user’s membership. It is never trusted as authorization by itself.
- An explicit all-tenant read mode may aggregate only tenants the user belongs to. Writes, authorization changes, and tenant-specific detail operations require one concrete tenant.
- Keep a process-local L1 authorization cache, keyed by tenant scope and policy revision. Tenant IDs must be part of every cache key.
- New tenant creation does not copy Community content. Import/copy can be a separate future feature.

## Data ownership

### Global identity and platform data

The following remain global and are not given a tenant owner:

- Global user/account record and username uniqueness.
- Credentials, federated identities, sessions, OAuth flows/tickets, recovery contacts, and password reset state.
- Platform-level role definitions and system-level authorization.
- Judge workers, worker capabilities, runtime profiles, and platform execution configuration.
- Process telemetry and infrastructure health data.

Global account settings that determine authentication or platform-wide visibility remain on the global identity. Any profile fields or progression data that describe a user inside a particular community are tenant-owned.

### Tenant-owned business data

Tenant ownership applies to all business domains, including:

- Problems and their revisions, tests, assets, tags, difficulties, and taxonomies.
- Contests, contest configuration/content, registrations, announcements, standings, rating results, and contest transitions.
- Submissions, judge jobs/results, judgements, scoring attempts, and all derived contest/problem results.
- Materials, categories, tags, revisions, slugs, and view aggregates.
- Per-tenant profile attributes, traits, onboarding choices, progression, rewards, solve history, stats, and rankings.
- Tenant notifications, campaigns, recipients, preferences, and tenant-scoped realtime events.
- Tenant settings, role assignments, authorization policies, audit entries, and business outbox events.

Every tenant-owned child, join, history, projection, aggregate, and asynchronous work record carries `tenant_id`, even if its parent also has that column. This allows database row policies and indexes to scope the row directly and lets constraints prevent cross-tenant references. Tables not explicitly listed here must be classified during schema inventory; no business table may be left implicitly global.

Some cross-cutting records may be global or tenant-scoped depending on their event/resource. For example, an audit record for a tenant action includes that tenant ID; a platform operation does not. The schema inventory must encode this distinction explicitly rather than treating missing tenant IDs as an all-tenant scope.

## Storage and isolation

Tenant-owned rows have a non-null `tenant_id` referencing `tenants.id`. Tenant-owned uniqueness is scoped, for example `(tenant_id, slug)`, rather than globally unique unless the product contract explicitly requires a global key.

Tenant role definitions have a tenant ID and are unique by `(tenant_id, role_key)`. Membership records active participation only; Casbin grouping rules assign one or more tenant roles to a member. Tenant-scoped authorization state and rules carry the tenant ID; explicitly global system policy rows are marked as global and cannot be used as tenant rules.

Relationships between tenant-owned rows must include tenant identity. Where practical, the database enforces ownership with composite unique keys and composite foreign keys such as `(tenant_id, problem_id)` and `(tenant_id, contest_id)`. A submission from one tenant therefore cannot reference another tenant’s problem or contest even if application code passes an incorrect ID.

PostgreSQL Row-Level Security (RLS) is defense in depth for tenant-owned tables:

- Enable and force RLS on tenant-owned tables.
- Use an application database role that does not own protected tables and does not have `BYPASSRLS`.
- Default missing/invalid tenant context to deny access.
- Set tenant scope transaction-locally for request and worker transactions. Multi-tenant read-only aggregation uses the authenticated membership set, never an unrestricted wildcard.
- Keep an explicit, separately controlled platform path for bootstrap and platform administration.

Ent privacy/interceptor rules and repository APIs also require a tenant scope and apply tenant predicates. RLS is not a replacement for authorization, membership checks, or application-level filtering.

## Tenant lifecycle and authorization

### Bootstrap

Clean database initialization creates the `Judgify Community` tenant and its baseline tenant-owned catalogs/configuration. It assigns the configured bootstrap administrator an active membership and Owner role assignment, and seeds tenant-scoped Owner/Member role definitions and system-level authorization separately. This process must be idempotent so restarting initialization does not duplicate tenants, memberships, or policies.

New account registration creates the global identity, then joins it to Judgify Community as Member. Account creation and membership creation must succeed or fail as one logical operation.

### Create and invite

Creating a tenant creates the tenant, creator’s active membership, tenant-scoped Owner/Member role definitions, initial policies, and creator’s Owner role assignment atomically. The tenant starts empty apart from required system defaults. An Owner can invite an existing global account as Member; accepting an invitation creates an active membership and a Member role assignment for that tenant in one transaction. Invitations and memberships are unique per tenant/user and have explicit lifecycle states.

An Owner cannot leave a tenant while retaining the Owner role; they must transfer ownership or first be demoted to Member. A tenant must always retain at least one active Owner role assignment to an active member. Tenant administration operations require a concrete tenant ID and Owner capability.

### Tenant name changes

An Owner may change a tenant's display name at most once per rolling 365-day period. A newly created tenant can be renamed once immediately; creating it does not consume that first rename. The backend records the successful rename timestamp and atomically rejects further changes until 365 days have elapsed. It returns the next permitted rename time so the UI can show the remaining duration. The tenant UUID and technical slug remain stable across display-name changes and are never regenerated from the new name.

Every account must always retain at least one active tenant membership. A Member cannot leave their last tenant, and an Owner cannot remove another account’s last active membership. The API returns a clear conflict and leaves membership state unchanged when removal would violate either invariant.

### Authorization

System roles remain global. A tenant membership records active participation only; tenant role definitions and user-to-role assignments are separate, allowing multiple roles per member and additional role types later. Tenant role definitions and policy assignments use a domain derived from the tenant’s stable ID. Every tenant authorization check supplies the validated tenant scope. Empty scope is rejected by writes and authorization APIs; it never means a wildcard domain.

Authorization cache keys include tenant domain, subject, object/action dimensions as used by the current runtime, and policy revision. Policy changes invalidate the relevant local tenant cache after commit. Existing durable policy revision/outbox/reconciliation behavior remains the source for recovering from interrupted publication; this work must preserve it while adding per-tenant scope.

## Tenant context and API behavior

The FE maintains an active tenant selection and sends it on tenant-specific API requests. The backend resolves it to a typed scope only after checking that the current authenticated user has an active membership. Request bodies and URL IDs are identifiers, not proof of membership.

List endpoints that explicitly support aggregation accept an all-memberships read scope (represented by an empty tenant filter in the client contract). The backend expands it to the caller’s active memberships and constrains every query to that set. Responses identify the tenant for each item. Aggregation is opt-in per endpoint; an empty filter on an endpoint without all-scope support is rejected rather than silently broadened.

Mutations and tenant-specific detail/authorization operations require one concrete tenant. The FE disables tenant-owned mutations while in All view and prompts the user to select a tenant. Requests without a concrete tenant fail closed.

Tenant list/create, membership/invitation management, and current-membership APIs are global identity-facing APIs with their own authorization rules. Tenant business APIs receive tenant context through shared middleware and scoped repository context.

## Frontend behavior

- Add a shared tenant selector to the authenticated application shell.
- Show Judgify Community and the user’s other memberships; display tenant role labels as Tông chủ and Đệ tử while keeping API and storage values Owner and Member.
- Provide Create Tông môn and Owner membership/invitation management flows.
- Support All only on read-only collection screens whose APIs declare aggregation support. Clearly label each result with its tenant.
- Switching tenants clears or namespaces client query state, updates realtime subscriptions, and reloads tenant-bound data. No cached response from the previous tenant may appear under the new selection.
- Every authenticated page and API client must be audited; adding a selector alone is not considered tenant support.

## Background work, realtime, files, and caches

Tenant context is part of every job payload, outbox event, audit record for business actions, realtime subscription/topic, object-storage key, and tenant-owned cache key. Workers validate the tenant and parent ownership before processing. Realtime publication is scoped to the tenant and must not fan out across tenant boundaries. Existing global infrastructure events remain global and carry no tenant authority.

## Failure behavior and security invariants

- Missing tenant context on tenant-owned reads/writes fails closed, except for explicitly declared all-memberships read endpoints.
- A user with no active membership gets no tenant business data.
- A tenant ID belonging to another user is denied without revealing resource existence through error details.
- Cross-tenant foreign keys and bulk operations are rejected by both database constraints and application checks.
- Cache keys, pagination cursors, exports, search results, notifications, and realtime topics preserve tenant scope.
- Cross-tenant aggregation cannot be used for writes, policy administration, or mutation targets.
- RLS context is transaction-local and cannot leak when a database connection is reused by the pool.

## Delivery decomposition

This feature spans a large existing schema and all backend/frontend domains. Delivery must be split into reviewable, dependent workstreams rather than adding a cosmetic tenant picker first:

1. **Tenant foundation and authorization:** tenant/membership/invitation schema, bootstrap, registration membership, request scope, tenant-aware Casbin domain/cache, RLS infrastructure, and tenant management APIs.
2. **Backend data-plane isolation:** complete schema inventory; tenant keys and same-tenant constraints; scoped repositories and APIs across problem, contest, submission/judging, material, cultivation/profile, notifications, audit/outbox, and all background/realtime/file flows. Keep tenant creation hidden until tenant-owned API surfaces are isolated.
3. **Frontend tenant experience:** active tenant context, selector, create/invite flows, read-only All behavior where supported, cache/realtime switching, and audit of every authenticated page.
4. **Fresh database initialization and operational verification:** clean database migrations, idempotent Community/bootstrap seed, app database roles/RLS policies, deployment configuration, and documented initialization behavior. No old-row backfill is included.

Each workstream needs a concrete implementation plan and verification criteria before coding it. They must preserve global identity and enforce the same tenant boundary across HTTP, background workers, realtime, caches, and storage.

## Out of scope

- Migrating or retaining records from the current database.
- Separate database/schema per tenant or tenant-specific infrastructure.
- Copying Community content into newly created tenants.
- Treating empty tenant scope as authorization to access every tenant.
- Billing, tenant quotas, custom domains, and cross-tenant sharing.

## Acceptance criteria

- A clean installation initializes exactly one Judgify Community tenant and assigns the bootstrap administrator as Owner.
- New users join Community as Member while keeping one global identity.
- A user can create multiple empty tenants and receives the Owner role in each; Owner can invite an existing user with the Member role.
- An Owner can rename a tenant display name once per rolling 365 days; the UI shows when rename becomes available, and the tenant ID/slug remain stable.
- Every account retains at least one active tenant membership.
- An Owner cannot leave a tenant while keeping Owner status, and every tenant retains at least one active Owner.
- Each tenant-owned record has a non-null tenant owner, and database constraints reject cross-tenant references.
- A member can access only tenant data for their memberships, including through direct IDs, search, pagination, exports, notifications, realtime, and background work.
- Tenant-scoped authorization permits tenant Owner/Member policy differences, supports future additional role assignments, and does not change global system roles or leak assignments across domains.
- All view aggregates only the caller’s memberships and is read-only; concrete-tenant mutation behavior is enforced by both FE and BE.
- Tenant switching cannot reuse data, cache entries, cursors, or realtime subscriptions from the previous tenant.
- Clean database setup is repeatable; no current-database data is imported.
- Every Ent schema table and every FE/BE business endpoint has an explicit global-vs-tenant classification before rollout.

## References

- [PostgreSQL 17: Row Security Policies](https://www.postgresql.org/docs/17/ddl-rowsecurity.html)
- [AWS: Core isolation concepts for SaaS tenant isolation](https://docs.aws.amazon.com/whitepapers/latest/saas-tenant-isolation-strategies/core-isolation-concepts.html)
