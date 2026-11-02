# Judgify multi-tenancy implementation status

The implementation is not complete yet. The clean-database migration/bootstrap and representative runtime RLS checks now pass; the remaining audit items below are still open.

## Implemented so far

- Added tenant registry, membership, invitation, tenant role schemas, Community bootstrap, registration membership, creation, and yearly rename cooldown.
- Added `tenant_id` to 87 tenant-owned Ent schemas, generated Ent code, and added fresh-database migrations with tenant RLS.
- Added Ent query/mutation isolation and forced RLS for the separate `TenantRole` table, bringing the tenant role definitions under the same boundary.
- Added typed tenant context, Ent query/mutation isolation, scoped request transactions, optional tenant context for identity routes, and tenant-aware Casbin evaluation.
- Added a read-only all-membership mode for GET/HEAD; writes require one concrete tenant. Authorization in all mode checks each tenant separately.
- Added tenant-aware judge lease metadata and SQL RLS binding in the direct judge job/finalization paths. Worker startup scopes claims to active tenant IDs and each lease executes in its tenant. Platform queue overview and activation retirement bind the full registry scope for their fixed global reads; submission backlog admission temporarily widens only that count query and restores the request tenant scope before continuing.
- Added tenant-scoped L1 cache keys for tenant-owned catalogs and attribute definitions; All-scope keys are stable across membership query ordering and isolated from single-tenant keys.
- Added tenant-local natural keys and per-user sequence fences for catalogs, ranking privacy/projections, profile/reward state, notification preferences, material slugs, problem taxonomy, test-asset hashes, and tenant singleton rows. Ent code and the fresh-install index migration now agree.
- Added one profile-schema and cultivation-ranking state row when a new empty Tông môn is created; bootstrap initializes the Community ranking state under Community scope.
- Bound validated tenant scope in the Ent UnitOfWork itself so request and internal transaction paths install transaction-local RLS before their callback.
- Added optional tenant ownership to audit and outbox records. Tenant events restore concrete scope in the delivery wrapper, global events clear inherited worker scope, and audit rows use FORCE RLS with separate global-row behavior.
- Limited missing-header All scope to account-level tenant create/invitation acceptance, membership/invitation reads, and explicit read-only collection searches for problems, tags, difficulties, and material categories. Aggregated DTOs carry `tenant_id` so consumers can label mixed-tenant rows.
- Made frontend capability checks suppress every non-read action while All is selected, added an All-scope read-only notice, and show tenant names on the four mixed-tenant admin lists.
- Realtime tickets now capture the validated request tenant scope and restore it on the WebSocket request; anonymous public sockets receive the Community scope. This lets contest membership/publication authorization queries run within their tenant boundary.
- The contest lifecycle background worker now enumerates active tenants and performs status scans, transitions, and rating settlement one concrete tenant at a time.
- All four registered bounded operation workflows persist `tenant_id` both in the immutable request payload and as a first-class column on the operation, each batch receipt, and each control receipt. Composite foreign keys and a parent-scope trigger prevent cross-tenant receipts.
- Added FORCE RLS to operations, operation batches, and control receipts. Tenant request transactions only see rows matching their authorized IDs; the runner's control-plane capability is transaction-local and disabled again for each tenant-bound domain batch. Read endpoints open a scoped read transaction so their SQL receives the RLS settings.
- Added FORCE RLS to outbox events. The relay binds its own transaction-local worker capability for claim, retry, finalize, and dead-event replay operations; tenant-scoped app transactions remain restricted to their tenant, while no-scope app paths see only platform-global events.
- Added FORCE RLS to memberships and invitations: rows are visible by selected tenant or by the authenticated account's principal GUC; inserts require a tenant scope, keys are immutable, and membership/invitation reads before tenant selection use short principal-bound transactions.
- Routed outbox idempotency receipt reads through a helper that uses the current scoped transaction or opens a short tenant-bound transaction; material receipts use the same path instead of raw SQL outside the request UOW.
- Bound worker readiness and fixed aggregate durable metrics to read-only transactions scoped from tenant IDs read inside the same transaction. The collectors return aggregate values only and use transaction-local capabilities for outbox/operation control rows.
- Ran the complete clean-install chain on PostgreSQL 18 using the production `init-roles.sql`, applying all 12 migrations as `judgify_migrator` and bootstrapping Community/Admin with the same role. Removed unnecessary operation backfills after PostgreSQL proved the append-only child tables reject UPDATEs; the fresh-database migration no longer attempts them.
- Verified `judgify_api` has neither superuser nor `BYPASSRLS`, all 95 tables with `tenant_id` have FORCE RLS, all 95 RLS-enabled tables have a policy, unscoped tenant-role/membership/invitation queries reveal no tenant rows, principal scope returns only the account's membership/invitation rows, tenant scope returns only that tenant's rows, and cross-tenant writes are rejected. Temporary probe rows were removed afterward.
- `go-common` now carries a context updated by the trusted WebSocket principal resolver into subscription authorization. Contest topic authorization confirms the contest exists in that ticket tenant; lifecycle topics are tenant-scoped; notification and submission topics include tenant ID in both publishers and FE subscriptions.
- Added Owner invitation by existing account username, pending invitation listing, acceptance, membership role grant, audit, and authorization revision update.
- Added Member leave and Owner removal endpoints; both preserve at least one membership per account, and Owners cannot leave or be removed through these paths.
- Added Owner member listing and ownership transfer, preserving one active Owner while atomically swapping tenant grants. FE includes tenant selector, create/rename, invite/accept, leave/removal, transfer, and API tenant header.

