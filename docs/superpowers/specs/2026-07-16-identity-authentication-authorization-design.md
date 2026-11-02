# Judgify Identity, Authentication, and Authorization Design

**Date:** 2026-07-16

**Status:** Draft for written review  

**Deployment model:** Single tenant, single host initially

**Primary stack:** Go, Gin, Ent, PostgreSQL, Casbin v3, Ent Casbin adapter, Next.js

## 1. Purpose

This document defines the production identity and access-control architecture for Judgify. It replaces stateless refresh tokens, transport-coupled JWT parsing, and row-by-row Casbin mutations with a session-based authentication system and an atomic, revisioned authorization policy.

The design has four invariants:

1. A refresh token can never authorize an application request.
2. A previously consumed refresh token revokes its whole token family unless it is replaying the exact short-lived idempotent rotation receipt; the recovery exception returns the same replacement credential and cannot create a second branch.
3. Authorization readers observe either a complete old policy or a complete new policy, never a partially updated policy.
4. Suspending a user, revoking a session, or changing authorization policy becomes effective for HTTP and WebSocket traffic without issuing a new access token.

## 2. Resolved decisions

- Judgify is single tenant. The Casbin model has no tenant/domain column.
- Browser authentication uses secure cookies. Tokens are not stored in `localStorage` or returned in normal JSON responses.
- Access tokens are short-lived signed JWTs. Refresh tokens are opaque, rotating, single-use secrets whose hashes are stored in PostgreSQL.
- Cookie-authenticated unsafe HTTP methods require CSRF protection and an allowed `Origin`.
- `go-common/pkg/security/authentication` owns reusable authentication contracts and JWT primitives.
- `go-common/pkg/security/authorization` owns reusable authorization contracts and the concurrency-safe Casbin runtime.
- Gin middleware remains in `go-common/pkg/common/http/middlewares`; it adapts HTTP credentials to the reusable security contracts.
- Judgify Identity owns users, credentials, sessions, refresh-token state, roles, canonical Casbin rows, authorization revisions, security/audit semantics, and the associated outbox events. The shared audit rows themselves are owned by `api/internal/platform/audit`.
- Ent migrations exclusively own schema lifecycle.
- The Ent Casbin adapter remains the policy-loading adapter, with a minimal pinned patch that supports `SkipSchemaCreate`.
- Admin policy mutations use Judgify's Ent transaction, not Casbin AutoSave and not a sequence of individual Casbin mutations.
- Roles are not authoritative JWT claims. Authorization always uses the current policy runtime.
- Route middleware performs coarse authorization. Application services perform object ownership and state checks.
- Correlation IDs are supplied by go-common. Judgify enriches logs with user, session, policy revision, and audit identifiers without hardcoding Judgify fields into go-common.

## 3. Current problems being corrected

The existing implementation has these production blockers:

- JWT claims contain a token type, but the authentication middleware does not require `access`; a valid refresh JWT can pass protected routes.
- The refresh endpoint is protected by the generic access middleware and ignores its empty request body, so it issues a new access token without rotating or consuming a refresh credential.
- Access and refresh tokens are returned to the browser as JSON and are persisted by the frontend in `localStorage`.
- Refresh tokens are stateless: there is no rotation, revocation, device/session inventory, or reuse detection.
- Deleted, disabled, or suspended users can remain authorized until JWT expiry.
- `SetUserRoles`, `SetRolePolicies`, and role deletion remove policy rows and then add replacements one at a time. Concurrent requests can observe partial state, and an intermediate error can permanently truncate access.
- Casbin's Ent adapter creates its schema at application startup, requiring runtime DDL privileges and bypassing migration control.
- Role/catalog writes and Casbin policy writes use different clients and transactions.
- Authorization is route-oriented only; submission source ownership, hidden tests, and other object-sensitive operations require application-level decisions.

## 4. Ownership and package boundaries

### 4.1 go-common reusable core

```text
pkg/security/
├── authentication/
│   ├── principal.go
│   ├── purpose.go
│   ├── verifier.go
│   ├── errors.go
│   └── jwt/
│       ├── claims.go
│       ├── issuer.go
│       ├── verifier.go
│       └── keyset.go
├── authorization/
│   ├── authorizer.go
│   ├── request.go
│   ├── decision.go
│   ├── errors.go
│   └── casbin/
│       ├── runtime.go
│       ├── snapshot.go
│       └── validator.go
└── password/
    ├── bcrypt.go
    └── policy.go
```

The core packages do not import Gin or Judgify domain packages. They do not know about submissions, contests, superadmins, or Ent schemas.

The reusable authentication contract is:

```go
type TokenPurpose string

const (
    PurposeAccess  TokenPurpose = "access"
    PurposeRefresh TokenPurpose = "refresh"
)

type Principal struct {
    Authenticated bool
    Subject   string
    UserID    string
    SessionID string
    Username  string
    TokenID   string
    AuthenticatedUntil time.Time
}

type Verifier interface {
    Verify(
        ctx context.Context,
        rawToken string,
        expectedPurpose TokenPurpose,
    ) (Principal, error)
}
```

This is the canonical go-common type, not a Judgify duplicate. A successfully verified JWT/live-session credential always returns `Authenticated=true` with a nonempty stable subject and valid `AuthenticatedUntil`. `Authenticated=false` is the explicit zero anonymous principal: subject, user, session, username, token ID, and authenticated-until fields must all be empty. Only an optional transport resolver may construct it when no credential is present and that endpoint permits anonymous access. A presented malformed, expired, revoked, or wrong-purpose credential is an error and is never downgraded to anonymous.

The reusable authorization contract is:

```go
type Request struct {
    Principal authentication.Principal
    Resource  string
    Action    string
}

type Decision struct {
    Allowed  bool
    Reason   string
    Revision int64
}

type Authorizer interface {
    Authorize(ctx context.Context, request Request) (Decision, error)
}
```

The Casbin runtime hides `casbin.IEnforcer` from consumers. It builds immutable policy snapshots, validates candidates, atomically swaps the active snapshot, tracks desired/applied revisions, and fails closed while the runtime is stale.

### 4.2 Transport adapters

- `pkg/common/http/middlewares/authentication.go` extracts bearer or cookie credentials, calls an authentication verifier with `PurposeAccess`, and attaches the typed principal to context.
- `pkg/common/http/middlewares/authorization.go` maps a route resource/action to the reusable `Authorizer`.
- `pkg/common/grpc/interceptors/authentication.go` and future gRPC authorization interceptors use the same contracts.
- `pkg/common/websocket` accepts a verified principal from an upgrade authentication hook; it does not parse Judgify tokens itself.

