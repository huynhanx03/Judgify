# Judgify Identity and Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stateless browser tokens and non-atomic policy writes with recoverable cookie sessions, verified recovery contacts, secure OAuth completion, and atomic revisioned Casbin authorization that administrators can manage safely.

**Architecture:** go-common verifies credentials and evaluates immutable Casbin snapshots; Judgify Identity owns users, credentials, sessions, recovery, role/policy persistence, invariants, and security events. All cross-module onboarding and policy mutations use a transaction-scoped `UnitOfWork`; successful domain mutation, audit, and outbox state commit together.

**Tech Stack:** Go 1.26.5, Gin, Ent, PostgreSQL, Casbin v3, pinned `github.com/casbin/ent-adapter v1.4.0`, go-common authentication/authorization/password/HTTP packages, RSA access JWTs, opaque rotating refresh tokens, AEAD-protected recovery receipts, and Next.js cookie clients.

## Global Constraints

- Single tenant; the Casbin model has no tenant/domain column.
- Access JWT lifetime defaults to 15 minutes with at most 60 seconds clock skew.
- Session idle lifetime defaults to 30 days; absolute lifetime defaults to 90 days.
- Recent reauthentication defaults to ten minutes.
- Refresh and issuance recovery ciphertext defaults to a two-minute recovery window.
- Password-reset tokens expire after 15 minutes.
- Session cache entries never outlive the access token and have at most 30 seconds defensive TTL.
- Browser tokens use `Secure`, `HttpOnly`, same-origin cookies; JavaScript never reads them.
- CSRF and Fetch Metadata protect every cookie-authenticated unsafe method and anonymous session-establishing command.
- A presented invalid credential never downgrades to anonymous.
- Password login recovery always verifies the password again.
- Email is a recovery contact, not the login/account identity key.
- At least one active user always retains protected `superadmin`.
- Casbin persistence changes are complete-snapshot replacements in one Ent transaction; AutoSave is disabled.
- API/worker runtime credentials have no DDL permission and the Ent Casbin adapter performs no startup schema creation.
- No commits; preserve all current user changes.

---

## File Structure

```text
api/internal/
├── platform/
│   ├── transaction/
│   └── audit/
├── application/workflow/onboarding/
├── authctx/
└── identity/
    ├── core/
    │   ├── entity/
    │   ├── dto/
    │   └── service/
    ├── ports/
    ├── adapters/
    │   ├── driven/db/
    │   └── driver/http/
    └── di/
```

Ent schemas are added under `api/internal/ent/schema`; generated files are regenerated and never hand-edited.

### Task 1: Add the Transaction-Scoped Unit of Work and Shared Audit Appender

**Files:**
- Create: `api/internal/platform/transaction/unit_of_work.go`
- Create: `api/internal/platform/transaction/ent_unit_of_work.go`
- Create: `api/internal/platform/transaction/context.go`
- Create: `api/internal/platform/transaction/ent_unit_of_work_test.go`
- Create: `api/internal/platform/audit/entry.go`
- Create: `api/internal/platform/audit/appender.go`
- Create: `api/internal/platform/audit/repository.go`
- Create: `api/internal/platform/audit/repository_test.go`
- Create: `api/internal/ent/schema/audit_log.go`
- Create: `api/internal/platform/eventing/event.go`
- Create: `api/internal/platform/eventing/outbox.go`
- Create: `api/internal/platform/eventing/outbox_test.go`
- Create: `api/internal/ent/schema/outbox_event.go`
- Modify: `api/internal/ent/generate.go`

**Interfaces:**

```go
type UnitOfWork interface {
    Within(ctx context.Context, fn func(txCtx context.Context) error) error
}

type Appender interface {
    Append(ctx context.Context, entry Entry) (uuid.UUID, error)
}

type Entry struct {
    ID            uuid.UUID
    ActorUserID   *uuid.UUID
    ActorSessionID *uuid.UUID
    Action        string
    Resource      string
    ResourceID    string
    Reason        string
    Before        map[string]any
    After         map[string]any
    CorrelationID string
    OccurredAt    time.Time
}

type Event struct {
    ID               uuid.UUID
    Type             string
    AggregateType    string
    AggregateID      uuid.UUID
    AggregateVersion int64
    CorrelationID    string
    CausationID      *uuid.UUID
    Payload          json.RawMessage
    OccurredAt       time.Time
}

type OutboxAppender interface {
    Append(ctx context.Context, events ...Event) error
}
```

- [ ] **Step 1: Write failing transaction tests**

Use an Ent test client and assert commit on nil, rollback on error, rollback plus re-panic after cleanup on panic, commit-error preservation, nested-transaction rejection, context invalid after callback, and no transaction client visible through a public/domain API.