## Still open

- Finish a final read-only audit of every direct SQL path and background worker/realtime consumer for transaction-local RLS binding. The focused audit of tenant-sensitive judge SQL is complete; worker readiness and durable aggregate metrics bind their required scopes. Realtime authorization tailing and ticket cleanup are global-only infrastructure; authorization invalidations remain global and contain only a revision watermark. Notification and realtime projections restore tenant scope from durable outbox metadata.
- Expand All-scope collection reads only where the route is explicitly safe and the FE has a useful mixed-tenant view. Current allowlist intentionally covers problems, tags, difficulties, and material categories; other reads still require a concrete tenant.
- Confirm object-storage/export boundaries. No general-purpose object-storage adapter or export endpoint was found; judge staging uses globally unique attempt UUID directories. Recheck when adding tenant-owned exports or external blob storage.
- Add English translation coverage across the rest of the site. A persistent language switch and English copy now cover the shared navigation, registration/OAuth profile onboarding, and tenant controls; untranslated pages fall back to Vietnamese. Date/number formatting, data-driven profile labels, and the large admin catalog remain Vietnamese.
- Seed or provision Community cultivation catalogs before public registration is enabled. Fresh database migrations create no traits/rarities/effects; admin bootstrap creates the administrator, authorization, profile, and ranking state only. No canonical game catalog seed data exists in this repository, so the first trait-offer request cannot succeed until an administrator publishes it.

## Checks run

