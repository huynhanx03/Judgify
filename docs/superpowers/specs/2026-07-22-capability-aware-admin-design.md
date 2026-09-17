# Capability-Aware Admin Design

**Status:** Approved on 2026-07-22

## Goal

Make every administrative entry point, route, navigation item, dashboard read,
and mutation control reflect the authenticated principal's effective,
code-owned authorization capabilities without exposing Casbin implementation
details or turning transient failures into logout.

## Authorization contract

`GET /authorization/me` is protected by session authentication only. It returns:

```json
{
  "revision": 42,
  "capabilities": [
    { "resource": "problem", "actions": ["create", "read", "update"] }
  ]
}
```

The identity authorization service derives this projection from the same
immutable `authorizationView` used by the live Casbin runtime. Multiple roles
are unioned. Wildcard system policies are expanded only across
`permissions.Catalog()`. Unknown resources, unknown actions, role keys, raw
policy rows, wildcard values, and internal subjects never cross the API.
Resources and actions retain catalog order.

The effective projection is precomputed when a revision-fenced authorization
view is built. Publishing or reconciling a newer authorization revision swaps
the whole view atomically, invalidating every prior projection without a TTL or
a second cache. Reads fail closed when the runtime and view revisions differ or
authorization health is unavailable.

`authorization:manage` permits entry to the role/authorization management
surface and policy reads; exact `role:create`, `role:update`, and `role:delete`
remain required for role metadata mutations. Role deletion continues to require
both `role:delete` and `authorization:manage`.

## Frontend session and capability state

The auth provider keeps the access credential, profile, and effective
capability snapshot only in memory. It exposes pure `can`, `canAny`, and
`hasAnyCapability` helpers plus a capability refresh operation. A profile read
establishes authentication; capability loading has independent
`loading | ready | error` state so the public product is not blocked by an
authorization read outage.

Only a final HTTP `401` is authentication-invalid and may clear the in-memory
session. Refresh network errors, malformed gateway responses, and HTTP 5xx
remain recoverable errors. They preserve the last session state and present a
retry affordance. Login and registration establish the profile first, then
load capabilities; a capability outage does not misreport valid credentials as
invalid.

## Admin policy

A centralized policy catalog maps routes, navigation items, and dashboard cards
to code-owned resource/action pairs:

- `/admin` requires at least one effective administrative capability;
- list routes require their resource `read` capability;
- create and edit routes require exact `create` and `update` capabilities;
- `/admin/roles` accepts `role:read` or `authorization:manage`;
- nested material categories use `material_category`, not `material`.

An authenticated principal with no matching route capability sees an explicit
403 state inside the themed admin surface. The client never redirects that
principal to login and never logs them out. Capability load failure shows a
separate unavailable state with retry. Sidebar sections with no visible items
are omitted. The dashboard calls only APIs the principal can read and renders
only the corresponding cards.

Reusable capability-aware action components hide create, update, delete,
publish, role-assignment, and policy-management controls. Backend permission
middleware remains authoritative; UI guards are usability and least-surprise
controls, not a security boundary.

## Verification

Backend tests cover role union, duplicate elimination, wildcard expansion,
unknown-policy filtering, stable ordering, revision swap invalidation,
unavailable-view fail-closed behavior, unauthenticated handler rejection, and
authentication-only route mounting. Frontend tests cover capability lookup,
route/nav/card filtering, explicit 403 and retry states, 401-only session
clearing, transient refresh preservation, and guarded action contracts.

No contest or query repository is changed. No commit is created.
