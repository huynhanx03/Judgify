# Reusable Scoped Casbin Design for go-common and Judgify

**Date:** 2026-10-18  
**Status:** Draft for written review; no implementation until an execution plan is approved  
**Scope:** Reusable authorization model and Ent persistence adapter across Go services

## 1. Purpose

Prepare Judgify for multiple application instances and multiple tenants while building a reusable Casbin capability in `go-common`. The shared library owns one canonical authorization model and reusable Ent persistence building blocks. Each application owns its role catalog, actual policy data, scope/resource meaning, authorization rules, audit semantics, and schema migration lifecycle.

The design extends the authorization parts of `2026-07-16-identity-authentication-authorization-design.md`. Authentication and unrelated identity decisions in that document are unchanged.

## 2. Agreed design direction

- There is **one shared, fixed Casbin model** maintained by `go-common`. Applications do not upload or assemble arbitrary model text at runtime.
- Each project stores its own policy rows and chooses which roles/permissions exist. A shared model standardizes how those rows are interpreted; it does not make every project's roles identical.
- `go-common` provides reusable Ent schema building blocks and a Casbin Ent adapter. It does not own a consuming application's Ent client, generated code, migration runner, or database lifecycle.
- Judgify owns the authoritative authorization data and validates each proposed change against its own permission catalog, protected-role rules, actor authority, and domain invariants.
- Database state is authoritative. Runtime enforcers use immutable snapshots; a policy write publishes a new local snapshot only after its transaction commits.
- Tenant and narrower scopes are explicit inputs to authorization. A request that needs a scope but has no trusted scope fails closed.
- Authorization uses current server-side policy, not role claims embedded in access tokens.

## 3. Ownership boundaries

### `go-common` owns

- The stable authorization request, decision, scope, and resource-reference contracts.
- The single versioned Casbin model and model/policy structural validation.
- Runtime snapshot construction, atomic replacement, revision tracking, and bounded snapshot caching primitives.
- Reusable Ent schema mixins or schema definitions for canonical Casbin policy rows, including `ptype` and `v0` through `v5` with stable indexes/constraints.
- An Ent adapter that loads and saves the canonical row format and supports filtered loading by scope without creating tables at runtime.
- Generic revision notification interfaces and cache invalidation hooks, if the consuming app chooses to connect them.

### Each application owns

- Its role and permission catalog, role assignment rules, system-role protections, and policy authoring UX/API.
- Its trusted resolution of tenant/scope/resource identifiers from persisted domain state.
- Domain-level authorization validation and audit meaning.
- The transaction that writes policy rows, revision, audit, and outbox records.
- Ent client generation, migration files, migration application, and database privileges.
- The choice of deployment notification transport and reconciliation interval.

The shared schema is a reusable source definition, not a shared database migration. Judgify and future consumers each generate their own Ent code and migrations, pin a compatible `go-common` release, and explicitly review generated schema changes.

## 4. Shared authorization semantics

The first stable shared model is a fixed hybrid model with:

1. **RBAC scoped to a domain:** a principal can receive a role at a named scope; a permission grants a role a resource/action pair at a scope.
2. **Explicit scope hierarchy support:** the model/runtime can evaluate a scope and its configured ancestor relationships, subject to the inheritance rule selected for the application.
3. **Object relationship support:** the shared contract can carry a resource reference and trusted resource attributes/relations so an application can express ownership or collaboration checks without pretending that tenant scope alone implies ownership.
4. **Deny by default:** no matching allow rule means denied. Any explicit deny behavior, if later supported, must have documented precedence and cannot be inferred from missing rows.

The model is shared; policy rows are data. Apps cannot add matcher code, functions, or arbitrary expressions through an admin API. The runtime validates row arity, value bounds, known policy types, and scope requirements before publication.

### Request contract (conceptual)

An authorization request contains:

