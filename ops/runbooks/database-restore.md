# Database backup or restore incident

## Symptoms and evidence

Use backup freshness/failure alerts and the product dashboard. Preserve the
last successful backup ID, publisher receipt, database schema head, release
identity, and incident start time. Never print libpq credentials or age
identities into a ticket.

The host collector must read backup and restore metrics from distinct files
configured by `JUDGIFY_BACKUP_METRICS_FILE` and
`JUDGIFY_RESTORE_METRICS_FILE`; a failure counter increment is incident
evidence even when no archive was produced.

## Containment

For a backup failure, keep serving only while a previously verified off-host
recovery point remains inside the accepted risk window; otherwise freeze
mutations and escalate. For restore, stop API, worker, migrator, cron, and
operator writers. Keep ingress closed.

```sh
export JUDGIFY_BACKUP_SIGNING_PUBLIC_KEY_FILE=/secure/backup-signing.pub
export JUDGIFY_BACKUP_IDENTITY_FILE=/secure/backup-age-identity
scripts/backup/verify.sh /recovery/judgify-postgres-YYYYMMDDTHHMMSSZ.tar
```

Keep the adjacent `.tar.sha256` beside the bundle; verification refuses a
detached bundle. Expected output names the verified backup. To restore, point
libpq at an already-created blank database, fence every runtime writer at the
database/host boundary, and explicitly acknowledge the disaster action:

```sh
export JUDGIFY_RESTORE_DISASTER_ACK=RESTORE_TO_BLANK_JUDGIFY
export JUDGIFY_RESTORE_EXPECTED_SOURCE_DATABASE=judgify
export JUDGIFY_RESTORE_EXPECTED_RELEASE_ID=2026.08.02.1
export JUDGIFY_RESTORE_EXPECTED_MIGRATION_HEAD=20260802010101
export PGDATABASE=judgify_recovery
scripts/restore/postgres.sh /recovery/judgify-postgres-YYYYMMDDTHHMMSSZ.tar
```

The script refuses a signed archive whose database/release/migration identity
differs, revokes CONNECT from PUBLIC and runtime roles, terminates remaining
client sessions, refuses a nonblank target, restores atomically in one
transaction, and then checks schema/domain invariants. Runtime roles stay
fenced after either success or failure.

## Recovery boundary

Do not admit traffic until credentials exposed by the incident are rotated and
the release declares the restored schema compatible. Then run:

```sh
export JUDGIFY_RESTORE_ADMIT_ACK=ADMIT_VALIDATED_JUDGIFY_RUNTIME
export JUDGIFY_RESTORE_ADMIT_RELEASE_ID=2026.08.02.1
export JUDGIFY_RESTORE_ADMIT_MIGRATION_HEAD=20260802010101
scripts/restore/unfence.sh
```

This only grants runtime DB CONNECT atomically. Keep ingress closed until API
and worker private readiness prove authorization desired/applied convergence,
expired judge/outbox leases are reconciled through supported operations, and
authenticated submission/WebSocket smoke passes.
Escalate on a bad signature, checksum mismatch, missing required table,
unvalidated constraint, or failed domain invariant; never edit the archive.
Record measured RPO/RTO and retain the restore evidence off-host.
