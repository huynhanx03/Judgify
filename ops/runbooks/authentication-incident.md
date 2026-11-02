# Authentication or credential incident

## Symptoms

Unexpected login/refresh/reset failures, suspected token/key disclosure,
session anomalies, or provider credential rejection. Preserve CIDs and audit
references, never raw cookies, bearer tokens, reset tokens, OAuth codes,
passwords, keys, or email addresses.

## Containment

Disable or rotate only the affected credential path. Revoke affected sessions
through supported admin/session operations. Keep authorization fail closed.
For signing-key incidents, publish a new active key ID, retain only explicitly
safe previous verification keys for the bounded overlap, then retire the
compromised key.

```sh
scripts/deploy/status.sh
docker compose --env-file /etc/judgify/env.production --file deploy/compose.production.yml \
  exec -T api wget --quiet --output-document=- http://127.0.0.1:9090/live
```

Provider and database passwords require explicit server-side rotation; changing
a mounted file alone does not change an existing PostgreSQL role.

## Recovery

Verify register/login/refresh/logout, password reset, recovery-contact, OAuth
login/link/unlink, recent reauthentication, and session revocation. Confirm old
credentials fail, security versions/revisions advance as designed, email
delivery converges, and audit records are redacted. Record actor, key IDs,
affected interval, session counts, release, CIDs, and external incident ID.