Audit tests reject empty action/resource/reason, redact configured keys, cap payload bytes/depth, and prove audit row rollback with its enclosing mutation.

Outbox tests require transaction context, bounded closed event metadata, unique aggregate event identity, deterministic JSON bytes, and rollback with the enclosing mutation. The schema includes pending/leased/published/dead dispatch state, attempt/lease/backoff timestamps, and indexes needed by the later dispatcher.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/platform/transaction ./internal/platform/audit ./internal/platform/eventing -count=1
```

Expected: FAIL because the platform packages and schema do not exist.

- [ ] **Step 3: Implement the private Ent transaction context**

Only database adapters call:

```go
func Client(ctx context.Context, fallback *ent.Client) (*ent.Client, error)
```

The private context value stores a transaction-scoped client plus an atomic valid flag. `Within` invalidates it before return. Application/domain contracts see only `context.Context`; nested `Within` returns `ErrNestedTransaction`.

- [ ] **Step 4: Implement append-only audit persistence**

Serialize only redacted bounded JSON. `Append` requires the transaction-scoped client for state-changing operations and returns `ErrTransactionRequired` otherwise. The schema has immutable created fields and no update/delete repository method.

Implement `eventing.OutboxAppender` on the same transaction-bound client. Domain modules may append versioned events now; delivery, inbox, and realtime projection are added later without changing this call site.

- [ ] **Step 5: Generate and verify**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/platform/transaction ./internal/platform/audit ./internal/platform/eventing -count=1
go test ./...
git diff --check
```

Expected: all exit 0 and generated changes originate from schema only. Do not commit.

### Task 2: Expand the Identity Security Schema

**Files:**
- Modify: `api/internal/ent/schema/user.go`
- Modify: `api/internal/ent/schema/credential.go`
- Modify: `api/internal/ent/schema/federated_identity.go`
- Modify: `api/internal/ent/schema/role.go`
- Modify: `api/internal/ent/schema/attribute_definition.go`
- Modify: `api/internal/ent/schema/user_attribute_value.go`
- Create: `api/internal/ent/schema/auth_session.go`
- Create: `api/internal/ent/schema/auth_refresh_token.go`
- Create: `api/internal/ent/schema/auth_refresh_receipt.go`
- Create: `api/internal/ent/schema/auth_session_issuance_receipt.go`
- Create: `api/internal/ent/schema/user_recovery_contact.go`
- Create: `api/internal/ent/schema/recovery_contact_verification_token.go`
- Create: `api/internal/ent/schema/password_reset_token.go`
- Create: `api/internal/ent/schema/security_email_delivery.go`
- Create: `api/internal/ent/schema/oauth_flow.go`
- Create: `api/internal/ent/schema/oauth_completion_ticket.go`
- Create: `api/internal/ent/schema/attribute_definition_revision.go`
- Create: `api/internal/ent/schema/authorization_state.go`
- Create: `api/internal/ent/schema/authorization_resource.go`
- Create: `api/internal/ent/schema/authorization_action.go`
- Create: `api/internal/identity/adapters/driven/db/schema_invariants_test.go`
- Create: `api/internal/ent/migrate/migrations/20260718000100_identity_security_expand.sql`
- Modify: `api/internal/ent/migrate/migrations/atlas.sum`

**Interfaces:**
- Produces additive UUID-based tables and constraints defined in the approved identity spec.

- [ ] **Step 1: Write schema invariant tests before generation**

Use Ent schema descriptors and PostgreSQL integration assertions for:

- one current verified plus one pending recovery email per user;
- unique refresh-token HMAC;
- unique refresh receipt `(session_id, presented_token_id, idempotency_key)`;
- purpose-specific issuance-receipt uniqueness;
- OAuth `(provider, provider_subject)` uniqueness;
- one `authorization_state` row;
- canonical Casbin row uniqueness;
- indexes for active sessions, family, expiry, security-email leases, recovery fingerprints, and revision polling;
- check constraints for every closed status/purpose.

- [ ] **Step 2: Run invariant tests**

Run: `cd api && go test ./internal/identity/adapters/driven/db -run TestSchemaInvariants -count=1`

Expected: FAIL because new schema types are absent.

- [ ] **Step 3: Implement additive Ent schemas**

Use UUID IDs and UTC timestamps. Store only:

- HMACs for refresh/reset/verification/OAuth one-time secrets;
- AEAD ciphertext plus key ID for short-lived token/contact recovery;
- normalized user-agent display text and HMAC IP, never raw security IP;
- optimistic versions on contacts, definitions, and policy commands.

No raw access/refresh/reset/verification/OAuth credential column exists.

- [ ] **Step 4: Generate an additive migration**