Optional authentication is anonymous only when no credential is present. A malformed, expired, revoked, or wrong-purpose credential returns an authentication error instead of silently becoming anonymous.

### 4.3 Judgify Identity

Judgify Identity owns:

- user status and profile identity;
- password and federated credentials;
- authentication sessions and refresh-token families;
- login, logout, refresh, password reset, and account security workflows;
- role catalog and user-role assignments;
- canonical `casbin_rules` rows;
- authorization revision and reconciliation;
- system-role and superadmin invariants;
- security/authorization audit semantics and transactional outbox events.

No other Judgify module writes Identity tables directly. Identity writes the shared `audit_log` only through the platform transaction-aware appender.

## 5. Authentication data model

All identifiers are UUIDs unless otherwise stated. Time values are UTC timestamps.

### 5.1 User security state

`users` gains explicit security state:

| Column | Meaning |
|---|---|
| `status` | `active`, `suspended`, or `deleted` |
| `security_version` | Monotonic integer incremented on global session invalidation or security-sensitive account change |
| `suspended_at` | Suspension time when applicable |
| `suspension_reason` | Administrative reason, excluded from public user responses |

Soft deletion never leaves an authenticatable user. Only `active` users can create or use sessions.

### 5.2 Authentication sessions

`auth_sessions` represents a browser/device or API-client session:

| Column | Meaning |
|---|---|
| `id` | Session ID included in access JWT `sid` |
| `user_id` | Session owner |
| `family_id` | Refresh-token family identifier |
| `status` | `active`, `revoked`, or `expired` |
| `user_security_version` | User security version captured at creation |
| `created_at` / `last_seen_at` | Lifecycle timestamps |
| `last_reauthenticated_at` | Last completed password/provider step-up, distinct from token refresh |
| `idle_expires_at` | Sliding inactivity limit, 30 days by default |
| `absolute_expires_at` | Non-extendable lifetime, 90 days by default |
| `revoked_at` / `revoke_reason` | Revocation metadata |
| `ip_hash` | HMAC of normalized client IP for security history, not raw IP |
| `user_agent` | Length-limited, control-character-cleaned display value |
| `created_cid` | Correlation ID of the session-creation flow |

The session row has indexes on `(user_id, status)`, `family_id`, and expiry timestamps.

### 5.3 Rotating refresh tokens

`auth_refresh_tokens` stores one row per issued opaque refresh token:

| Column | Meaning |
|---|---|
| `id` | Internal token record ID |
| `session_id` | Parent authentication session |
| `token_hash` | HMAC-SHA-256 of the random token with a server-side pepper |
| `parent_id` | Previous token in the rotation chain |
| `status` | `issued`, `consumed`, or `revoked` |
| `issued_at` / `expires_at` | Validity window |
| `consumed_at` | First successful rotation time |
| `replaced_by_id` | Token issued by the successful rotation |
| `revoked_at` | Revocation time |

The client receives at least 256 bits of cryptographically random entropy encoded using base64url without padding. The database never stores the raw token. `token_hash` is unique.

`auth_refresh_receipts` makes the commit/HTTP-response boundary recoverable without weakening single-use rotation:

| Column | Meaning |
|---|---|
| `id` | Receipt UUID |
| `session_id` / `presented_token_id` | Rotation identity |
| `idempotency_key` | Client-generated UUID retained until success |
| `replacement_token_id` | Exact replacement created by this rotation |
| `recovery_ciphertext` / `key_id` | AEAD-encrypted replacement token and CSRF nonce |
| `expires_at` | Recovery window, two minutes by default |
| `recovered_at` | Optional recovery telemetry |

The unique key is `(session_id, presented_token_id, idempotency_key)`. Associated data binds ciphertext to that tuple and replacement ID; the encryption key is held outside PostgreSQL. The ciphertext is never logged, is erased after the bounded recovery/retention window, and is not an alternative long-lived token store.

The browser treats the idempotency UUID as the identity of one logical rotation, not one HTTP attempt. The initial refresh request, a retry after an unknown network outcome, and recovery after a tab/process crash must present the same UUID while the receipt can still be recovered. The non-secret UUID may live only in the bounded cross-tab coordinator record described by the frontend design; it is cleared after a validated success or receipt-window expiry, never merely because a request timed out.

Anonymous commands that establish a session use the same recoverable issuance primitive. `auth_session_issuance_receipts` binds purpose, request/completion-ticket identity, a keyed canonical request fingerprint where applicable, client idempotency UUID, subject/pre-generated session/refresh IDs, and a short-lived AEAD ciphertext containing the exact raw refresh token and CSRF nonce. Password login additionally binds the verified user ID and the stable anonymous pre-session identity carried by the signed CSRF bootstrap; its unique command key is `(purpose = password_login, user_id, pre_session_id, idempotency_key)`. Registration uses the pre-session ID as its stable request identity plus an HMAC-SHA-256 fingerprint over canonical bounded payload bytes—including normalized recovery email, profile, and immutable onboarding trait-offer facts—so stored idempotency metadata is not an offline dictionary for PII. New-user OAuth finalize binds the same applicable onboarding facts through its completion ticket. OAuth login without onboarding has no trait offer. Their unique key is `(purpose, request_or_ticket_id, idempotency_key)`. The table has matching partial unique constraints, and AEAD associated data covers the complete applicable tuple, request fingerprint, offer binding when present, and subject/session/refresh IDs. Fingerprint/receipt keys are versioned and rotated under the secrets policy.

The access JWT is signed for the pre-generated session before the first transaction consumes a one-time ticket or commits an account/session. An exact retry re-performs the flow's credential/ticket proof, decrypts the committed receipt, verifies the session remains active, re-signs an access JWT, and returns the same refresh/CSRF bytes; it never creates a second session. For password login, the idempotency UUID or pre-session identity is never sufficient authentication: every initial/recovery attempt must verify the password again before receipt lookup/result release. Receipt keys/ciphertext have the same secret-handling and bounded-erasure rules as refresh receipts. Password login, registration, and OAuth finalize share this helper rather than implementing incompatible commit/response recovery.

### 5.4 Password recovery

Password accounts require one bounded normalized recovery email during onboarding, but email is never the login/account identity key. `user_recovery_contacts` stores user ID, kind `email`, HMAC lookup/fingerprint, AEAD-encrypted normalized address with key ID, `pending|verified|revoked` status, optimistic version, and lifecycle timestamps. The launch supports at most one current verified email plus one pending replacement per user. A verified email asserted by a supported OAuth provider may seed the contact only with explicit onboarding/user consent; provider email changes never silently replace it.

