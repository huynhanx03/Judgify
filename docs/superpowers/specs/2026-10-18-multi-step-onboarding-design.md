# Multi-step onboarding and OAuth recovery design

## Goal

Reduce the density of Judgify's registration and first-time OAuth completion
screens by splitting account setup, profile details, and character selection into
clear steps. Do not create a partially onboarded Judgify account. Remove the
recovery-email field from public password registration and use a deliberately
linked OAuth identity's provider-verified email as the recovery destination
when the account has no other verified recovery contact.

## Current behavior

- Password registration currently puts username, password, recovery email,
  dynamic profile fields, and the trait offer/selection in one form. One request
  creates the user and provisions onboarding data transactionally.
- First-time OAuth completion already skips username/password entry. It gathers
  dynamic profile fields and trait selection before finalizing account creation.
- Existing OAuth users sign in without onboarding.
- OAuth provider email is intentionally not persisted as a recovery authority.
  Password reset resolves only a verified `RecoveryContact`.

## BeGroup pattern considered

The checked-out BeGroup monorepo does not include the `be-onboarding` service
implementation. Its `be-autos` integration checks driver-document completion
using driver, vehicle type, and city, and schedules reminders for drivers who
still need to upload documents. Migration data also shows required documents
and vehicle document forms change by city/vehicle context.

The transferable lesson is to make requirements and progress explicit and
context-aware. Judgify already has dynamic required profile schema and explicit
trait-offer/selection validation, so this design adds a visible step indicator
and completion gating. It does not add persisted incomplete-account reminders:
Judgify creates the user only after this short onboarding completes, unlike the
BeGroup driver flow that follows up after driver registration.

## External patterns applied

- GOV.UK's question-page guidance supports grouping a journey into focused
  pages, with a clear heading, a back link, and a continue action. It recommends
  adding a progress indicator only when it helps, and cautions against a large
  clickable step map that is hard to use on small screens and conditional flows.
- W3C WAI's multi-page form guidance recommends communicating the current step,
  keeping the entered data when returning to earlier steps, and making progress
  understandable to assistive technology.
- OWASP's password-reset guidance recommends generic responses for existing
  and unknown accounts, consistent response timing, rate limits, and secure,
  single-use, expiring reset tokens. Judgify should retain its existing
  enumeration-safe reset contract and protected contact storage.
- OIDC defines `email_verified` as the provider having taken steps to confirm
  control of the address at the time it verified it; the verification method is
  context-specific. Google additionally says email can change and must not be
  used as the identity key; use the provider subject (`sub`). Therefore Judgify
  must bind recovery provenance to the linked provider identity, never match or
  merge accounts by email, and only accept verified-email claims from providers
  whose adapter validates that claim. An existing manually verified recovery
  contact remains authoritative. Provider email remains encrypted and
  revocable when its identity is unlinked.

The resulting UI uses a compact textual step label (for example, “Step 2 of 3”)
and one active step at a time, with Back and Continue. It does not use an
all-steps clickable wizard. On step changes, move keyboard focus to the page
heading and expose the current step to screen readers; on validation errors,
show field-level text and focus the first invalid field. Keep entries in memory
across Back/Continue, but do not persist credentials in browser storage.

## User flow

### Password registration

1. **Account**: collect username and password. Do not ask for a recovery email.
2. **Profile**: collect required, revisioned profile fields.
3. **Character**: issue the durable trait offer (one root bone and six talents),
   then require selection of three talents.
4. **Create account**: submit all gathered evidence, credentials, and selection
   together. The account is created only after the final step succeeds.

Back navigation preserves in-memory form state. Do not store the password in
local storage or other persistent browser storage. A page reload may restart
the registration flow.

### OAuth

- An existing OAuth account continues straight to the arena.
- A first-time OAuth user skips **Account** and starts at **Profile**, then goes
  to **Character**. The OAuth provider authenticates the user; Judgify generates
  a unique username from provider profile data as it does today. No password
  credential is created unless the user separately sets one later.
- Finalizing OAuth continues to create the Judgify user and provision onboarding
  data in the existing transaction.

### Progress display

- Show a compact current-step label and heading; use Back and Continue controls
  instead of a clickable map of all steps. Disable Continue until the current
  step is valid.