Run the repository’s Atlas/Ent generation path. Inspect SQL for destructive `DROP`, table rewrite, runtime DDL, nullable-to-required without backfill, or bigint IDs. Add explicit partial unique indexes and checks Atlas cannot express through Ent annotations.

- [ ] **Step 5: Verify migration from zero and upgrade fixture**

Run:

```bash
cd api
go generate ./internal/ent
go test ./internal/identity/adapters/driven/db -run TestSchemaInvariants -count=1
go test -tags=integration ./internal/ent/migrate/... -run 'Test(MigrationFromZero|IdentityUpgrade)' -count=1
```

Expected: schema and migration tests exit 0. Do not commit.

### Task 3: Implement Session Authentication and Bounded Cache Revalidation

**Files:**
- Modify: `api/internal/authctx/authctx.go`
- Create: `api/internal/identity/core/entity/session.go`
- Create: `api/internal/identity/core/entity/refresh_token.go`
- Create: `api/internal/identity/core/dto/session.go`
- Create: `api/internal/identity/ports/session.go`
- Create: `api/internal/identity/ports/token.go`
- Create: `api/internal/identity/adapters/driven/db/session.go`
- Create: `api/internal/identity/adapters/driven/db/refresh_token.go`
- Create: `api/internal/identity/core/service/session_authenticator.go`
- Create: `api/internal/identity/core/service/session_authenticator_test.go`
- Modify: `api/internal/identity/core/service/cache_service.go`
- Modify: `api/internal/identity/di/di.go`

**Interfaces:**

```go
type SessionAuthenticator interface {
    Verify(ctx context.Context, raw string, expected authentication.TokenPurpose) (authentication.Principal, error)
}

type SessionRepository interface {
    GetActiveForAuthentication(ctx context.Context, sessionID, userID uuid.UUID) (SessionSecurityState, error)
    TouchLastSeen(ctx context.Context, sessionID uuid.UUID, observedAt time.Time) error
    Revoke(ctx context.Context, sessionID uuid.UUID, reason string, at time.Time) error
    RevokeAll(ctx context.Context, userID uuid.UUID, reason string, at time.Time) error
}
```

- [ ] **Step 1: Write authentication-state tests**

Test active, idle-expired, absolute-expired, revoked, suspended/deleted user, security-version mismatch, cache miss, cache invalidation, 30-second maximum TTL, earliest `AuthenticatedUntil`, PostgreSQL outage, and access/refresh purpose separation.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/identity/core/service -run TestSessionAuthenticator -count=1`

Expected: FAIL because service/repositories do not exist.

- [ ] **Step 3: Implement verification order**

The authenticator:

1. cryptographically verifies a `PurposeAccess` JWT through go-common;
2. parses UUID subject/session only after verification;
3. loads canonical session/user security state from bounded cache or PostgreSQL;
4. rejects non-active/expired/version-mismatched state;
5. returns the go-common principal with `AuthenticatedUntil` equal to the earliest valid boundary;
6. schedules a bounded last-seen touch outside the authentication decision without treating touch failure as successful state mutation.

- [ ] **Step 4: Implement synchronous invalidation hooks**

Logout, revoke, password change/reset, suspension/deletion, and security-version changes invalidate affected local entries only after database commit. A failed commit leaves entries intact. Cross-process invalidation is represented by an outbox event for the realtime plan.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/identity/core/service -run TestSessionAuthenticator -count=100
go test -race ./internal/identity/core/service -run TestSessionAuthenticator -count=10
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 4: Build Recoverable Session Issuance for Login and Registration

**Files:**
- Create: `api/internal/identity/core/entity/issuance_receipt.go`
- Create: `api/internal/identity/ports/issuance_receipt.go`
- Create: `api/internal/identity/adapters/driven/db/issuance_receipt.go`
- Create: `api/internal/identity/core/service/session_issuance.go`
- Create: `api/internal/identity/core/service/session_issuance_test.go`
- Modify: `api/internal/identity/core/service/authentication.go`
- Modify: `api/internal/identity/core/service/oauth_password.go`
- Modify: `api/internal/identity/ports/authentication.go`
- Modify: `api/internal/identity/core/dto/authentication.go`
- Create: `api/internal/application/workflow/onboarding/register.go`
- Create: `api/internal/application/workflow/onboarding/register_test.go`

**Interfaces:**

```go
type IssueCommand struct {
    Purpose         string
    UserID          uuid.UUID
    PreSessionID    uuid.UUID
    CompletionTicketID *uuid.UUID
    IdempotencyKey  uuid.UUID
    RequestFingerprint []byte
    AuthenticatedAt time.Time
    Client          ClientMetadata
}

