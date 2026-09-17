# Recovery contact lifecycle implementation plan

## Global constraints

- Preserve the single-tenant, single-host deployment and current Go/Ent/Gin/Next stack.
- Reuse the existing session issuance, transaction manager, outbox, audit, security-email dispatcher,
  settings and lifecycle infrastructure.
- Provider I/O never occurs inside a request transaction.
- All collections, batches, messages and input fields are bounded.
- No plaintext address/token/password/ciphertext in logs, metrics, audit, URLs after bootstrap or FE
  persistence.
- Maintain normal and strict package-boundary gates; no cross-domain Identity imports.
- Do not commit.

## Phase 1 — contracts and cryptography

- [ ] Add failing unit/fuzz tests for email normalization/masking, token parsing, key rotation and AAD
  transplant rejection.
- [ ] Implement `RecoveryContactProtector` with domain-separated keyring operations and explicit
  typed errors.
- [ ] Add verification/reset token hash-key IDs and correct contact/delivery AAD metadata.
- [ ] Define masked projections, command DTO limits, recent-reauth and safe public error catalog.
- [ ] Verify secrets never implement `Stringer` and formatted errors contain no sensitive bytes.

## Phase 2 — transaction-bound recovery repository

- [ ] Add neutral Identity entities/ports for contact state, verification, resend/replacement,
  forgot/reset and dispatcher validity gating.
- [ ] Implement Ent lock order `user -> contacts -> token -> delivery` and optimistic versions.
- [ ] Persist security quotas and mutation idempotency so restart cannot reset them.
- [ ] Add `PrepareForSend` plus fenced cancel/erase transitions to the delivery repository.
- [ ] Prove one verified/one pending constraints, race winner behavior and stale-intent cancellation
  with PostgreSQL integration tests.

## Phase 3 — registration and verification

- [ ] Extend password registration with required normalized recovery email.
- [ ] Remove password from the registration fingerprint; exact retry must verify the committed
  password credential before receipt recovery.
- [ ] Compose user, credential, onboarding, pending contact, token, encrypted delivery, audit/outbox
  and session issuance in one transaction.
- [ ] Implement public verification service/route with one safe invalid-token result and `no-store`.
- [ ] Implement verification payload codec and provider template without logging recipient/token.
- [ ] Test rollback at every write boundary and concurrent verification/replacement races.

## Phase 4 — authenticated contact management

- [ ] Implement masked `GET /auth/recovery-contact` projection.
- [ ] Implement password reauthentication updating the current session's
  `last_reauthenticated_at` with bounded TTL.
- [ ] Implement idempotent replace, resend, pending-cancel and safe-remove commands with expected
  versions, CSRF/Origin/Fetch Metadata and recent-reauth guards.
- [ ] Keep the previous verified contact active until replacement verification commits.
- [ ] Add route/service/DB tests for authorization, stale versions, quotas and exact retries.

## Phase 5 — forgot/reset correction

- [ ] Delete the Gmail-credential fallback and every implicit contact verify/replace path.
- [ ] Make forgot resolve only an active password credential plus current verified contact.
- [ ] Return uniform 202 responses and enforce comparable missing/ineligible/eligible work plus local
  per-IP and persisted subject quotas.
- [ ] Revalidate reset eligibility before and after password hashing; update only an existing password
  credential.
- [ ] Consume reset token, bump security version, revoke all sessions, cancel siblings/deliveries and
  append audit/outbox atomically.
- [ ] Test enumeration resistance, token single use, session invalidation and OAuth-only rejection.

## Phase 6 — dispatcher and janitor

- [ ] Extend encrypted payload codec/provider adapter for verification messages.
- [ ] Run `PrepareForSend` before decryption/network I/O and cancel invalid intents without provider
  calls.
- [ ] Preserve lease/idempotency fencing across success, timeout, retry, crash and shutdown.
- [ ] Extend the bounded janitor to expire tokens, cancel/erase delivery payloads, revoke/erase pending
  contacts and delete retained terminal rows in FK order.
- [ ] Add CID-aware debug/info/warn logs, low-cardinality health/metrics and redaction tests.

## Phase 7 — frontend product flow

- [ ] Add recovery email to registration state/validation/API types and pending verification result.
- [ ] Add fragment-based contact verification page; erase the fragment immediately and never persist
  the token.
- [ ] Change reset links/pages to the same fragment-to-memory pattern.
- [ ] Build the settings/security card with masked projection, reauth dialog, replace, resend countdown,
  cancel pending and safe-remove behavior.
- [ ] Use only centralized API/routes/text, exact loading/error/empty/terminal states and accessible
  light/dark/system UI.
- [ ] Add static/pure contract tests and browser E2E for registration, verify, replace, resend,
  forgot/reset and session revocation.

## Phase 8 — generation, migration and gates

- [ ] Regenerate Ent and mocks once schema/ports stabilize; consolidate the development baseline only
  after all recovery changes are final.
- [ ] Run focused normal/race/vet tests after each phase.
- [ ] Run strict architecture, generated drift, database baseline, full API/go-common/FE tests,
  production FE build and packaging verifier.
- [ ] Run malicious input, dispatcher crash/restart and browser E2E suites after cleanup.
- [ ] Inspect the complete diff for secret leakage, dead compatibility code, duplicated mappings and
  unbounded queries; remove obsolete Gmail fallback and old query-token flow.

## Acceptance gate

- [ ] A newly registered password account can verify and later safely replace its recovery contact.
- [ ] Forgot/reset works for password accounts and cannot create credentials for OAuth-only accounts.
- [ ] Stale/superseded delivery never calls the provider.
- [ ] Password reset is single-use and revokes all sessions.
- [ ] No raw recovery secret survives in durable plaintext or browser history.
- [ ] All concurrency, crash, redaction, architecture, migration, FE and packaging gates are green.
- [ ] No commit was created.

