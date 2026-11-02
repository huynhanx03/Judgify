# Multi-step Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split password signup and first-time OAuth onboarding into clear steps, remove the signup recovery-email field, and use a verified email from a linked OAuth identity as a protected recovery contact when appropriate.

**Architecture:** Keep account creation and onboarding provisioning atomic at the final submit. Password signup gathers credentials, dynamic profile evidence, and trait selection through a compact, accessible question-page flow; first-time OAuth skips credential entry and uses the same remaining steps. Reuse encrypted `RecoveryContact` storage and password-reset delivery, recording provider provenance so unlinking the OAuth identity can revoke the generated contact. Never match or merge accounts by email; use provider + subject as the identity key.

**Tech Stack:** Go, Ent/PostgreSQL migrations, Next.js/React/TypeScript, existing OAuth completion and recovery-contact lifecycles.

**Spec:** `docs/superpowers/specs/2026-10-18-multi-step-onboarding-design.md`

## Global Constraints

- Do not create a partially onboarded Judgify account.
- Do not store the password in local storage or other persistent browser storage.
- A provider email is eligible only when the OAuth provider marks it verified and the identity has been explicitly linked to the Judgify account.
- If a user already has a verified recovery contact, linking OAuth must not silently replace it.
- If no verified recovery contact exists, password-reset requests keep the same generic public response but do not send an email.
- Provider email must not enter logs or public DTOs.
- Use one active step with a concise “Step n of m” label, Back and Continue. Avoid a clickable map of every step; move focus to the new heading on transition and to the first invalid field after validation errors.
- Accept `email_verified` only through provider adapters that validate the claim. Keep provider + subject as the federated identity key; treat provider email as a recovery contact attribute, never an account identifier.
- Do not commit changes.

## Review Focus

- OAuth provider omits email or says it is unverified: account linking succeeds without creating a recovery destination.
- An account already has a verified manually provided recovery contact: OAuth linking preserves it.
- An OAuth identity that supplied a recovery contact is unlinked: only that source-backed contact is revoked.
- Provider password registration cannot create an account until profile and character steps are valid.
- OAuth first-time registration skips account credentials, while existing OAuth login skips onboarding entirely.

---

### Task 1: Add OAuth provenance to protected recovery contacts

**Files:**
- Modify: `api/internal/identity/domain/recovery_contact.go`
- Modify: `api/internal/ent/schema/user_recovery_contact.go`
- Modify: `api/internal/ports/identity.go`
- Modify: `api/internal/identity/adapters/driven/db/recovery_contact.go`
- Modify: `api/internal/identity/usecase/recovery_contact.go`
- Create: `api/internal/ent/migrate/migrations/20261018090000_oauth_recovery_contact_source.sql`
- Generate: `api/internal/ent/generate/**`
- Modify: existing recovery-contact test files only where their fixtures or assertions require the new optional source field/purpose

**Interfaces:**
- Add optional `SourceFederatedIdentityID *uuid.UUID` to `identity/domain.RecoveryContact` and persisted `user_recovery_contacts` rows.
- Add a closed `oauth_link` recovery-contact purpose.
- Add lifecycle operations to prepare/persist a provider-verified contact and revoke contacts by source federated identity ID. Preparation accepts `(userID, identityID uuid.UUID, email string, emailVerified bool)`; unverified or empty email returns no prepared contact and no error.
- Name the use-case methods `PrepareOAuthVerifiedContact(userID, identityID uuid.UUID, email string, emailVerified bool) (*ports.PreparedOAuthRecoveryContact, error)`, `PersistOAuthVerifiedContact(ctx context.Context, prepared *ports.PreparedOAuthRecoveryContact) error`, and `RevokeOAuthRecoveryContact(ctx context.Context, userID, identityID uuid.UUID, at time.Time) error`.
- Add `RecoveryContactStore.CreateOAuthVerifiedIfAbsent(ctx context.Context, contact entity.RecoveryContact) (bool, error)` so a concurrent/manual verified contact wins and is never replaced.
- Provider-sourced contacts use the existing email protector, keyed lookup/fingerprint hashes, and contact transaction boundaries. They are already verified only when the provider claim is verified and the identity link/finalization transaction succeeds.