type SessionBundle struct {
    AccessToken  string
    RefreshToken string
    CSRFToken    string
    SessionID    uuid.UUID
    ExpiresAt    time.Time
}

type Issuer interface {
    Issue(ctx context.Context, command IssueCommand, mutate func(context.Context, PreparedSession) error) (SessionBundle, error)
}
```

- [ ] **Step 1: Write failure-boundary and convergence tests**

Test signer failure before transaction, mutation failure, crash before commit, commit followed by dropped response, exact idempotency recovery, different-key conflict, expired/erased receipt, concurrent same-key convergence, and exactly one session/refresh branch.

Password login recovery must invoke password verification on every attempt. Registration fingerprint covers canonical bounded username/password-proof version, normalized recovery email, required profile values, and immutable trait-offer facts using keyed HMAC—not a plain PII hash.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/identity/core/service ./internal/application/workflow/onboarding \
  -run 'Test(SessionIssuance|Registration)' -count=1
```

Expected: FAIL because the issuance helper/workflow is absent.

- [ ] **Step 3: Implement prepare-before-transaction issuance**

Generate session ID, family ID, refresh record ID, 256-bit opaque refresh bytes, CSRF nonce, and access JWT before entering `UnitOfWork`. Inside one transaction, lock the command identity, recheck existing receipt, invoke the bounded domain mutation, create session/refresh rows, encrypt exact refresh+CSRF bytes with associated data covering the full tuple, create receipt, audit, and outbox intent.

- [ ] **Step 4: Implement login and registration consumers**

Password hash/verification runs outside the transaction. Login rechecks user security state under lock before commit. Registration workflow composes narrow Identity and Cultivation participants; it does not import their repositories or expose Ent types. The Cultivation trait-offer participant is delivered by the cultivation plan; until then, a contract fake drives tests and the production route remains disabled.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/identity/core/service ./internal/application/workflow/onboarding \
  -run 'Test(SessionIssuance|Registration)' -count=100
go test -race ./internal/identity/core/service ./internal/application/workflow/onboarding -count=10
go test ./...
```

Expected: all implemented tests exit 0; production registration is not enabled without the real trait-offer participant. Do not commit.

### Task 5: Implement One-Time Refresh Rotation and Lost-Response Recovery

**Files:**
- Create: `api/internal/identity/core/service/refresh.go`
- Create: `api/internal/identity/core/service/refresh_test.go`
- Create: `api/internal/identity/adapters/driven/db/refresh_receipt.go`
- Create: `api/internal/identity/adapters/driven/db/refresh_receipt_test.go`
- Modify: `api/internal/identity/ports/token.go`

**Interfaces:**

```go
type RefreshCommand struct {
    RawToken      string
    IdempotencyKey uuid.UUID
    Client        ClientMetadata
}

func (s *RefreshService) Rotate(ctx context.Context, command RefreshCommand) (SessionBundle, error)
```

- [ ] **Step 1: Write PostgreSQL race tests**

Assert exactly one concurrent rotation winner, old token becomes consumed, one child token exists, same UUID recovers identical refresh/CSRF bytes, different UUID or expired receipt triggers durable family revocation, transaction failure leaves original issued, session expiry/status/security-version blocks rotation, and reuse revocation invalidates caches only after commit.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/identity/core/service -run TestRefresh -count=1`

Expected: FAIL because rotation service does not exist.

- [ ] **Step 3: Implement locked rotation**

HMAC the presented token before lookup. In `UnitOfWork`, lock token/session/user, classify issued/consumed/revoked, recover exact receipt for same UUID, otherwise revoke the family on reuse. For issued tokens, pre-generated child material is inserted, parent consumed, receipt encrypted, audit/outbox appended, and session last-seen advanced atomically.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/identity/core/service -run TestRefresh -count=50
go test -race ./internal/identity/core/service -run TestRefresh -count=10
```

Expected: exactly one rotation branch in every run. Do not commit.

### Task 6: Add Verified Recovery Contacts and Durable Security Email

**Files:**
- Create: `api/internal/identity/core/entity/recovery_contact.go`
- Create: `api/internal/identity/core/entity/security_email.go`
- Create: `api/internal/identity/ports/recovery_contact.go`
- Create: `api/internal/identity/ports/security_email.go`
- Create: `api/internal/identity/adapters/driven/db/recovery_contact.go`
- Create: `api/internal/identity/adapters/driven/db/security_email.go`
- Create: `api/internal/identity/core/service/recovery_contact.go`
- Create: `api/internal/identity/core/service/password_recovery.go`
- Create: `api/internal/identity/core/service/security_email_worker.go`
- Create: `api/internal/identity/core/service/recovery_contact_test.go`
- Create: `api/internal/identity/core/service/password_recovery_test.go`
- Create: `api/internal/identity/core/service/security_email_worker_test.go`

**Interfaces:**

```go
type EmailSender interface {
    Send(ctx context.Context, message SecurityEmailMessage) (ProviderReceipt, error)
}