- Password registration displays three steps. First-time OAuth displays two
  steps (Profile and Character) because it skips credentials; do not imply that
  a separate Judgify password account step was completed.
- On step change, move focus to the new heading and announce the current step.
  When validation fails, show actionable field-level errors and focus the first
  invalid field.

## Recovery email policy

- Remove `recovery_email` from the public password-registration request and stop
  requiring a recovery contact during that registration transaction.
- A provider email is eligible only when the OAuth provider marks it verified,
  its adapter validates that claim, and the provider identity is attached to the
  Judgify account (the initial identity attached during first-time OAuth
  registration counts). Use provider + subject as identity; never match or
  merge accounts by email. The verified claim reflects the provider's
  verification at a point in time, so retain provenance and revoke the
  provider-sourced contact when that identity is unlinked.
- Reuse the protected recovery-contact lifecycle and encrypted storage rather
  than returning or logging provider email. Record enough provenance to revoke
  a provider-sourced recovery contact when its OAuth identity is unlinked.
- If a user already has a verified recovery contact, linking OAuth must not
  silently replace it. If there is no verified contact, the provider identity
  attached during first-time OAuth registration or an explicitly linked
  identity can supply one when its email is verified. Do not seed a contact
  during ordinary later logins: a user who removes a recovery contact must not
  have it silently re-added. They can explicitly relink the provider or
  configure recovery in account settings.
- If no verified recovery contact exists, password-reset requests keep the same
  generic public response but do not send an email. The user can still sign in
  through a linked OAuth identity and can configure a recovery contact from
  account settings.
- Keep existing verified recovery contacts working. This change does not alter
  admin-created or invited-account requirements.

## Implementation boundaries

- Extract/reuse step presentation and navigation for password registration and
  first-time OAuth completion, while keeping their credential/finalization
  handlers separate.
- Preserve the current profile-schema evidence checks, trait-offer expiry and
  checksum validation, OAuth completion ticket/idempotency rules, and atomic
  account provisioning.
- Update the recovery-contact domain, persistence schema/migration, OAuth
  registration/link/unlink lifecycle, and password-reset eligibility to support
  verified provider-sourced contacts without storing provider email in logs or
  public DTOs.
- Update the existing test that asserts OAuth email is never recovery authority
  to assert the new verified-and-linked-only rule; cover missing/unverified
  provider email and unlink behavior.

## Failure and recovery behavior

- Moving between steps is local UI state; no account or reward profile is
  provisioned before final submission.
- Existing durable idempotency and exact-retry behavior remain authoritative for
  registration and OAuth finalization.
- If provider email is missing or unverified, OAuth registration/linking still
  succeeds, but does not create a recovery destination.
- Unlinking an OAuth identity revokes a recovery contact sourced from that
  identity. It must not revoke a manually supplied contact or disclose whether
  a username has a recovery destination during forgot-password requests.

## Acceptance criteria

- The registration page exposes one step at a time and has working Next/Back
  navigation; the trait offer is requested only when entering Character.
- First-time OAuth skips Account and uses the same Profile and Character steps;
  existing OAuth login skips onboarding.
- The final request still creates the account and initial profile/reward state
  atomically.
- Public password registration no longer asks for or sends a recovery email.
- Only verified provider email attached to a deliberately linked OAuth identity
  can seed recovery. Existing verified contacts remain unchanged, and
  unlinking revokes only the provider-sourced contact.
- Password reset remains enumeration-safe and sends only to an eligible,
  protected recovery contact.

## Review note

This changes a security boundary: verified OAuth email becomes eligible to seed
password recovery only after explicit linking. It retains the current rule that
an unverified provider email is never recovery authority.

## Research references

- [GOV.UK Design System: Question pages](https://design-system.service.gov.uk/patterns/question-pages/)
- [W3C WAI: Multi-page forms](https://www.w3.org/WAI/tutorials/forms/multi-page/)
- [OWASP: Forgot Password Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html)
- [OpenID Connect Core 1.0, Standard Claims](https://openid.net/specs/openid-connect-core-1_0.html#StandardClaims)
- [Google: OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect)
