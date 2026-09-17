# Recovery contact lifecycle design

**Date:** 2026-07-22  
**Status:** Approved implementation contract  
**Deployment:** Single tenant, single host  

## Goal

Every password account has an explicit, verified recovery email without making email the login
identity. Registration, verification, replacement, forgot-password, reset, delivery and cleanup
must remain correct across retries, provider outages, process crashes and concurrent requests.

This design extends the existing Identity/session and durable security-email architecture. It does
not introduce a generic workflow engine and adds nothing Judgify-specific to go-common.

## Non-negotiable invariants

1. A password account has at most one verified recovery email and at most one pending candidate.
2. A replacement never revokes the verified contact until the candidate is verified atomically.
3. OAuth/provider email is never silently promoted, verified or substituted as recovery state.
4. Forgot-password uses only an active password credential and the current verified contact.
5. Reset updates an existing password credential; it never converts an OAuth-only account.
6. Raw addresses and tokens are absent from PostgreSQL, logs, metrics, URLs after page bootstrap,
   local/session storage and audit payloads.
7. Token/contact/delivery state is revalidated immediately before provider I/O. Superseded or stale
   deliveries are cancelled and cryptographically erased without a network call.
8. Registration/session receipt, user, password credential, onboarding projection, pending contact,
   verification token, encrypted email intent, audit and outbox commit or roll back together.
9. All state transitions use a declared lock order: user, contacts, token, delivery.
10. Exact retries are idempotent, but idempotency identifiers are never sufficient authentication.

## Public HTTP contract

All action responses set `Cache-Control: no-store`. Unsafe browser requests use the existing Origin,
Fetch Metadata, CSRF and bounded-body protections.

### Registration

`POST /auth/register` requires `recovery_email` in addition to the current profile, password and
opaque onboarding ticket. The address is normalized and bounded before expensive password hashing.
The success response remains the recoverable session response and never echoes the address.

### Verify contact

`POST /auth/recovery-contact/verify`

```json
{ "token": "opaque-base64url-secret" }
```

Possession of the one-time token is the authority. Malformed, expired, consumed, superseded and
wrong-version tokens share one safe public error. Success consumes the token and returns only a
status. This endpoint is public because a verification link may open without a live session.

### Forgot/reset password

`POST /auth/forgot-password` accepts `{ "username": "..." }` and always returns HTTP 202 with the
same `{ "status": "accepted" }` projection and comparable work for missing, ineligible and eligible
accounts. Persisted account/contact quotas and a local per-IP limiter apply before scheduling.

`POST /auth/reset-password` accepts the token in the JSON body. Invalid token states share
`invalid_or_expired_reset`. Successful reset consumes the token, changes the existing credential,
increments `users.security_version` and revokes every active session in one transaction.

## Authenticated contact management contract

- `GET /auth/recovery-contact`
- `POST /auth/reauthenticate`
- `PUT /auth/recovery-contact`
- `POST /auth/recovery-contact/resend`
- `DELETE /auth/recovery-contact/pending`
- `DELETE /auth/recovery-contact`

Mutation commands require the existing authenticated session, CSRF, Origin validation, a stable
`Idempotency-Key`, expected contact version where applicable and recent reauthentication. The
reauthentication window defaults to ten minutes and is persisted in `auth_sessions`.

The read projection is deliberately masked:

```json
{
  "current": {
    "status": "verified",
    "masked_address": "j***@example.com",
    "version": 2
  },
  "pending": {
    "purpose": "replacement",
    "masked_address": "n***@example.com",
    "version": 3,
    "verification_expires_at": "2026-07-23T08:00:00Z",
    "resend_available_at": "2026-07-22T08:01:00Z",
    "delivery_state": "queued"
  },
  "reauthenticated_until": "2026-07-22T08:10:00Z"
}
```

Ciphertext, lookup/fingerprint hashes, raw address/token and internal delivery/provider IDs never
cross the boundary. Removing the only verified recovery contact is rejected while an active password
credential would otherwise have no recovery mechanism.

## Cryptographic storage

`RecoveryContactProtector` owns a bounded keyring. Configuration names one active key ID and a
bounded set of decrypt-only old keys. Domain-separated derivation produces independent keys for:

- contact AEAD;
- address lookup and fingerprint blind indexes;
- verification-token HMAC;
- reset-token HMAC;
- rate-limit subject HMAC.

Contact AEAD associated data binds `user_id`, `contact_id`, `kind`, `version` and `key_id`; ciphertext
cannot be transplanted between contact rows. Token rows store a hash key ID beside the 32-byte HMAC
so rotation does not invalidate live tokens. Tokens contain at least 32 random bytes and use base64url
without padding.

The encrypted delivery payload is versioned and bounded. It contains the normalized recipient
snapshot, raw one-time credential and link expiry only. Its AAD binds delivery ID, kind, linked token
ID, provider idempotency key and expiry. Ciphertext is erased on sent, dead, cancelled or expiry.

