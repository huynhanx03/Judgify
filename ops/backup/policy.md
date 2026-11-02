# PostgreSQL backup and recovery policy

## Objectives and ownership

Judgify's initial single-host target is an observed recovery point of at most
five minutes and an observed blank-environment recovery time of at most sixty
minutes. These are recovery objectives, not high-availability claims. The
database operator owns the schedule, the off-host publisher, encryption and
signing keys, retention, and restore drills. Product operators may inspect
freshness metrics but cannot read backup keys.

PostgreSQL is the system of record. Local caches, WebSocket state, and Forge
files are disposable accelerators. Test assets currently live in PostgreSQL
and are covered by the database archive. Introducing filesystem/object-store
source blobs requires an atomic manifest and a matching off-host backup adapter
before that storage mode can ship.

## Backup contract

`scripts/backup/postgres.sh` creates a consistent PostgreSQL custom archive,
encrypts it with one or more external `age` recipients, signs a bounded
manifest with an external private key, checksums the final bundle, and only
reports success after an off-host publisher returns a durable receipt or an
attested off-host mount verifies the copied checksum. The `.tar` and adjacent
`.tar.sha256` are one recovery artifact; verification deliberately refuses
either file by itself. Signed manifest schema 2 binds the real database name,
recovery-point timestamp, immutable application release, migration head, and
PostgreSQL server version. Production refuses an unrecorded release or missing
migration head. One staging lock prevents overlapping five-minute jobs.

Credentials use libpq configuration (`PGSERVICE`, `PGPASSFILE`, or separate
`PG*` variables). Never put a password in a command-line URL. Required key
files are outside the repository:

- `JUDGIFY_BACKUP_RECIPIENTS_FILE`: public age recipients.
- `JUDGIFY_BACKUP_SIGNING_PRIVATE_KEY_FILE`: manifest signing key.
- `JUDGIFY_BACKUP_IDENTITY_FILE`: restore-only age identity.
- `JUDGIFY_BACKUP_SIGNING_PUBLIC_KEY_FILE`: verification key.

Point `JUDGIFY_BACKUP_METRICS_FILE` and `JUDGIFY_RESTORE_METRICS_FILE` at
different absolute paths in the node-exporter textfile directory. The scripts
update them atomically, preserve their monotonic failure counters across
invocations, and never mix restore evidence into backup freshness.

Production should set an absolute `JUDGIFY_BACKUP_PUBLISHER` and a
non-symlink `JUDGIFY_BACKUP_RECEIPT_DIR` outside the staging directory. Its
interface is:

```text
publisher publish /absolute/backup.tar /absolute/backup.tar.sha256
```

It must atomically publish both objects to another failure domain and write a
bounded receipt to stdout with exactly one value for `schema_version=1`,
`backup_id`, `bundle_sha256`, and `durable=true`. The script verifies and
retains that receipt as `BACKUP_ID.receipt` before reporting success. The
built-in directory publisher is
acceptable only for a genuinely off-host, non-symlink directory containing an
operator-controlled regular `.judgify-offhost` file whose entire contents are
`judgify-offhost-backup-v1`. A directory on the database host's root disk does
not satisfy policy even if it has the marker. In production it additionally
requires `JUDGIFY_BACKUP_OFFHOST_ACK=VERIFIED_SEPARATE_FAILURE_DOMAIN` and the
destination must be a different mounted filesystem from staging.

## Schedule and retention

- Run the encrypted logical backup every five minutes while its measured p99
  duration remains below two minutes and its I/O does not consume more than
  20% of the database disk budget.
- Retain five-minute recovery points for 48 hours, daily recovery points for
  35 days, and one month-end recovery point for 12 months. A remote publisher
  implements tiered retention; the built-in publisher only supplies the
  conservative 35-day floor.
- Alert when `time() - judgify_backup_last_success_timestamp_seconds > 300`,
  on any backup failure increment, and when staging/off-host free space crosses
  the configured safety watermark.
- If a full logical backup can no longer finish inside the two-minute budget,
  stop claiming the five-minute RPO and enable encrypted continuous WAL
  archival plus scheduled physical base backups before the next release.
  WAL archival is deliberately not pretended by the logical script.

The freshness timestamp is the dump recovery point at backup start, not upload
completion. A slow dump or publisher therefore consumes the RPO instead of
hiding it.

## Key separation and rotation

The backup service receives public encryption recipients and the signing
private key. The production host never receives an age decryption identity.
The isolated recovery operator receives the identity and signing public key.
Rotate by adding the new recipient, producing and restoring one dual-recipient
backup, then removing the old recipient. Signing-key rotation retains both
public keys for every still-retained backup.

## Restore proof

At least monthly, and before every migration with destructive potential:

1. Fetch one bundle into an isolated recovery host.
2. Fetch its matching `.sha256` and run `scripts/backup/verify.sh`.
3. Create a blank database with runtime writers absent.
4. Set `JUDGIFY_RESTORE_EXPECTED_SOURCE_DATABASE`, and preferably the expected
   release and migration head, then run `scripts/restore/postgres.sh`. It
   revokes database CONNECT from PUBLIC and runtime roles before terminating
   existing sessions. Archive application uses one transaction, so a restore
   error cannot leave a partially applied archive. Domain validation runs after
   that commit; runtime roles remain fenced on both success and failure.
5. Rotate any credential exposed by the incident scenario.
6. Select a release that declares the restored schema compatible. Run
   `scripts/restore/unfence.sh` with the exact signed migration head and release
   identity. This grants CONNECT to API/worker atomically but does not open
   ingress; private readiness and authenticated smoke remain mandatory.
7. Reconcile expired leases and disposable projections through supported
   application operations.
8. Retain timestamps, archive identity, release identity, row-count/invariant
   evidence, and operator identity in the incident record.

A file that has never completed this drill is not considered a usable backup.