| Field | Meaning |
|---|---|
| Principal | Stable authenticated subject identifier, namespaced so user IDs cannot collide with role names |
| Scope | Trusted scope kind and opaque scope ID, such as tenant or project; resolved by the app from persisted data |
| Resource | Stable resource type and ID, not an untrusted client-supplied ownership claim |
| Action | Canonical action string from the app's permission catalog |
| Attributes/relations | Optional bounded, typed facts resolved by the application for ABAC/ReBAC-style checks |

The contract is transport-neutral. HTTP middleware may map a route to an action, while application services still perform object-sensitive checks and state validation. A request with an unknown scope kind, malformed identifier, missing required resource fact, or untrusted attribute is rejected or denied; it is never treated as a wildcard.

### Role and policy data

- System roles and tenant-defined roles are namespaced distinctly and cannot shadow one another.
- A user-role assignment includes an explicit scope; global system roles use an explicit system scope rather than an empty string or wildcard.
- Permission rows name a role, scope or scope class, resource type, and action.
- Grouping/role rows and permission rows use one canonical schema. Their semantic positions are fixed by the shared model and documented constants, not guessed by each service.
- Wildcards are only allowed in the specific policy fields documented by the shared model and validated by the app. Scope IDs themselves are never wildcard values.

### Scope inheritance (review decision)

The intended scalable default is **downward inheritance**: a role assigned at tenant scope may apply to child scopes in that tenant; a role assigned at a project scope does not grant access to its parent tenant or sibling projects. The app supplies or persists the hierarchy, and the model only follows validated ancestry. This rule is proposed but not yet confirmed; see §11.

## 5. Object ownership and ABAC/ReBAC boundary

Tenant scope answers “which authorization boundary does this request belong to?” It does not answer “does this principal own this particular object?”

The shared request/model should make room for relationships and bounded attributes. Judgify remains responsible for resolving facts such as `submission.owner_id`, contest membership, or resource state from the database. Client-provided owner IDs are never authoritative. Application services use the shared authorizer for reusable policy decisions and retain domain checks for invariants that are not policy (for example, whether a submission is editable after judging starts).

The initial implementation should not turn every domain relationship into a Casbin row by default. It should define the extension point and demonstrate one concrete ownership-sensitive Judgify use case before declaring broad ABAC/ReBAC behavior stable.

## 6. Ent schema and adapter

### Canonical row shape

Use the existing Casbin-compatible row shape (`ptype`, `v0` … `v5`) with a composite uniqueness constraint over the rule fields, matching Casbin's adapter expectations. The exact indexes and nullable/empty-field semantics must be verified against the pinned Casbin and Ent adapter versions before implementation.

`go-common` may expose Ent mixins/helpers that a service's `ent.Schema` embeds. Because Ent generation runs in the consumer repository, generated code remains local to that service. The reusable adapter accepts the consumer's Ent client/driver and context; it does not create an independent database connection or run DDL.

### Adapter responsibilities

- Load complete policy sets or bounded scope-filtered sets.
- Preserve the exact `ptype` and value ordering expected by the shared model.
- Support the standard Casbin adapter contract where useful, while exposing a context-aware, filtered load path for production runtime construction.
- Never auto-create or alter schema in production.
- Bound row counts and reject malformed rows before they enter a runtime snapshot.

### Transactional writes

Generic adapter convenience writes may exist for tools or simple applications, but Judgify's production policy mutation path must use its transaction-bound Ent client/repository. The adapter must therefore expose a transaction-compatible persistence seam (or a pure row encoder/decoder) that does not silently open and commit a separate transaction. Judgify writes policy rows, revision, audit, and outbox in one transaction, then publishes the committed revision locally.

## 7. Runtime, multi-instance updates, and cache

PostgreSQL remains the source of truth; enforcer memory is a cache. Each process builds a validated immutable snapshot from a committed database revision and atomically swaps it into service. Readers see either the prior complete snapshot or the next complete snapshot.

Deleting or clearing a cache only affects the process where that operation runs. Therefore a multi-instance deployment needs a way for other instances to learn that a revision changed. The proposed design is:

1. The app commits policy rows, a monotonically increasing revision, audit, and an outbox event atomically.
2. The outbox event carries a scope identifier and revision, not the full policy payload.
3. Each instance receiving the event checks whether its local revision is behind, reloads the latest rows from Ent, validates a complete snapshot, and swaps it atomically.
4. A periodic reconciler/revision check remains as recovery for missed or delayed notifications.

The event is an invalidation signal, not a second policy database or a source of truth. If initial implementation avoids a notification transport, revision polling is an acceptable first mode with a documented maximum stale window. No claim of immediate cross-instance invalidation may be made from local cache deletion alone.

For many tenants, snapshots should be partitionable and bounded by tenant/scope so one process does not eagerly load every tenant's policy. The cache needs explicit size/eviction and in-flight load controls. System/global policy composition must be deterministic and included in the effective revision.

## 8. Judgify migration

1. Keep Judgify's permission catalog, role lifecycle, semantic validation, audit/outbox, revision, and reconciliation logic in Judgify.
2. Adopt the common schema definition and adapter without transferring migration ownership.
3. Map the existing single-tenant rows into the shared model's explicit system-scope representation; do not interpret absent scope as a wildcard.
4. Add tenant and narrower scope identifiers to assignments/policies only as the tenant domain model is introduced. Until then, use one explicit default scope.
5. Update route and service authorization requests to include trusted scope/resource data as those domain boundaries become available.
6. Keep current runtime behavior compatible during migration using a versioned model/schema transition and verify old/new decisions against a policy corpus before switching.

No destructive data rewrite or permission broadening is allowed as an implicit migration step. Any legacy row that cannot be mapped unambiguously blocks rollout and must be resolved explicitly.

## 9. Compatibility and release rules

- Existing vendor-neutral `authorization.Request` fields may require an additive evolution strategy or a new scoped request API; avoid a breaking change to current consumers without a migration path.
- The Casbin-specific package may expose Casbin types internally, but the core authorization contract remains vendor-neutral.
- Shared model changes are versioned and have compatibility notes because all consumers interpret the same rows.
- Experimental ABAC/ReBAC facilities remain marked experimental until behavior, safety limits, and benchmarks are established.
- The common schema helper and adapter may evolve independently only where the persisted row format remains compatible.

## 10. Non-goals

- Centralizing each application's role names or permission catalog in `go-common`.
- Storing per-tenant executable Casbin models or admin-authored matcher expressions.
- Replicating policy state through an eventually consistent second database.
- Having `go-common` run migrations or own application database credentials.
- Replacing app-specific audit, authorization validation, resource lookup, or business-state checks.
- Treating domain scopes as a substitute for object ownership checks.

## 11. Decisions requested before planning

1. Confirm or change the proposed downward scope-inheritance rule: tenant assignments flow to child scopes; child assignments never flow upward or sideways.
2. Confirm that cross-instance updates should use the existing durable outbox as an invalidation signal, with periodic reconciliation as fallback; local cache clearing alone cannot update other instances.
3. Confirm the delivery boundary: first implement the shared fixed scoped RBAC model and schema/adapter, then add the ABAC/ReBAC ownership extension with a concrete Judgify use case, rather than making generic relationship authorization stable before a real use case is defined.

After these decisions are reviewed, a separate implementation plan will name the exact go-common and Judgify package/API changes, migration strategy, compatibility path, and verification steps.

## References

- Existing Judgify authorization design: `2026-07-16-identity-authentication-authorization-design.md`
- Existing go-common library design: `2026-07-16-go-common-production-library-design.md`
- Ent schema mixins: https://entgo.io/docs/schema-mixin/
- Ent code generation: https://entgo.io/docs/code-gen/
- Ent versioned migrations: https://entgo.io/docs/versioned/intro/
- Casbin RBAC with domains: https://casbin.org/docs/rbac-with-domains/
- Casbin ABAC: https://casbin.org/docs/abac/
- Casbin RBAC with resource roles: https://casbin.org/docs/rbac-with-resource-roles/