`recovery_contact_verification_tokens` stores only a one-time token HMAC, contact/version, purpose, expiry, consumed/superseded state, and attempt metadata. Registration/contact-change commits the pending contact, verification token, leased security-email delivery, audit, and outbox atomically; network email remains asynchronous. Verification locks the token/contact, checks the exact pending version, marks the contact verified, revokes the previous contact when replacing one, consumes the token, and writes audit/outbox in one transaction. Adding/replacing/removing a recovery contact from an active account requires CSRF and recent reauthentication. Resend is idempotent/rate-limited and supersedes the older verification intent without logging an address or raw secret.

Password-reset credentials use the same one-time-secret principles in a dedicated `password_reset_tokens` table: only a token hash is canonical, tokens expire after 15 minutes, consumption is transactional, and a successful password reset increments `users.security_version` and revokes all active sessions.

Forgot-password resolves only the user's current verified, non-revoked recovery contact. The request transaction creates the reset token and a leased `security_email_deliveries` row with a stable provider idempotency key and a short-lived AEAD-encrypted transport payload containing the recipient snapshot and raw reset credential. The same delivery model handles recovery-contact verification with a distinct purpose. Before every send or retry, an API-owned bounded worker transactionally verifies that the delivery intent and linked token/contact version are still active, unexpired, unconsumed, and latest/non-superseded for that purpose. An expired, consumed, revoked, replaced, or superseded intent moves the delivery to terminal `cancelled` and erases ciphertext without calling the provider. Otherwise the worker decrypts only while sending through the existing email HTTP adapter, retries with backoff, records sent/dead state, and erases ciphertext after send or expiry. Network email is never sent inside the request transaction. A provider timeout may deliver the same active link more than once, but provider idempotency and the one-time token prevent multiple security effects.

Delivery rows expose only low-cardinality operational state: kind, ready/leased/sent/dead/cancelled status, attempt count, next-attempt time, lease expiry, created/sent time, and last bounded error class. Queue depth, oldest-ready age, attempt outcome, request-to-sent latency, lease recovery, dead/cancelled deliveries, and ciphertext-erasure failures are measured without recipient, token, delivery, or provider-message identifiers as labels.

Forgot-password always returns the same catalog code and comparable timing whether the account or verified contact exists. IP/account throttles, delivery quotas, and token replacement rules are enforced. Raw verification/reset credentials, encrypted payloads, recipient addresses/fingerprints, and provider bodies never appear in logs; local development uses an explicit test-only mail sink, never a logged token.

OAuth state and temporary tickets are one-time, purpose-bound, short-lived, and stored as hashes. The provider callback consumes state atomically but creates only a completion ticket; it never creates a user/session or links an identity. OAuth identity linking requires an already authenticated active session, a ticket bound to that session and `link` purpose, recent reauthentication, and a fresh provider-identity uniqueness check at final commit.

### 5.5 Profile attribute schema and OAuth identities

The existing attribute-definition feature remains a profile-data capability; it is not Casbin input and does not introduce ABAC. `attribute_definitions` stores stable identity: immutable normalized key, `kind` (`system` or `custom`), independent `required_on_onboarding` boolean, lifecycle, `draft_revision_id`, `active_revision_id`, and optimistic version. The built-in `first_name`, `last_name`, `gender`, and `birthday` definitions have `kind = system`; their code-owned required flags are set independently as appropriate. System definitions cannot be renamed, deleted, archived, or changed to an incompatible type, and a code-required definition cannot be disabled for onboarding.

`attribute_definition_revisions` is append-only and contains the closed data type (`string`, `number`, `boolean`, or `date`), admin-authored display label/description as domain content, visibility (`public`, `private`, or `admin`), user-editable flag, and bounded typed validation specification. Publishing atomically advances the active revision only after validation/migration succeeds.

`user_attribute_values` is unique by `(user_id, attribute_id, definition_revision_id)` and stores a canonical typed JSON value plus source-value version. Ordinary reads join the definition's active revision. Every write validates that revision and expected value version. Ordinary users can edit only user-editable fields; public profile mapping returns only public fields and still respects account privacy. Admin reads/edits require separate capabilities and audit.

A schema change with existing values creates a durable migration operation. Bounded batches write shadow values for the candidate revision with the source version they converted. Before the short pointer switch, the operation locks that definition, reconciles values changed during backfill, verifies every current value has a valid candidate, then advances `active_revision_id`; old revisions/values remain for rollback. Legacy text-value rows are backfilled into revision 1, and invalid values are quarantined for admin resolution rather than coerced silently.

The migration command captures the definition optimistic version and source active revision at preview time. Resume is idempotent; a changed definition, candidate revision, or source-value version is a conflict, not an implicit overwrite. The protected system definitions have a code-owned compatibility matrix: their key/kind cannot change, they cannot be archived, code-required `required_on_onboarding` flags cannot be cleared, and an incompatible type/validation revision cannot be published even when every current row happens to convert. Activation, audit, and outbox commit atomically; a crash before activation leaves ordinary reads on the old revision, and a crash after activation is recovered from the committed pointer.

`oauth_identities` is unique by `(provider, provider_subject)` and records user, provider metadata needed for display, and timestamps without storing provider access tokens unless a separately approved feature requires them. Link/unlink requires recent reauthentication, and unlink cannot remove the account's final usable credential. Provider email is profile metadata, never the immutable identity key.

## 6. Access JWT contract

The JWT implementation initially retains the existing RSA key type and strictly enforces RS256. Algorithm confusion is rejected.

Required claims:

| Claim | Value |
|---|---|
| `iss` | Configured Judgify issuer |
| `aud` | Configured browser/API audience |
| `sub` | Immutable user UUID |
| `sid` | Authentication session UUID |
| `jti` | Unique access-token UUID |
| `typ` | Exactly `access` |
| `iat` / `nbf` / `exp` | Standard temporal claims |
| `auth_time` | Last completed primary/step-up authentication time for this session |
| `username` | Display convenience only, never authorization authority |
| `security_version` | Version captured from the active session/user |

Access-token lifetime is 15 minutes by default. Verification validates signature, configured issuer and audience, `kid`, purpose, expiry, not-before, issued-at, and a maximum 60-second clock skew. Public keys are selected from a configured key set; issuance uses one active private key and includes its `kid`.

Roles and permissions are deliberately absent. A role change takes effect through the authorization runtime without waiting for token expiry. Sensitive commands never trust `auth_time` alone: they load `auth_sessions.last_reauthenticated_at` and require it to fall within the configured ten-minute step-up window.