type SecurityEmailWorker interface {
    Run(ctx context.Context) error
    Shutdown(ctx context.Context) error
}
```

- [ ] **Step 1: Write recovery and delivery tests**

Cover pending/verified/revoked lifecycle, one verified plus one replacement, HMAC lookup, AEAD address round trip/key rotation, exact version lock, resend superseding old intent, recent-reauth requirement, explicit OAuth verified-email consent, reset enumeration resistance, token single use, all-session revocation, lease claim/reclaim, provider timeout retry with stable idempotency key, cancellation of expired/consumed/replaced intent before provider I/O, dead state, and ciphertext erasure.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/identity/core/service -run 'Test(RecoveryContact|PasswordRecovery|SecurityEmail)' -count=1
```

Expected: FAIL because recovery components do not exist.

- [ ] **Step 3: Implement encrypted contact and one-time tokens**

Normalize bounded email, store HMAC fingerprint and AEAD ciphertext separately, and never return the fingerprint. Verification/reset tokens contain at least 256 bits entropy; store only versioned HMAC. Every contact mutation, token, delivery intent, audit, and outbox row commits in one transaction.

- [ ] **Step 4: Implement leased email worker**

Claim bounded rows with `FOR UPDATE SKIP LOCKED`, unique lease token, expiry, attempts, exponential backoff, and stable provider idempotency key. Before every send, re-open a transaction and revalidate linked token/contact state; mark invalid intents `cancelled` and erase ciphertext without network I/O. Decrypt only immediately around `EmailSender.Send`.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/identity/core/service \
  -run 'Test(RecoveryContact|PasswordRecovery|SecurityEmail)' -count=20
go test -race ./internal/identity/core/service -run TestSecurityEmail -count=10
go test ./...
```

Expected: all exit 0 and captured logs contain no address, fingerprint, or token. Do not commit.

### Task 7: Replace OAuth Side Effects with State, Completion Tickets, and Finalize Commands

**Files:**
- Create: `api/internal/identity/core/entity/oauth_flow.go`
- Create: `api/internal/identity/core/entity/oauth_ticket.go`
- Create: `api/internal/identity/ports/oauth_flow.go`
- Create: `api/internal/identity/adapters/driven/db/oauth_flow.go`
- Create: `api/internal/identity/core/service/oauth.go`
- Create: `api/internal/identity/core/service/oauth_test.go`
- Modify: `api/internal/identity/adapters/driver/http/authentication.go`
- Modify: `api/internal/identity/adapters/driver/http/handler.go`

**Interfaces:**

```go
type OAuthPurpose string

const (
    OAuthLogin OAuthPurpose = "login"
    OAuthLink  OAuthPurpose = "link"
    OAuthReauthenticate OAuthPurpose = "reauthenticate"
)

type StartOAuthCommand struct {
    Provider      string
    Purpose       OAuthPurpose
    PreSessionID  uuid.UUID
    SessionID     *uuid.UUID
    RedirectAfter string
}

type FinalizeOAuthCommand struct {
    Ticket        string
    IdempotencyKey uuid.UUID
    ConsentRecoveryEmail bool
    Onboarding    *OAuthOnboarding
}
```

- [ ] **Step 1: Write route and workflow tests**

Prove initiation is same-origin CSRF/Fetch-Metadata-protected POST, callback is registered top-level GET only, callback consumes state and creates only a hashed one-time completion ticket, finalize is protected POST, ticket purpose/pre-session/session binding is exact, provider subject—not email—is identity, link/reauth require active session and recent reauthentication, final credential cannot be unlinked, and concurrent provider-link uniqueness is enforced.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/identity/core/service ./internal/identity/adapters/driver/http \
  -run 'TestOAuth' -count=1
```

Expected: FAIL because current callback/finalize behavior does not satisfy the route matrix.

- [ ] **Step 3: Implement one-time state and tickets**

Store only state/ticket HMACs with purpose, PKCE verifier ciphertext, pre-session/session binding, redirect allowlist key, expiry, and consumed state. Callback exchanges the code through go-common, stores bounded normalized provider facts in a completion ticket, and redirects with only the raw one-time ticket in URL fragment or another approved non-loggable handoff; it never sets auth cookies.

- [ ] **Step 4: Implement finalize/link/reauth**

Finalize locks and consumes ticket inside the same transaction as provider-link/account/onboarding/session issuance. Existing-user login rechecks active user/security state. New-user finalize uses the onboarding UnitOfWork and exact issuance receipt. Link and reauthenticate bind the ticket to the current session and recheck recent auth at commit.

