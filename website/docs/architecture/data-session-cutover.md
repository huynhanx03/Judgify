# Data and session cutover contract

This document is the activation contract between the Next.js frontend, the
same-origin reverse proxy, and the Judgify API. The frontend source implements
this target contract. Deployment must not enable it partially: identity
transport, CSRF, session projection, and proxy routing switch together.

## Browser authority boundary

- Browser REST traffic is same-origin and uses `credentials: "include"`.
- Access and refresh authority live only in `HttpOnly`, `Secure` production
  cookies. JavaScript never receives, decodes, stores, or forwards a bearer
  token.
- The server issues one signed, readable CSRF cookie. Every unsafe request
  echoes it in `X-CSRF-Token`; safe requests do not.
- `X-Correlation-ID` is one canonical UUID on every request. Response header
  and response body CID must agree.
- `Idempotency-Key` is one canonical UUID per logical command. An unknown
  outcome retains that UUID until the server returns a canonical result or an
  authoritative non-retryable rejection.
- Browser storage may contain only bounded coordinator tuples:
  purpose/attempt UUID/expiry or refresh lease owner/attempt UUID/expiry. It
  may not contain credentials, CSRF values, request bodies, users, profiles,
  or session projections.

The typed frontend paths omit any deployment prefix. If production exposes
Judgify beneath `/api`, the reverse proxy removes that prefix before routing;
feature code and service code remain unchanged.

## Success and error envelopes

The target success envelope is:

```json
{
  "data": {},
  "meta": {
    "cid": "019f6abb-8dd5-7581-8449-2b9ad77873e5"
  }
}
```

Pagination belongs in `meta.pagination`; the frontend normalizes it once to
the existing `{records, pagination}` domain shape. During the coordinated
backend migration only, the transport also accepts the current go-common
`{code,message,data,pagination,cid}` success envelope and normalizes it at the
same boundary. Feature services never branch on the envelope generation.

The target failure envelope is:

```json
{
  "error": {
    "code": "stable_machine_code",
    "params": {},
    "fields": {}
  },
  "meta": {
    "cid": "019f6abb-8dd5-7581-8449-2b9ad77873e5"
  }
}
```

Wire messages contain stable codes and bounded safe parameters, not canonical
Vietnamese UI copy.

## Anonymous pre-session

`POST /auth/pre-session`:

- requires a canonical `Idempotency-Key`;
- intentionally omits CSRF because it creates the CSRF/pre-session boundary;
- reuses an existing unexpired signed pre-session instead of rotating it;
- returns an empty success value;
- sets the signed pre-session and readable CSRF cookies.

React remounts and reloads do not spend another pre-session. A retry after an
unknown response reuses the same command UUID.

## Session issuance and refresh

Login, registration, OAuth finalization, password reset, and refresh return:

```json
{"success": true}
```

inside the approved success envelope. Their credentials are delivered only by
cookie headers. Token fields in JSON are a contract violation.

Each session-issuing command is pre-session bound, CSRF protected, and
idempotent. Refresh is single-flight in one tab and coordinated across tabs
with Web Locks when available; the bounded local-storage lease plus
BroadcastChannel is the fallback. After any issuance/refresh result, the
frontend reads `/session/me`; it never constructs identity from the command
response.

## Canonical session projection

`GET /session/me` returns exactly:

```json
{
  "user": {
    "id": "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
    "username": "jerry",
    "first_name": "Jerry",
    "last_name": "Nguyen"
  },
  "session": {
    "id": "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e18",
    "expires_at": "2026-07-28T12:00:00Z",
    "reauthenticated_until": null
  },
  "capabilities": ["contest:update", "submission:inspect"],
  "authorization_revision": 42
}
```

Optional names may be absent. `reauthenticated_until` may be absent or null.
Raw roles, wildcard Casbin rules, subjects, tokens, cookies, and profile
authority do not cross this endpoint.

A `401` means anonymous. A retryable network/server failure is a distinct
session error state and must not silently log the user out.

## Session management

- `GET /auth/sessions?limit=<1..50>&cursor=<opaque>` returns bounded safe
  session projections and an optional opaque next cursor.
- `DELETE /auth/sessions/:id` and `POST /auth/logout-all` require CSRF and one
  durable idempotency UUID and return `{success:true}`.
- Logout clears local in-memory identity only after server confirmation or a
  canonical anonymous projection.

## Checked product data added at this cutover

Runtime authority comes only from `GET /judge/runtimes`:

```json
{
  "version": 1,
  "maximum_source_code_bytes": 1048576,
  "runtimes": [{
    "runtime_key": "go1.24",
    "language": "go",
    "display_name": "Go 1.24",
    "source_filename": "main.go",
    "file_extensions": [".go"],
    "compiled": true
  }]
}
```

The browser submits
`{problem_id,runtime_key,source_code,contest_id?,contest_problem_id?}`. The two
contest fields are all-or-none, and the runtime language is resolved by the
server. `POST /submissions` requires one logical idempotency UUID and exactly
`202 Accepted`; its body is the checked durable intake snapshot, not a
fabricated submission list row.

Contest detail supplies `version` and the authoritative visible membership
projection:

```json
{
  "contest_problem_id": "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e19",
  "problem_id": "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e20",
  "display_order": 1,
  "alias": "A",
  "points": 100,
  "visible_before_start": true
}
```

Legacy `problem_ids`, when present, must be the exact ordered derivation of
that projection. Arena navigation carries both contest and membership UUIDs.
Publishing a draft requires:

- `POST /contests/:id/publish`;
- `Idempotency-Key: <logical command UUID>`;
- `If-Match: "contest-v<positive version>"`;
- body `{reason}` with 1–1024 UTF-8 bytes;
- a checked `draft -> upcoming`, `version + 1` receipt.

## Coordinated activation checklist

1. Put Next.js and the API behind the same public origin and finalize the
   reverse-proxy prefix contract.
2. Implement signed pre-session and readable signed CSRF cookies.
3. Change identity command HTTP responses to cookie-only `{success:true}`;
   remove token JSON from the browser adapter.
4. Add `/session/me` and session-management endpoints with the exact checked
   projections above.
5. Confirm every unsafe route validates Origin/Fetch Metadata and CSRF.
6. Confirm idempotent recovery returns the original committed result for the
   same command UUID.
7. Activate frontend and backend together, then run contract, type, lint,
   build, browser, and multi-tab refresh verification.
8. After all consumers and deployments use `{data,meta}`, remove the temporary
   legacy go-common success-envelope parser.

At the time this document was authored, the Go identity core still contained
browser DTOs with `access_token` fields and no matching `/auth/pre-session` or
`/session/me` route was found. Therefore the frontend session source is ready
for the target contract, but production activation remains blocked on that
backend cutover and its verification.
