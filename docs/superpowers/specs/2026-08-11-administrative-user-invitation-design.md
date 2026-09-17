# Administrative User Invitation Design

## Goal

Replace administrative password-based account creation with a durable,
reviewed invitation flow. An administrator never chooses, receives, logs, or
transmits the user's password. The invitee proves control of the recovery
address and sets their own password before the account becomes usable.

## Chosen architecture

Judgify reuses the existing recovery-contact token protection, encrypted
security-email payload, PostgreSQL delivery queue, leased email dispatcher,
audit log, outbox, recent-authentication gate, and HMAC confirmation workflow.
No second token framework, broker, cache, or service is introduced.

An invited account is persisted with the closed security status `invited`.
That state cannot authenticate, start password recovery, appear in public
profile reads, or receive a browser session. It reserves the username and lets
administrators inspect and revoke the pending account safely.

## Administrative command

The admin client sends username, recovery email, role keys, typed profile
schema evidence, an audit reason, and one UUIDv7 command identity. It never
sends a password.

1. `POST /users/invitations/preview` normalizes the complete request, validates
   active roles and the server-owned profile schema, and returns a short-lived
   HMAC confirmation bound to actor, live session, command ID, normalized
   recovery-address fingerprint, roles, profile evidence, and reason.
2. `POST /users/invitations/apply` requires the same command, confirmation and
   recent authentication. It prepares all randomness and cryptography before
   opening the transaction.
3. One transaction creates the `invited` user, typed profile values, direct
   role bindings, pending recovery contact, one-time invitation token,
   encrypted email delivery intent, audit entry, and outbox event.
4. A duplicate apply loads the exact durable receipt by command ID and rejects
   any cross-actor, cross-session, or request-hash mismatch.

The response contains only a safe account projection, invitation expiry, and
delivery state. It never contains the address, address fingerprint, token,
token hash, ciphertext, or credential material.

## Invitation activation

The email worker selects an invitation template and URL from the closed email
kind `account_invitation`. Recipient and raw token exist only inside the
existing AEAD payload and the provider call boundary.

`POST /auth/invitations/activate` accepts the opaque token and a new password.
The service hashes the password before opening the transaction, resolves every
active token-hash key to support key rotation, and locks the token, contact,
and invited user. One transaction then:

- creates the user's sole active password credential;
- consumes the one-time invitation token;
- marks the recovery contact verified;
- changes user status from `invited` to `active` and advances the security
  version;
- writes an audit entry and `identity.user.invitation.accepted.v1` outbox event.

The endpoint does not create a session. The user signs in through the normal
password flow after activation, keeping session issuance and replay semantics
in one existing path. A lost successful activation response can be recovered
only by presenting the same token and a password that verifies against the
new credential; token knowledge alone is insufficient.

## Revocation and expiry

The existing reviewed user-delete lifecycle accepts `invited -> deleted` and
atomically revokes the invitation token, queued delivery, pending contact, and
role bindings. Expired invitations remain non-authenticating and visible to
administrators. A reviewed resend command supersedes the old token and
delivery, retains the account/profile/roles, and queues a fresh encrypted
delivery with rate limits and a new expiry.

## UI contract

The admin create dialog becomes an invitation composer:

- username, recovery email, roles and dynamic profile fields;
- mandatory audit reason;
- server review screen showing the normalized username, masked destination,
  roles, profile-field count, expiry and immutable-command notice;
- recent-authentication prompt only when the apply gate requires it;
- conflict recovery that reloads current schema/role catalogs without losing
  compatible draft values.

Invited accounts have a distinct status badge and actions to resend or revoke
the invitation. The public activation page has token-invalid, expired,
already-accepted, password-invalid, submitting and success states, with all
Vietnamese copy in the typed text catalog and no token stored in persistent
browser storage.

## Security and data invariants

- Only `active` users may authenticate or receive sessions.
- Invitation, verification and password-reset purposes/kinds are closed enums.
- Every secret token has at least 256 bits of entropy and only keyed hashes are
  stored.
- Recipient plaintext and token plaintext never enter logs, audit, outbox, or
  unencrypted database fields.
- Invitation email is sent only if the token/contact/user version fence is
  still live immediately before provider I/O.
- Username, role set, profile evidence, actor, session, reason and command ID
  are all confirmation-bound.
- Activation and revocation are mutually exclusive under row locks.
- All state changes are transaction-bound and observable by CID plus bounded
  `user_id`, `command_id`, `event_id`, and `delivery_id` fields.

## Verification deferred to source freeze

After all product source is complete: regenerate Ent and mocks, reconcile the
baseline/Atlas migration, format Go and TypeScript, then run unit, integration,
race, build, browser contract, accessibility, dark/light, and E2E tests.