- [ ] **Step 5: Remove production legacy routes and verify**

Run:

```bash
cd api
go test ./internal/identity/... -run TestOAuth -count=50
go test -race ./internal/identity/core/service -run TestOAuth -count=10
go test ./...
```

Expected: route enumeration shows no start GET or callback POST handler; all tests pass. Do not commit.

### Task 8: Version Profile Attribute Definitions and Values

**Files:**
- Modify: `api/internal/identity/core/entity/attribute_definition.go`
- Modify: `api/internal/identity/core/dto/attribute_definition.go`
- Modify: `api/internal/identity/ports/attribute_definition.go`
- Modify: `api/internal/identity/adapters/driven/db/attribute_definition.go`
- Modify: `api/internal/identity/adapters/driven/db/user_attribute_value.go`
- Modify: `api/internal/identity/core/service/attribute_definition.go`
- Create: `api/internal/identity/core/service/attribute_migration.go`
- Create: `api/internal/identity/core/service/attribute_migration_test.go`
- Modify: `api/internal/identity/adapters/driver/http/attribute_definition.go`
- Modify: `api/internal/identity/adapters/driver/http/profile.go`

**Interfaces:**

```go
type DefinitionKind string
type ValueType string
type Visibility string

type DefinitionRevision struct {
    ID          uuid.UUID
    DefinitionID uuid.UUID
    DataType    ValueType
    Label       string
    Description string
    Visibility  Visibility
    UserEditable bool
    Validation  ValidationSpec
}

type AttributeMigrationCommand struct {
    DefinitionID       uuid.UUID
    ExpectedVersion    int64
    SourceRevisionID   uuid.UUID
    CandidateRevisionID uuid.UUID
    OperationID        uuid.UUID
}
```

- [ ] **Step 1: Write system-definition and migration tests**

Cover immutable normalized key/kind, `required_on_onboarding` independent of kind, protected first/last/gender/birthday matrix, typed validation, optimistic value writes, public/private/admin mapping, bounded resumable shadow migration, concurrent source-version reconciliation, definition drift conflict, invalid-value quarantine, crash before/after pointer switch, and rollback through retained revisions.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/identity/core/service -run TestAttribute -count=1`

Expected: new revision/migration tests fail.

- [ ] **Step 3: Implement immutable revisions and typed values**

Edits create new append-only revisions. Values store canonical typed JSON keyed by `(user_id, attribute_id, definition_revision_id)` plus source version. Ordinary reads join `active_revision_id`; writes require expected value version and current active definition.

- [ ] **Step 4: Implement bounded activation operation**

Backfill candidate shadow values in bounded batches. Before switch, lock definition, reconcile changed source versions, verify every current value or quarantine, then atomically advance pointer with audit/outbox. Resume uses operation ID and captured versions.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/identity/core/service -run TestAttribute -count=20
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 9: Implement Canonical Resource/Action Registry and Policy Snapshots

**Files:**
- Modify: `api/pkg/permissions/constants.go`
- Create: `api/internal/identity/core/entity/authorization.go`
- Modify: `api/internal/identity/core/dto/authorization.go`
- Modify: `api/internal/identity/ports/authorization.go`
- Modify: `api/internal/identity/core/service/authorization.go`
- Modify: `api/internal/identity/core/service/authorization_test.go`
- Create: `api/internal/identity/adapters/driven/db/authorization.go`
- Create: `api/internal/identity/adapters/driven/db/authorization_test.go`
- Create: `api/internal/identity/core/service/authorization_invariants.go`
- Create: `api/internal/identity/core/service/authorization_invariants_test.go`

**Interfaces:**

```go
type PolicySnapshot struct {
    Revision int64
    Roles    []RolePolicy
    UserRoles []UserRoleBinding
}

type ReplaceRolePolicyCommand struct {
    Actor          authentication.Principal
    RoleID         uuid.UUID
    ExpectedRevision int64
    Reason         string
    Resources      []ResourceActions
}

type Preview struct {
    CurrentRevision int64
    Added            []PolicyLine
    Removed          []PolicyLine
    Warnings         []string
}
```

- [ ] **Step 1: Write normalization and invariant tests**

Test closed resource/action registry, immutable normalized role keys, complete replacement semantics, explicit empty replacement, duplicate normalization, wildcards only for protected system roles, at least one active superadmin, self-lockout prevention, protected capability retention, active-user mapping, expected revision, and non-empty audit reason.

- [ ] **Step 2: Run focused tests**

Run: `cd api && go test ./internal/identity/core/service -run TestAuthorization -count=1`

Expected: FAIL on new complete-snapshot and invariant cases.

- [ ] **Step 3: Implement canonical snapshots**

Repository loads roles, active user-role bindings, canonical `casbin_rules`, resource/action registry, and `authorization_state` in deterministic order. Normalize service input into complete Casbin `p`/`g` lines. Preview computes a server-side diff; apply recomputes it and never trusts preview output.

- [ ] **Step 4: Verify**

Run:

```bash
cd api
go test ./internal/identity/core/service ./internal/identity/adapters/driven/db \
  -run TestAuthorization -count=100