Judgify wraps cryptographic verification with a session authenticator. It resolves the session and user security state through a bounded local cache backed by PostgreSQL and sets `Principal.AuthenticatedUntil` to the earliest of access-token expiry, idle session expiry, and absolute session expiry. Admin suspension, logout, password change, and session revocation synchronously invalidate affected cache entries and close affected WebSocket connections. Cache misses and restarts resolve from PostgreSQL. A cache entry never outlives the access token and has a maximum 30-second defensive TTL.

## 7. Browser cookie and CSRF contract

### 7.1 Cookies

Production browser cookies are:

| Cookie | Properties |
|---|---|
| `__Host-judgify_access` | JWT; `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, no `Domain` |
| `__Secure-judgify_refresh` | Opaque refresh token; `HttpOnly`, `Secure`, `SameSite=Strict`, refresh endpoint path only |
| `judgify_csrf` | Signed anonymous-pre-session or session-bound CSRF value; `Secure`, `SameSite=Strict`, `Path=/`, readable by the frontend |
| `__Host-judgify_oauth_state` | Short-lived browser-bound OAuth state nonce; `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/` |
| `__Secure-judgify_oauth_completion` | One-time completion ticket; `HttpOnly`, `Secure`, `SameSite=Strict`, finalize-endpoint path only |

Development may disable `Secure` only through an explicit development environment setting. Production validation refuses to start with insecure cookie settings.

Login, same-origin OAuth finalize, and refresh responses set authentication cookies and return session/user metadata, not access or refresh tokens. Logout expires authentication and temporary OAuth cookies regardless of whether the server-side revoke succeeds, while reporting the server error with its CID.

### 7.2 CSRF

Cookie-authenticated `POST`, `PUT`, `PATCH`, and `DELETE` requests require:

1. an exact allowed `Origin`;
2. `X-CSRF-Token` equal to the signed, session-bound CSRF cookie value;
3. a currently active session.

The refresh endpoint follows the same rule. `SameSite` is defense in depth, not the only CSRF control.

Every same-origin browser endpoint that creates or changes authentication cookies—including login, registration, OAuth initiation/finalize, refresh, and password-reset completion—also requires exact Origin validation and same-origin Fetch Metadata. Login and registration use a signed anonymous pre-session CSRF nonce: a bootstrap endpoint sets the readable nonce cookie, and the command must echo it in the CSRF header before any authenticated session exists. Its signed claims contain a server-generated stable `pre_session_id`, purpose, issued/expiry times, and nonce; the server validates those claims and never accepts a client-selected pre-session identity for an issuance-receipt key.

Bootstrap is idempotent for an existing valid anonymous cookie: it returns/reuses that `pre_session_id` and does not rotate it while an onboarding trait offer or login/registration issuance receipt can still be recovered. The pre-session expiry covers the maximum offer plus issuance-receipt recovery window, and the server caps each recovery to both its own expiry and the pre-session expiry. It rotates/clears only after validated session issuance, an explicit user reset that abandons the pending logical command/offer, or natural expiry. A page reload, repeated bootstrap call, network retry, or tab remount alone cannot change the offer/receipt-key identity.

The provider redirect callback is the deliberate exception: it is a cross-site top-level navigation and is not required to carry an Origin or same-origin Fetch Metadata. It accepts only the registered callback method/path and validates the browser-bound one-time state cookie/query value, PKCE verifier, provider issuer, authorization-code exchange, and exact redirect URI. It creates only a short-lived completion ticket and never establishes a session directly. The subsequent finalize is a same-origin CSRF-protected POST. This prevents both OAuth/login CSRF and rejection of valid provider redirects.

Bearer-authenticated non-browser clients do not use the CSRF cookie, but cannot combine a bearer credential with conflicting auth cookies. Conflicting credentials are rejected with `400 ambiguous_credentials`.

## 8. Authentication workflows

### 8.1 Password login

1. Require exact Origin/Fetch Metadata, a valid signed anonymous pre-session CSRF identity, and one client-generated issuance idempotency UUID for the logical login; normalize the username and apply IP/account rate limits.
2. Load the user and versioned password credential without revealing whether the account exists. Verify the password using `pkg/security/password` with a constant-shaped failure response on every attempt, including receipt recovery, then require user status `active`.
3. Derive the complete receipt key and make a non-mutating exact-receipt read. If a committed receipt exists, verify/integrity-check its active session, decrypt the same refresh/CSRF bytes, and successfully sign a fresh access JWT for the receipt's session ID before returning; no new candidate/session is created.
4. If no receipt exists, pre-generate candidate session/refresh IDs and raw refresh/CSRF values and successfully sign an access JWT for that candidate session before beginning a mutating transaction. A signer failure changes no durable state.
5. In one Ent transaction, serialize the `(password_login, user_id, pre_session_id, idempotency_key)` command, lock/recheck the active user's security version and the credential version that was verified, and recheck the receipt. If a concurrent winner appeared, roll back the unused candidate and restart step 3 so JWT claims are signed for the committed session; never return a token signed for the losing candidate. A suspension/password/security change is a failure, not a stale login. Otherwise atomically create the session, first refresh-token record, encrypted issuance receipt, security audit, and `identity.session.created.v1` outbox row. The receipt binds the pre-generated identities and complete command key.
6. Set the three cookies from the committed/recovered result and return the canonical session/user response. A timeout, connection loss, or tab reload retains the same non-secret UUID until receipt expiry; retry/re-entry must use that UUID and supply the password again.

Failed login logs contain CID, a normalized non-secret subject hash, and reason category. Passwords, credential hashes, raw usernames for nonexistent accounts, tokens, cookies, and authorization headers are never logged.

### 8.2 Password registration

1. Require exact Origin/Fetch Metadata, a valid signed anonymous pre-session CSRF identity, one client-generated issuance idempotency UUID, a bounded normalized recovery email, and a durable unexpired onboarding trait-offer ID. Normalize and validate the username, recovery contact, required typed profile attributes, offered root-bone/three-talent selection, and all bounded field sizes before opening a transaction.
2. Apply the password policy and create the versioned keyed request fingerprint over purpose, pre-session identity, normalized username/recovery-email/profile payload, offer ID/catalog revision/checksum, exact root-bone/talent selection, and relevant schema/policy revisions. It excludes the raw password and credential hash; password equality is proved cryptographically against the credential. The same key with a different fingerprint is `409 registration_request_conflict`.
3. Make a non-mutating exact-receipt lookup. For a committed exact receipt, require the same request fingerprint, load the committed active user/credential, verify the supplied password against that credential again, validate the session/security versions, decrypt the same refresh/CSRF bytes, and sign a fresh access JWT for the receipt's session. The receipt/idempotency value alone can never recover an account.
4. With no receipt, derive the Argon2 password hash through `pkg/security/password` before the database transaction, then pre-generate user/session/refresh IDs and raw refresh/CSRF values and successfully sign the candidate access JWT. Hash/signing failure changes no durable state and does not hold database locks during expensive password work.
5. Execute `api/internal/application/workflow/onboarding` through one `platform/transaction.UnitOfWork`. Serialize/recheck the registration request, lock and validate the offer against the signed pre-session, enforce username/entity uniqueness, revalidate active profile-attribute revisions, and prove the root bone plus three talents are immutable members of that offer. Then atomically consume the offer and create the Identity user/credential, pending encrypted recovery contact plus verification token/delivery, required profile values, Cultivation traits/reward profile and initial `1200` stats, authentication session, first refresh record, encrypted issuance receipt with request/offer fingerprint, audit, and outbox events. Every participant uses only the transaction-bound context; all effects commit or all roll back.
6. If a concurrent exact request committed while the candidate was being prepared, roll back and restart receipt recovery so claims use the committed IDs. A different payload/idempotency key cannot attach to, mutate, or recover the existing account; username/request conflicts return the canonical conflict without a partial account. Set cookies only from the committed or exact recovered result.

### 8.3 Refresh rotation

1. Read the refresh cookie, CSRF proof, and required refresh idempotency UUID.
2. Hash the raw refresh token before lookup.
3. Begin an Ent transaction and lock the token and session rows with `SELECT ... FOR UPDATE`.
4. If the token is already consumed and the exact unexpired receipt tuple exists, decrypt and integrity-check that receipt, verify the session remains active, sign a fresh access JWT, and return the same replacement refresh token/CSRF nonce. No second token branch is created.
5. Otherwise require token status `issued`, unexpired session, active user, and matching security versions.
6. Generate the replacement refresh token and CSRF nonce and sign the access JWT before consuming the old token. A signing failure rolls the transaction back and leaves the old token usable.
7. Mark the current token `consumed`; create its one-time replacement and encrypted recovery receipt in the same transaction.
8. Update `last_seen_at` and the sliding idle expiry without extending the absolute expiry.
9. Append a security audit record and commit.
10. Replace the access, refresh, and CSRF cookies with the values bound to that receipt.

If a consumed token is presented with a missing/different idempotency key or after the recovery window, the transaction revokes the whole family/session, records `identity.refresh.reuse_detected.v1`, and rejects the request. Concurrent refreshes with different keys therefore still trigger reuse defense. The frontend serializes refresh across tabs and retains the same idempotency key until a success response; a lost response safely replays the receipt instead of bricking the session.

### 8.4 OAuth completion and account linking

The provider redirect callback validates the browser-bound state, PKCE, issuer, code exchange, and redirect URI, then stores a short-lived hashed completion ticket and returns to the application. Both existing-user and new-user paths continue only through the same-origin, anonymous-CSRF-protected finalize POST with a required client idempotency UUID. The client retains that non-secret UUID through unknown outcomes for at most the issuance-receipt TTL. Finalize uses this pipeline:

1. Read the ticket identity without mutating it. An unconsumed ticket resolves the provider subject to an existing user or a pre-generated new user ID/onboarding candidate; a consumed ticket may continue only through its exact matching issuance receipt.
2. Pre-generate session/refresh IDs and raw refresh/CSRF values, then successfully sign the access JWT for that candidate before consuming the ticket. A signer failure changes no durable state.
3. Begin the transaction, lock the completion ticket for exactly one declared purpose, and look up the issuance receipt by ticket/purpose/idempotency tuple. An exact committed retry follows the receipt recovery path; a different UUID cannot consume or branch an already consumed ticket.
4. Re-resolve `(provider, provider_subject)` under its database uniqueness constraint and lock the linked user. Provider email is not an account key. Recheck active status, credential ownership, security version, and every user/session claim input captured before signing; if the user/candidate or any claim input changed, roll back and restart resolution/pre-signing rather than committing stale claims.
5. For an already linked active user whose locked state still matches the pre-signed candidate, atomically consume the ticket, create the session/first refresh record and encrypted issuance receipt, and append audit/outbox records.
6. For a new user, require the complete onboarding payload plus an unexpired trait offer bound to the anonymous pre-session and call `api/internal/application/workflow/onboarding` in one `platform/transaction.UnitOfWork`. A provider-only account may explicitly consent to seed a verified recovery contact from a provider-verified email; otherwise it can add/verify one later. If onboarding also creates a password credential, the normal required pending recovery-email contract applies. Offer consumption, user, OAuth identity, applicable recovery contact/verification delivery, required profile attributes, cultivation trait assignment/reward profile, session, first refresh record, issuance receipt, audit, outbox, and completion-ticket consumption either all commit or all roll back.
7. Set cookies only from the pre-signed result or recovered receipt after commit. If a uniqueness race discovers that the provider identity was linked concurrently, return the canonical safe conflict/restart result; never attach it to a second account or leave a partially onboarded user.

Account linking is a separate authenticated purpose. The initiation is bound to the current session ID, and the callback creates a `link` completion ticket bound to that same session. Same-origin link finalize requires an active session, CSRF, recent reauthentication, an unexpired ticket, and a final uniqueness check in the linking transaction. It never falls back to anonymous login/finalize behavior. Unlink uses the same recent-reauthentication rule and cannot remove the final usable credential.

### 8.5 Recent reauthentication

`POST /api/auth/reauthenticate` is the explicit step-up command. It requires an active cookie session, exact Origin, same-origin Fetch Metadata, CSRF, and either the current password or a one-time provider proof whose OAuth state/ticket is bound to the current session and `reauthenticate` purpose. Provider step-up forces fresh provider authentication (`max_age=0` or the provider-equivalent) rather than accepting a stale callback session.

The command verifies the proof, user status, session status, and credential ownership, then atomically updates `auth_sessions.last_reauthenticated_at` to server time and appends the security audit/outbox event. It returns the new reauthentication expiry; it does not accept an access/refresh token as proof and ordinary refresh never advances this timestamp. Sensitive link/unlink and configured security/admin commands load the session row and require the timestamp within the ten-minute window. A reauthentication ticket is one-time, purpose-bound, session-bound, and cannot migrate to another tab's different session.

### 8.6 Logout and session management

- Logout revokes the current session and all unconsumed tokens in its family.
- “Log out all devices” increments `users.security_version` and revokes all active sessions.
- A user may list their sessions with coarse device metadata and revoke any one of them.
- Administrators with the required permission may revoke sessions for another user; the action requires a reason and creates an audit record.
- Password change and password reset revoke every existing session, then optionally establish a new session for the initiating client according to the same login transaction.

Every logout, individual/global session revoke, password change/reset, and account status command commits durable revocation plus its outbox event before invalidating caches or closing sockets. A post-commit invalidation dispatcher closes only the affected session/user connections with the stable authorization close reason. Rollback closes nothing. If the initiating process crashes after commit, the outbox consumer plus bounded session revalidation/TTL completes the invalidation; no database transaction performs a WebSocket write.

### 8.7 User-state changes

Suspension or deletion is one transaction containing user-state mutation, session revocation, audit, and outbox. Only after commit does the shared invalidation dispatcher invalidate the session cache and close the user's sockets. If local invalidation is interrupted by a crash, the outbox consumer, session resolution from PostgreSQL, and the 30-second cache bound enforce the durable state.

## 9. Authorization model

The single-tenant Casbin model remains role-based:

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
```

