# Administrative User Invitation Implementation Plan

1. Extend closed user/contact/token/email state and Ent relationships for
   `invited` and `account_invitation` without weakening existing invariants.
2. Add invitation DTOs, contracts, repository ports and strict durable outbox
   payload validation.
3. Implement protected preparation, transactional admin preview/apply,
   receipt replay, activation, resend and revocation behavior.
4. Extend the encrypted payload codec, delivery adapter, dispatcher and config
   with an invitation template/URL while preserving secret-redaction rules.
5. Register protected/public HTTP routes, permissions, DI, observation and
   post-commit realtime projection.
6. Replace the admin password-create UI with invitation review/apply, add
   invited-state actions, and build the public password-setting page using the
   central Vietnamese catalog and strict runtime schemas.
7. At final source freeze, generate Ent/mocks/migrations, format, and execute
   focused security tests followed by the complete product verification gate.

No commits are created. Test/build/generate/format steps remain deferred until
the product source freeze requested by the user.