go test ./...
```

Expected: all exit 0. Do not commit.

### Task 10: Patch the Ent Casbin Adapter and Add the Linearizable Revision Gate

**Files:**
- Create: `api/internal/identity/adapters/driven/casbin/adapter.go`
- Create: `api/internal/identity/adapters/driven/casbin/adapter_test.go`
- Create: `api/internal/identity/core/service/authorization_runtime.go`
- Create: `api/internal/identity/core/service/authorization_runtime_test.go`
- Create: `api/internal/identity/core/service/authorization_reconciler.go`
- Create: `api/internal/identity/core/service/authorization_reconciler_test.go`
- Modify: `api/internal/identity/core/service/authorization.go`
- Modify: `api/internal/identity/di/di.go`
- Modify: `api/internal/infrastructure/initialized.go`

**Interfaces:**

```go
type AuthorizationRuntime interface {
    authorization.Authorizer
    DesiredRevision() int64
    AppliedRevision() int64
    Healthy() error
}

type RevisionGate struct {
    // process-local RW gate
}
```

- [ ] **Step 1: Write DDL and linearizability tests**

Integration test boots the adapter against an already migrated schema with a DB role lacking `CREATE/ALTER/DROP`. Concurrency test repeatedly applies revision N/N+1 while authorizing and asserts each result is old or new only, no committed stale allow, and stale state fails closed. Crash-after-commit is simulated by skipping publish and starting reconciler.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd api
go test ./internal/identity/adapters/driven/casbin ./internal/identity/core/service \
  -run 'Test(AdapterNoDDL|AuthorizationLinearizable|AuthorizationReconcile)' -count=1
```

Expected: FAIL because adapter/gate/reconciler do not exist.

- [ ] **Step 3: Implement pinned no-DDL adapter path**

Wrap or minimally patch the pinned Ent adapter with `SkipSchemaCreate`. Runtime startup fails if expected migrated tables/constraints are absent; it never repairs schema. AutoSave remains false.

- [ ] **Step 4: Implement mutation order under exclusive gate**

Within one exclusive gate:

1. load and normalize candidate;
2. validate invariants and prepare go-common Casbin snapshot;
3. write complete desired rows, increment revision, audit, outbox, and transactional `NOTIFY`;
4. commit;
5. publish the prepared runtime;
6. update applied state and release gate.

Authorization takes shared gate across stale check, snapshot load, and enforce. Reconciler polls every five seconds in addition to `LISTEN/NOTIFY`, marks stale before reload, and clears stale only after complete publish.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test -tags=integration ./internal/identity/adapters/driven/casbin \
  ./internal/identity/core/service -run 'Test(AdapterNoDDL|Authorization)' -count=50