Subjects use canonical prefixes:

- users: `user:<uuid>`;
- roles: `role:<stable-role-key>`.

No tenant/domain value is introduced. A future multi-tenant model requires a separately versioned migration and is not simulated with unused columns.

## 10. Canonical authorization data

### 10.1 Casbin rules

`casbin_rules` remains compatible with the pinned Ent adapter:

```text
id, ptype, v0, v1, v2, v3, v4, v5
```

Canonical rows are normalized before persistence:

- role policy: `p, role:<key>, <resource>, <action>, '', '', ''`;
- user-role mapping: `g, user:<uuid>, role:<key>, '', '', '', ''`.

A unique index over `(ptype, v0, v1, v2, v3, v4, v5)` prevents duplicate rules. Policy rows are never soft-deleted.

### 10.2 Revision

`authorization_state` is a single-row table for the single tenant:

| Column | Meaning |
|---|---|
| `id` | Constant key `default` |
| `desired_revision` | Monotonic committed policy revision |
| `updated_at` | Last policy mutation time |
| `updated_by` | Actor user UUID |

The runtime records its applied revision in memory. Readiness requires `applied_revision == desired_revision`.

### 10.3 Audit

`api/internal/platform/audit.audit_log` is the single audit source of truth. For an authorization command, Identity supplies a typed, redacted payload containing:

- immutable audit ID;
- actor user/session IDs;
- CID;
- command type and reason;
- target role or user;
- old and new revision;
- normalized before/after policy diff;
- timestamp and outcome.

Diff payloads contain policy identifiers, role keys, resources, and actions; they do not contain tokens or credentials. The authorization mutation and shared audit row commit in the same transaction. There is no separately authoritative `authorization_audits` table or dual write.

### 10.4 Resource/action registry

Each Judgify module contributes a compile-time typed registry of supported resource/action pairs. Identity aggregates it and exposes a read-only admin endpoint. Candidate policy validation rejects unknown resources/actions, malformed wildcards, duplicate rules, and invalid subject prefixes.

The registry is code-owned because every permission must correspond to an implemented guard. The database owns assignments, not the existence of arbitrary capabilities.

## 11. Ent Casbin adapter lifecycle

The adapter version is pinned. A minimal maintained patch adds a constructor option equivalent to `SkipSchemaCreate(true)`.

- `judgify migrate` creates and upgrades `casbin_rules` and its indexes.
- API and worker database roles have no schema-creation privileges.
- The adapter loads policy into an enforcer and may serve read operations.
- Admin mutations do not use `EnableAutoSave`, `AddPolicy`, `RemovePolicy`, or row-by-row grouping changes.
- The patched surface is covered by an integration test that proves startup performs no DDL and can load a migrated schema.

## 12. Atomic authorization mutation

All role-policy and user-role replacement commands use the same pipeline:

1. Parse the command and require an explicit reason, then acquire the process-local authorization decision gate exclusively.
2. While holding the exclusive gate, authorize the admin command against the current runtime snapshot through a gate-held enforcement helper that does not try to reacquire the shared gate; the ordinary public `Authorize` path is never called recursively under this lock.
3. Begin a PostgreSQL/Ent transaction and acquire a transaction-scoped advisory lock dedicated to authorization mutation.
4. Load the current role catalog, complete Casbin policy, and current revision.
5. Normalize the requested change into a complete candidate snapshot.
6. Validate syntax, the resource/action registry, referential integrity, and administrative invariants.
7. Build and validate a candidate Casbin enforcer entirely in memory from that snapshot.
8. Replace affected role/catalog and `casbin_rules` rows through the same Ent transaction.
9. Increment `desired_revision`, append the before/after row through the platform audit appender, append `authorization.changed.v1` to the outbox, and execute PostgreSQL `NOTIFY` within the transaction.
10. Commit, atomically publish the already validated candidate runtime with the committed revision, then release the exclusive gate.

Every authorization decision takes the same gate in shared mode across revision/stale-state check, runtime pointer load, and `Enforce`. Therefore a decision linearizes wholly before commit against the old snapshot or after publish against the new snapshot; there is no committed-but-stale allow window. The in-memory publish is an atomic pointer store and cannot expose a partial enforcer.

If the process crashes after commit but before publish, startup loads the committed revision before readiness. A listener plus a five-second revision poll acts as reconciliation. On discovering a newer desired revision, the reconciler acquires the exclusive gate, marks the runtime stale/fail-closed before loading, validates and atomically publishes the complete snapshot, then clears stale state and releases the gate. This process-local barrier is sufficient for the initial single API process. A future multi-process deployment requires a coordinated revision barrier that prevents any instance serving stale decisions; PostgreSQL `NOTIFY` alone is not such a barrier.

Revision conflicts return `409 authorization_revision_conflict` with the current revision so the admin UI can reload and present a fresh diff.

## 13. Administrative invariants

Every policy mutation enforces all of the following:

- At least one active user retains the protected `superadmin` system role.
- An actor cannot remove their own final administrative path unless another active superadmin already exists.
- System roles cannot be deleted or have their stable keys renamed.
- The `superadmin` role cannot lose the protected authorization-management capability.
- Role keys are normalized, immutable after creation, and unique.
- A user-role mapping references an active user and an existing role.
- Wildcards are permitted only for protected system roles declared by the registry.
- Empty policy replacement is explicit and still subject to invariants; omitted fields never mean “delete everything.”
- Every mutating command supplies the revision it was based on and a non-empty audit reason.

The admin UI first requests a server-generated normalized preview. The write command submits the expected revision and the desired complete replacement. The server recomputes the diff and invariants; it never trusts the preview payload as authority.

## 14. Route and object authorization

Middleware authorization is intentionally coarse:

```text
HTTP route
→ verify active principal
→ Authorize(resource, action)
→ call application use case
```

Application services protect object-sensitive data:

- submission source: `submission.read_any` or `submission.read_own` plus `submission.user_id == principal.user_id`;
- hidden test data: admin/author capability only, never ordinary problem-read permission;
- contest submission and standings: registration, contest state, freeze rules, and administrative override;
- user security/session data: self or user-security administration capability;
- draft problem revisions: author/editor capability plus publication state.

Handlers never implement ownership rules themselves. Repositories accept already authorized query constraints or application services verify the loaded object before mapping sensitive fields. List endpoints enforce authorization in SQL/query predicates to avoid loading and then leaking unauthorized rows.

Authorization denial returns `403`. Missing/invalid credentials return `401`. Runtime, policy-load, or dependency errors fail closed and return a non-sensitive `500` or temporary `503` with the CID; they are not misreported as ordinary denial.

## 15. API surface at design level

### Browser/session APIs

- password registration establishes Identity/recovery-contact/profile/Cultivation/session state only through the onboarding UnitOfWork, keyed canonical request fingerprint, and required recoverable issuance UUID; OAuth completion establishes a session through its completion ticket/receipt;
- password login establishes a session only through a required client issuance-idempotency UUID and the shared recoverable issuance receipt;
- refresh rotates the current refresh token;
- logout revokes the current session;
- logout-all revokes all sessions;
- session list and revoke support account security management;
- `/api/session/me` returns canonical user, session, and effective navigation capabilities;
- `/api/auth/reauthenticate` performs an explicit password/provider step-up and returns its server-authoritative expiry;
- password change/reset revokes old sessions;
- recovery-contact status, verify, resend, replace, and remove commands use encrypted-at-rest contact storage; replace/remove require recent reauthentication and all responses avoid account/contact enumeration;
- OAuth login/onboarding finalizes only through the same-origin completion command; identity list/link/unlink uses an active session and link/unlink requires recent reauthentication;
- profile definition/value endpoints expose only authorized typed values and use revision checks.

Session responses never include raw access/refresh tokens. All errors use stable application error codes and include a CID in the response envelope.

The canonical OAuth browser routes are a same-origin CSRF/Fetch-Metadata-protected initiation `POST`, the exact registered provider callback `GET` used only for top-level navigation, and a same-origin protected finalize `POST`. The current state-changing `GET /oauth/:provider/start` alias and callback `POST` variant are retired; they are not compatibility routes in production. A route-table gate fails if initiation is reachable by GET, callback by POST, or any callback handler creates authentication cookies/session state directly.

`/admin/login` is not a second authentication system. During migration it redirects to `/login?next=/admin`; after route consumers are updated, only the unified login flow remains.

### Admin access APIs

- list resource/action registry;
- list roles and normalized policy snapshots;
- preview and replace a role policy at an expected revision;
- preview and replace user roles at an expected revision;
- create/update/delete non-protected roles;
- suspend/restore users and revoke sessions;
- preview/create/update/archive profile attribute definitions and run bounded value migrations;
- browse authorization/security audit records.

All list endpoints use bounded pagination and allowlisted filters/sorts.

## 16. Logging and audit semantics

go-common automatically enriches logs with CID. Judgify adds bounded fields appropriate to the operation:

```text
user_id, session_id, token_id, policy_revision, audit_id, resource, action
```

Security-relevant state transitions are `INFO`; rejected/replayed credentials and rate-limit events are `WARN`; dependency or reconciliation failures are `ERROR`; internal validation detail is `DEBUG`. Production defaults to `INFO` and may temporarily enable component-scoped `DEBUG`.

No level may log passwords, hashes, reset secrets, raw JWTs, refresh tokens, cookies, authorization headers, OAuth secrets/codes, private keys, or full sensitive policy request bodies.

## 17. Failure semantics

- Password-login signer or transaction failure creates no session, refresh token, issuance receipt, audit, or outbox effect. A lost response/cookie delivery after commit is recovered only by re-verifying the password with the exact pre-session/idempotency tuple, returning identical refresh/CSRF bytes and a re-signed access JWT; it never creates a second session branch or treats a different key as the same command.
- Registration password-hash/signer/transaction failure creates no partial Identity, profile, Cultivation, credential, or session state. A committed lost response is released only when the exact request identity/hash/idempotency tuple and supplied password re-prove the committed credential; changed payload/key conflicts and never creates or mutates a second account branch.
- Refresh transaction failure leaves the original token state unchanged unless reuse was durably detected and revoked.
- Concurrent refresh allows exactly one successful rotation; reuse revokes the family.
- A lost refresh response replayed with the same idempotency UUID returns the receipt's exact replacement; using a different UUID follows reuse defense rather than creating a branch.
- OAuth callback/finalize signer failure consumes nothing; transaction failure creates neither a partial account nor a session; new-user onboarding, issuance receipt, and ticket consumption share one transaction. A lost response is recoverable only with the exact finalize idempotency UUID.
- Reauthentication failure or stale/session-mismatched provider proof does not advance `last_reauthenticated_at`.
- Password-reset request commit is independent of provider availability; delivery is retried durably and token consumption remains single-use.
- PostgreSQL unavailability prevents login, refresh, revocation, and policy mutation; the service does not claim success from cache alone.
- Authorization policy transaction failure leaves both database and runtime on the old revision.
- Crash after policy commit is repaired before readiness or by stale-runtime reconciliation.
- `NOTIFY` loss is repaired by revision polling.
- Session cache invalidation loss is bounded by its defensive TTL and repaired from canonical PostgreSQL state.

## 18. Migration and rollout

The rollout is an expand-and-switch migration:

1. Add user security columns, session/refresh/rotation-receipt/session-issuance-receipt, encrypted recovery-contact/verification-token, and leased security-email tables (including pre-session/request-fingerprint bindings), OAuth completion-purpose fields, versioned profile-attribute tables, authorization state, the shared platform audit table, canonical Casbin uniqueness, outbox support, and indexes.
2. Normalize existing roles and Casbin rows; seed `authorization_state.desired_revision = 1` from the complete migrated policy.
3. Deploy the patched Ent adapter and prove runtime startup performs no DDL.
4. Deploy server-side session workflows and cookie/CSRF support while the old frontend is taken out of service for the coordinated release.
5. Deploy the frontend using `credentials: include`, CSRF, and `/api/session/me`; remove token persistence and JWT decoding.
6. Invalidate every legacy stateless access/refresh token by rotating the signing key and requiring users to log in again. Legacy refresh tokens are not grandfathered because they cannot provide rotation/reuse guarantees.
7. Switch role/user admin writes to revisioned full-policy replacement and disable Casbin AutoSave.
8. Verify desired/applied revisions, session operations, security audits, and WebSocket revocation before enabling readiness.
9. Remove obsolete token JSON fields and legacy row-by-row mutation methods after all Judgify call sites use the new contracts.
10. Switch OAuth clients to protected initiation/finalize POST plus the single registered callback GET, then remove the legacy start-GET and callback-POST aliases before production traffic.