## Data model changes

The existing recovery/contact/token/delivery schemas remain the base, with these corrections:

- add hash-key IDs to verification and reset tokens;
- ensure contact ciphertext AAD uses the final pre-generated contact ID and version;
- record delivery linkage for both verification and reset intents in the scheduling port;
- persist bounded quota buckets/counters or equivalent append-only issuance timestamps queried under
  the same lock; restart must not reset account/contact security quotas;
- persist command idempotency for replacement/resend/cancel flows;
- add indexes supporting latest-issued token checks and bounded janitor scans;
- retain the partial unique indexes for one verified and one pending email per user.

No schema permits a verified row without `verified_at`, a revoked row without `revoked_at`, a live
delivery without ciphertext, or a terminal delivery with ciphertext.

## Transaction workflows

### Register

Normalize and validate cheap fields, verify the onboarding ticket, hash the password, pre-generate
all IDs/secrets and seal the contact/delivery payload. One transaction then consumes the ticket,
creates Identity/onboarding/session state, inserts the pending contact/token/delivery and appends audit
and outbox records. An exact receipt retry verifies the password again before returning the committed
session credential.

The registration fingerprint excludes password bytes. It binds normalized non-secret request facts;
password equality is established by verifying against the committed credential on retry.

### Verify

Resolve the keyed token candidate, then transactionally lock user, both contact slots, token and
delivery. Revalidate issued state, expiry, contact version and latest-token status. For replacement,
revoke the old verified contact and activate the candidate in the same commit. Consume the token,
supersede sibling tokens, cancel/erase stale delivery and append audit/outbox.

### Replace/resend/cancel

Require recent reauthentication and expected version. Replacement keeps the verified row live while
creating or replacing only the pending candidate. Resend is a new token/delivery intent: it supersedes
the previous token and cancels/erases its delivery before inserting the replacement. Exact retries
return the existing masked projection.

### Forgot

The public response is selected independently from eligibility. The transaction locks the active
password account and current verified contact, enforces persisted quotas, supersedes previous issued
reset intent, and inserts one new token/encrypted delivery. Decryption of the contact occurs only in
bounded memory needed to seal the delivery payload.

### Reset

A cheap preflight validates token/user/contact/password-credential/security-version before password
hashing. The final transaction repeats all checks under locks, updates the existing credential,
consumes the token, supersedes siblings, increments security version, revokes sessions, cancels stale
deliveries and appends audit/outbox.

## Dispatcher validity gate

Claiming remains bounded and leased with `FOR UPDATE SKIP LOCKED`. Before opening ciphertext or
calling the provider, the dispatcher runs a transaction-bound `PrepareForSend` gate:

- verifies delivery fence and payload expiry;
- verifies linked token is issued, unexpired and the latest active intent;
- for contact verification, verifies the contact is still pending at the exact version;
- for reset, verifies user security version, active password credential and current verified contact;
- atomically cancels/erases invalid intents.

Only an active prepared lease is decrypted. Provider timeout retries use the stable provider
idempotency key; token single-use state prevents multiple security effects.

## Quotas and cleanup

Defaults live in validated settings, not handlers:

- resend cooldown: 60 seconds;
- verification sends: 5 per contact/hour and 10/day;
- replacement starts: 3 per user/day;
- reset requests: 3 per user/15 minutes and 10/day;
- verification TTL: 24 hours;
- reset TTL: 15 minutes;
- recent reauthentication: 10 minutes.

A lifecycle-owned bounded janitor uses `SKIP LOCKED` and the declared FK order. It expires issued
tokens, cancels/erases expired deliveries, revokes/erases stale pending contacts, erases revoked
contact ciphertext, then deletes terminal delivery/token/contact rows after configured retention.
Every batch has an explicit limit, timeout, shutdown path and CID-aware low-cardinality logs.

## Frontend behavior

- registration requires and validates recovery email before the expensive final command;
- onboarding and settings show pending verification without exposing the raw address after reload;
- `/verify-recovery-contact#token=...` reads the fragment once, immediately calls
  `history.replaceState`, keeps the token only in component memory and renders explicit
  success/expired/superseded/error states;
- reset-password uses the same fragment-to-memory pattern and sends the token only in JSON;
- settings/security supports view, reauthenticate, replace, resend countdown, cancel pending and
  safe remove states with capability-independent self-service authorization;
- forgot copy never confirms account existence;
- all text lives in `website/constants/text.ts`, and all flows support keyboard, screen readers,
  light/dark/system, loading, retry and terminal states.

## Observability and redaction

Debug logs trace flow and branch decisions with `cid`, command/delivery/token IDs, attempt, state and
bounded error class. Info logs record successful or terminal state changes. Warn/error is reserved for
degraded dependencies or invariant failures. Production log level hides debug; no level includes raw
address, token, ciphertext, blind index, provider body or password.

Metrics use kind/status/error-class only. User/contact/delivery IDs are never labels.