go test -race ./internal/identity/core/service -run TestAuthorization -count=20
```

Expected: all exit 0, revision mismatch makes readiness false. Do not commit.

### Task 11: Expose Cookie, Session, Recovery, Profile, and Admin Access APIs

**Files:**
- Modify: `api/internal/identity/adapters/driver/http/authentication.go`
- Modify: `api/internal/identity/adapters/driver/http/profile.go`
- Modify: `api/internal/identity/adapters/driver/http/role.go`
- Modify: `api/internal/identity/adapters/driver/http/user.go`
- Modify: `api/internal/identity/adapters/driver/http/handler.go`
- Create: `api/internal/identity/adapters/driver/http/session.go`
- Create: `api/internal/identity/adapters/driver/http/recovery.go`
- Create: `api/internal/identity/adapters/driver/http/oauth.go`
- Create: `api/internal/identity/adapters/driver/http/authorization.go`
- Create: `api/internal/identity/adapters/driver/http/http_test.go`
- Modify: `api/internal/infrastructure/router.go`

**Interfaces:**
- Produces the approved `/api/session`, `/api/auth`, `/api/profile`, and `/api/admin/access` routes with stable response codes and CID metadata.

- [ ] **Step 1: Write route-table and cookie/CSRF tests**

Enumerate methods and paths. Verify cookie flags/path/same-site, no token JSON, CSRF for every unsafe method, generic forgot-password response, bounded pagination/filter/sort, revision conflict 409, runtime error 503/500 not 403, `/admin/login` redirect behavior, and all API errors include CID.

- [ ] **Step 2: Run HTTP tests**

Run:

```bash
cd api
go test ./internal/identity/adapters/driver/http -count=1
```

Expected: FAIL because approved routes and cookie behavior are missing.

- [ ] **Step 3: Implement one response/cookie adapter**

Handlers only validate transport input, call services, set/clear cookies, and map typed errors. Cookies:

- access: `Secure`, `HttpOnly`, `SameSite=Lax`, path `/`;
- refresh: `Secure`, `HttpOnly`, narrow refresh path;
- CSRF/pre-session: signed/opaque as designed, readable only when the CSRF strategy requires it.

Never expose raw tokens, contact ciphertext/fingerprint, reset/verification secrets, Casbin internal errors, or audit sensitive before/after content.

- [ ] **Step 4: Register capability-protected admin routes**

Use go-common authentication and authorization middleware. Services still enforce self/ownership and administrative invariants. `/api/session/me` returns canonical user/session plus effective navigation capability keys, not raw Casbin lines.

- [ ] **Step 5: Verify**

Run:

```bash
cd api
go test ./internal/identity/adapters/driver/http -count=50
go test -race ./internal/identity/adapters/driver/http -count=10
go test ./...
```

Expected: all exit 0 and route enumeration contains no legacy OAuth methods. Do not commit.

### Task 12: Backfill, Switch, and Prove Identity Production Gates

**Files:**
- Create: `api/internal/identity/migration/backfill.go`
- Create: `api/internal/identity/migration/backfill_test.go`
- Create: `api/internal/identity/integration/authentication_test.go`
- Create: `api/internal/identity/integration/authorization_test.go`
- Create: `api/internal/identity/integration/recovery_test.go`
- Create: `api/internal/identity/integration/profile_test.go`
- Modify: `api/internal/infrastructure/initialized.go`
- Modify: `docs/BACKEND_GUIDE.md`

**Interfaces:**
- Produces a coordinated cutover from legacy stateless tokens and row-by-row policy mutation.

- [ ] **Step 1: Write idempotent migration tests**

Seed legacy users/roles/Casbin/profile values; run backfill twice; assert normalized roles/rules, `authorization_state.desired_revision=1`, revision-one protected attributes, invalid-value quarantine, no duplicate mappings, and rollback-safe additive schema.

- [ ] **Step 2: Run migration tests**

Run:

```bash
cd api
go test -tags=integration ./internal/identity/migration ./internal/identity/integration -count=1
```

Expected: FAIL until migration and end-to-end fixtures are implemented.

- [ ] **Step 3: Implement controlled switch**

Deploy new schema/services while old frontend is out of service for coordinated auth cutover. Rotate signing key to invalidate legacy stateless tokens, require login, disable Casbin AutoSave/row mutation, enable readiness only when PostgreSQL/schema and desired/applied authorization revisions match, then remove obsolete token fields/routes in a later contract release.

- [ ] **Step 4: Run security and failure matrix**

Exercise issuance/refresh lost response, reuse, OAuth purpose binding, recovery delivery outage, policy commit/publish crash, final-superadmin race, session revocation/suspension, attribute migration crash, and secret-redaction capture.

- [ ] **Step 5: Verify the complete plan**

Run:

```bash
cd api
go test ./internal/identity/... ./internal/platform/transaction ./internal/platform/audit
go test -race ./internal/identity/... ./internal/platform/transaction
go test -tags=integration ./internal/identity/integration ./internal/identity/migration
go test ./...
git diff --check
```

Expected: every command exits 0, security-critical coverage meets the approved threshold, and readiness fails on a stale authorization revision. Do not commit.

## Plan Acceptance Checklist

- [ ] Login/registration/refresh unknown outcomes converge on exactly one session/token branch.
- [ ] Password login receipt recovery requires password verification again.
- [ ] Recovery email is encrypted, verified, replaceable, and never an account key.
- [ ] Security email intent survives provider failure and never knowingly sends a dead token.
- [ ] OAuth callback creates only a ticket; finalize/link/reauth are purpose-bound protected commands.
- [ ] Access/refresh purpose, issuer/audience/kid/security-version/session state are all enforced.
- [ ] Authorization mutation and runtime publication are linearizable old-or-new snapshots.
- [ ] Ent Casbin adapter starts without DDL privileges and AutoSave is disabled.
- [ ] Admin cannot remove the final superadmin or protected authorization capability.
- [ ] Profile schema/value migrations are revisioned, bounded, resumable, and reversible.
- [ ] Browser/API responses contain no raw token or sensitive contact/policy payload.
- [ ] No commit was created.