- [ ] Add nullable source identity ID, the `oauth_link` purpose, and its FK/check constraints to the Ent schema and migration.
- [ ] Update the recovery-contact entity, repository mapper, create/update/query builders, and store interface for optional source provenance.
- [ ] Add a `RecoveryContactService` preparation/persistence path for verified OAuth email that seals the address and never returns plaintext in repository commands.
- [ ] Prepare ciphertext, hashes, contact IDs, and any new user/federated-identity IDs before opening account transactions; only persist the prepared contact inside the OAuth registration/link transaction.
- [ ] Add a repository operation that revokes only an active OAuth-sourced contact for a given user and federated identity.
- [ ] Regenerate Ent code with `cd api && go generate ./internal/ent`; inspect generated diff to preserve unrelated pre-existing edits.
- [ ] Run `gofmt` on changed Go files and `cd api && go build ./internal/identity/...`.

### Task 2: Wire OAuth registration, linking, and unlinking to recovery

**Files:**
- Modify: `api/internal/identity/usecase/oauth_password.go`
- Modify: `api/internal/identity/usecase/oauth_durable_lifecycle.go`
- Modify: `api/internal/identity/usecase/oauth_identity_management.go`
- Modify: `api/internal/identity/usecase/authentication.go` and identity lifecycle ports as needed
- Modify: existing OAuth/recovery test assertions only if they encode the superseded policy

**Interfaces:**
- OAuth finalization passes the provider's email and `EmailVerified` claim only to the protected recovery-contact lifecycle.
- Allocate the new user/federated identity UUIDs before the transaction; use the same federated identity UUID in the prepared contact and identity insert. Existing identities use their persisted UUID.
- Existing verified recovery contacts are authoritative and are never overwritten by OAuth.
- Recovery-contact seeding is prepared before the transaction and persisted with `CreateOAuthVerifiedIfAbsent` in the same transaction as the OAuth identity/user mutation.

- [ ] In first-time OAuth registration and explicit account linking, after the federated identity has a persisted ID, seed a provider-sourced recovery contact only when the provider email is verified and no verified recovery contact already exists.
- [ ] Do not seed provider email during ordinary later OAuth logins, so a user who removed a recovery contact does not have it silently re-added; allow explicit relinking or account settings to configure recovery again.
- [ ] Keep OAuth email transient outside the encrypted recovery-contact path; do not add it to `FederatedIdentity`, security-journal events, or response DTOs.
- [ ] Revoke the source-backed contact in the same transaction as unlinking its OAuth identity; preserve manual contacts.
- [ ] Preserve the generic forgot-password response and existing verified-contact lookup, so accounts without any verified contact do not reveal that fact.
- [ ] Run `gofmt` and `cd api && go build ./internal/identity/...`.

### Task 3: Remove recovery email from password registration contract

**Files:**
- Modify: `api/internal/identity/adapters/driver/http/register_dto.go`
- Modify: `api/internal/identity/adapters/driver/http/register_mapper.go`
- Modify: `api/internal/identity/contract/authentication.go`
- Modify: `api/internal/identity/usecase/authentication.go`
- Modify: `api/internal/identity/usecase/authentication_test.go` existing registration fixtures/fingerprint assertion
- Modify: `website/types/auth.ts`
- Modify: `website/modules/auth/RegisterFlow.tsx`
- Modify: `website/modules/auth/sections/personal-info-section.tsx`
- Modify: registration-only text/constants where needed; preserve recovery-contact UI and invitation/admin registration contracts