- `cd api && GOCACHE=/private/tmp/judgify-go-cache go build ./...` — passed after tenant-scoped realtime topic changes.
- `cd ../../go-common && GOCACHE=/private/tmp/go-common-cache go build ./...` — passed after WebSocket resolver context propagation.
- `cd website && npm run typecheck && npm run build` — passed after All-scope write suppression and tenant row labels.
- `cd website && npm run typecheck && npm run build` — passed after adding the persistent language switch and English registration/OAuth onboarding/tenant copy.
- `cd api && GOCACHE=/private/tmp/judgify-go-cache go build ./...` — passed after adding outbox relay transaction scope and its RLS migration.
- `cd ../../go-common && GOCACHE=/private/tmp/go-common-cache go build ./...` — passed after the outbox RLS change.
- `cd website && npm run typecheck && npm run build` — passed after the outbox RLS change; no frontend files changed in that step.
- `git diff --check` — passed after the outbox RLS change.
- `cd api && GOCACHE=/private/tmp/judgify-go-cache go build ./...` — passed after adding tenant scope to contest lifecycle worker scans and transitions.
- `cd api && GOCACHE=/private/tmp/judgify-go-cache go build ./...` — passed after binding tenant scope to all four bounded operation executors.
- `cd ../../go-common && GOCACHE=/private/tmp/go-common-cache go build ./...` — passed after the operation worker audit.
- `cd website && npm run typecheck && npm run build` — passed after the operation worker audit.
- `git diff --check` — passed after the operation worker audit.
- Migration consistency check: 87 tenant-owned tables have 87 RLS policies and 87 composite-FK scan entries; audit has its additional RLS policy; all 33 tenant-local indexes created/dropped by the new migration match generated Ent schema and baseline index names; audit/outbox indexes and tenant edges match generated Ent schema.
- Static RLS count after the TenantRole correction: 88 tenant-owned Ent types map to 88 tenant RLS policies; the cross-cutting audit table has its separate policy.
- Added a follow-up fresh-install migration for first-class operation ownership, tenant-local idempotency, same-tenant receipt references, and RLS on all three operation control tables.
- Added a follow-up fresh-install migration for FORCE RLS on outbox events; relay operations now bind the worker capability inside each transaction.
- Added a follow-up migration for FORCE RLS on tenant memberships and invitations, principal-scoped pre-selection reads, and immutable membership/invitation identity fields.
- Outbox receipt lookups now bind tenant scope for short reads; durable metric and worker readiness SQL use read-only, tenant-scoped transactions.
- `cd api && GOCACHE=/private/tmp/judgify-go-cache go build ./...` — passed after tenant-scoped metrics, readiness, and outbox receipt changes.
- `cd api && GOCACHE=/private/tmp/judgify-go-cache go build ./...` — passed after binding the platform judge queue/retirement reads to all tenants and constraining backlog admission's temporary scope widening.
- `cd ../../go-common && GOCACHE=/private/tmp/go-common-cache go build ./...` — passed in the final build pass.
- `cd website && npm run typecheck && npm run build` — passed in the final build pass.
- `git diff --check` — passed after tenant-scoped metrics, readiness, and outbox receipt changes.
- `cd api && go run /private/tmp/validate-judgify-migrations.go` — passed; Atlas validated the complete embedded migration checksum chain, including the outbox RLS migration.
- `docker run postgres:18-alpine` disposable database + `deploy/postgres/init-roles.sql` — passed; migration runner applied all 11 migrations as `judgify_migrator` (non-superuser, `NOBYPASSRLS`), then returned `already_current` on rerun.
- `cmd/bootstrap-admin` on the migrated database — passed; initialized one `Judgify Community`, one bootstrap Owner membership, Owner/Member role definitions, system authorization, profile schema, ranking state, and administrator credential.
- Runtime RLS probe as `judgify_api` — passed; no-scope reads returned zero tenant role rows, each of two tenant scopes returned only its own row, and a cross-tenant insert was denied. Probe rows were removed from the disposable database.
- Runtime principal/tenant RLS probe as `judgify_api` — passed; pre-selection reads expose only memberships and invitations for the authenticated account, tenant scope exposes only that tenant, and cross-tenant invitation insertion is denied. Probe rows were removed afterward.
- Fresh database catalog audit — passed: all 95 tables with `tenant_id` are FORCE RLS, every RLS-enabled table has a policy, and `judgify_api`, `judgify_worker`, and `judgify_migrator` are non-superuser/non-BYPASSRLS roles.
- `git diff --check` — passed after the latest backend and frontend changes.
- No tests were added or run; no commit was created.