Schema changes are additive during the switch release. Destructive column/table removal occurs only in a later release after backup, rollback-window, and compatibility verification.

## 19. Test strategy and release gates

### Unit and property tests

- JWT algorithm, issuer, audience, purpose, `kid`, time-window, and malformed-claim matrices;
- principal context propagation across HTTP, gRPC, and WebSocket adapters;
- password policy and `NeedsRehash` behavior;
- role/policy normalization and registry validation;
- system-role, final-superadmin, and self-lockout invariants;
- authorization decision-gate linearizability under repeated mutation commit/publish and reconciliation: every concurrent decision observes wholly the old or new revision, never a committed stale allow; stale runtimes fail closed;
- fuzz tests for token parsing, subject normalization, and policy input.

### PostgreSQL integration tests

- one-time refresh rotation and family reuse revocation;
- concurrent refresh race with one winner and durable family revocation on replay;
- commit a refresh rotation, drop the HTTP response, then replay the old cookie with the same idempotency UUID and prove the exact replacement token/CSRF bytes are recovered, only one child exists, and the family remains active;
- replay that consumed token with a different UUID or after receipt expiry and prove durable family revocation;
- session/user status invalidation;
- existing-user OAuth finalize creates one session only after same-origin ticket consumption and locked active-user/security-version/claim-input recheck; suspension/security/provider-link races force rollback and re-resolution/re-signing rather than stale session claims. New-user finalize atomically commits Identity, applicable recovery contact, required attributes, Cultivation onboarding, session, audit, and outbox, with rollback/race tests proving no partial account or duplicate provider link;
- Password-login, OAuth, and registration issuance tests cover signer failure before mutation, crash before commit, commit followed by lost response, exact-idempotency recovery of identical refresh/CSRF bytes with a re-signed access JWT, different-key rejection, receipt expiry/erasure, and exactly one account/session/token branch. Password-login recovery additionally proves password verification runs again, a wrong password cannot retrieve a receipt, and concurrent same-key attempts converge on one session. Registration tests prove Argon2 work is outside the transaction, keyed canonical request/trait-offer fingerprint mismatch conflicts, only the bound root bone and three-of-six offered talents can commit, offer consumption and account/recovery-contact/profile/Cultivation/session state roll back together, exact recovery can resolve a consumed offer through its issuance receipt, duplicate username/payload/offer races create no partial state, and the onboarding UnitOfWork rolls every participant back together;
- password reset single-use behavior plus durable delivery transaction rollback, API-crash recovery, lease expiry/reclaim, provider timeout retry with the same provider idempotency key, dead-state transition, and ciphertext erasure after send/expiry;
- delayed/retried delivery rechecks the linked token and cancels without provider I/O when the token is expired, consumed, revoked, or superseded, proving a backlog cannot knowingly send a dead reset link;
- password onboarding atomically creates one encrypted pending recovery contact and verification intent; verify/resend/replace/remove tests cover token hashing, contact/version races, recent reauthentication, explicit provider-verified OAuth consent, previous-contact switch, ciphertext/key rotation, enumeration resistance, and address/token redaction;
- protected profile-definition migrations test `kind` and `required_on_onboarding` as independent dimensions, reject system rename/archive and clearing a code-required flag/incompatible revision, survive crash/resume idempotently, reconcile concurrent value writes by source version, fail on definition drift, quarantine invalid legacy values, and switch active revision/audit/outbox atomically;
- complete role-policy and user-role replacements under concurrent admin requests;
- transaction rollback leaves Casbin rows, revision, audit, outbox, and runtime unchanged;
- adapter starts against a migrated schema using a database role without DDL permission;
- startup and revision reconciliation load the canonical complete policy.

### HTTP and browser tests

- exact CORS origin allowlist and credentials behavior;
- CSRF matrix for every unsafe cookie-authenticated method;
- anonymous login/registration CSRF, issuance-idempotency, and OAuth state/Fetch-Metadata matrices;
- repeated anonymous bootstrap across reload/tab remount preserves the valid `pre_session_id` through trait-offer and issuance-receipt recovery; validated success, explicit reset, and expiry rotate/clear it, and exact offer/login/registration recovery still finds committed state after reload;
- provider callbacks accept only the registered cross-site navigation contract and never establish a session; existing/new-user OAuth and link/reauth finalize endpoints require the correct same-origin CSRF/Fetch-Metadata and purpose/session-bound ticket;
- route enumeration proves OAuth initiation/finalize are POST-only, provider callback is GET-only, and the legacy state-changing start GET/callback POST aliases return no production handler;
- ambiguous bearer/cookie credentials are rejected;
- access token cannot refresh, refresh token cannot access a protected route;
- the browser refresh coordinator reuses one UUID after a lost response and after leader-tab crash/reload within the receipt TTL; no second rotation branch or false reuse revocation occurs;
- recent reauthentication accepts valid password/fresh provider proof, rejects stale/replayed/wrong-session proof, is not advanced by refresh, expires after the configured window, and gates OAuth link/unlink;
- logout/session revoke/password change/reset/suspension affects HTTP and WebSocket access only after commit; rollback leaves sockets open, while crash after commit is repaired through the outbox/session revalidation path;
- recovery verification and forgot-password remain enumeration-resistant, survive provider outage without losing committed delivery, reject pending/revoked/replaced contacts for reset, and emit no address/fingerprint/verification/reset credential in response, log, metric, or trace;
- `localStorage` contains no token or sensitive session material;
- admin preview, conflict, invariant, audit, and successful replacement flows.
- profile attribute type/visibility/editability, value migration, archive, and public-projection flows.

### Mandatory gates

- `go test -race` passes for authentication, authorization runtime, middleware, and Identity services.
- Identity security-critical packages maintain at least 90% statement coverage; all changed core packages maintain at least 80%.
- Static analysis, lint, vulnerability scan, migration-from-zero, migration-upgrade, and schema-drift checks pass.
- Load testing demonstrates bounded latency and memory for session-cache misses and concurrent authorization reads during policy updates.
- Security review finds no token/credential material in logs, responses, browser storage, or audit payloads.
- A clean process start is not ready until PostgreSQL is reachable, migrations are compatible, and authorization desired/applied revisions match.

## 20. Explicit exclusions

This design does not add multi-tenancy, Casbin domains, external Redis/session storage, passkeys, MFA, or long-lived personal API tokens. Their future addition must use the same session, revision, audit, and transport-neutral security boundaries without weakening the guarantees defined here.