**Interfaces:**
- Public `POST /auth/register` and `identity/contract.RegisterRequest` no longer contain `RecoveryEmail`.
- User registration no longer prepares or persists a recovery contact. User creation, profile, role, trait provisioning, audit, and session issuance remain in the same transaction.
- Admin invitations and explicit recovery-contact management retain their recovery email contracts.

- [ ] Remove `recovery_email` from public request DTO, mapper, contract, TypeScript request, and signup validation/payload.
- [ ] Remove only registration's `PrepareRegistration`/`PersistRegistration` call; preserve other recovery-contact lifecycle dependencies and invitation flows.
- [ ] Update existing registration tests/fixtures that bind the now-removed request field. Do not add or execute tests without explicit request.
- [ ] Run `gofmt`, `cd api && go build ./internal/identity/...`, and `cd website && npm run typecheck`.

### Task 4: Build a shared multi-step onboarding presentation

**Files:**
- Create: `website/modules/auth/onboarding-stepper.tsx`
- Modify: `website/modules/auth/RegisterFlow.tsx`
- Modify: `website/modules/auth/sections/personal-info-section.tsx`
- Modify: `website/modules/auth/onboarding-profile-fields.tsx` only if its layout API needs adjustment
- Modify: `website/modules/auth/sections/trait-selection-section.tsx` only if step-level layout needs adjustment
- Modify: `website/constants/text.ts` for step names and navigation labels

**Interfaces:**
- Stepper receives the step list, active step, and completion state; it renders accessible labels/progress and does not own form state.
- Password signup steps are `account -> profile -> character`; credentials remain in component memory only.
- Back/Next preserves current in-memory fields. Forward navigation is gated by the current step's validation; final submit requires the current profile schema evidence and a valid, unexpired 1-root/6-talent offer with exactly 3 selected talents.

- [ ] Add a compact accessible “Step n of m” label and reusable Continue/Back controls following existing design tokens; do not make every step a clickable navigation map.
- [ ] On transitions, move focus to the active page heading and announce step position to assistive technology; on invalid input, show actionable field errors and focus the first invalid field.
- [ ] Render only the active registration section: account credentials, dynamic profile fields, or character selection.
- [ ] Move trait-offer creation to first entry into Character; guard against duplicate requests and preserve current offer expiry/retry handling.
- [ ] Keep user creation on final submit; send username/password, profile evidence, and trait offer selection together.
- [ ] Check formatting/lint for changed frontend files and run `cd website && npm run typecheck`.

### Task 5: Use the shared onboarding steps for first-time OAuth

**Files:**
- Modify: `website/modules/auth/oauth-completion-client.tsx`
- Reuse: `website/modules/auth/onboarding-stepper.tsx`
- Modify: OAuth onboarding copy/constants in `website/constants/text.ts`

**Interfaces:**
- Existing OAuth login continues directly to the arena.
- First-time OAuth starts at Profile and proceeds to Character; provider auth is the completed Account step. Do not show username/password fields or create a password credential.
- Final OAuth request still carries profile schema evidence and trait selection in the existing idempotent completion command.

- [ ] Replace the single dense completion form with a two-step Profile and Character flow; do not count the skipped Judgify credential step as completed.
- [ ] Request the trait offer only when entering Character; retain the OAuth completion deadline as an upper bound on offer expiry.
- [ ] Preserve the existing completion ticket, retry, refresh-capability, and idempotency behavior.
- [ ] Run `cd website && npm run typecheck` and check changed frontend files with ESLint.

### Task 6: Review integration and working-tree diff

**Files:**
- Review all files changed in Tasks 1–5; no additional unrelated cleanup.

- [ ] Confirm account creation remains atomic and no code path provisions a partial account.
- [ ] Confirm recovery email plaintext is not logged, returned, or stored outside the encrypted recovery-contact path.
- [ ] Confirm old verified contacts, invitation/admin recovery contacts, and OAuth existing-user login retain their current behavior.
- [ ] Inspect `git diff` and `git status`; preserve unrelated pre-existing user changes and do not commit.
- [ ] Report build/typecheck results and any checks not run.
