#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"

[ "${JUDGIFY_BACKUP_RESTORE_TEST_ACK:-}" = "DESTROY_ONLY_JUDGIFY_RESTORE_TEST_DATABASES" ] ||
	ops_die "backup/restore test requires its destructive test-only acknowledgement"
: "${JUDGIFY_BACKUP_RESTORE_DRIVER:?set the absolute fixture database driver}"
: "${JUDGIFY_TEST_SOURCE_PGSERVICE:?set the isolated source libpq service}"
: "${JUDGIFY_TEST_TARGET_PGSERVICE:?set the isolated target libpq service}"
: "${JUDGIFY_TEST_SOURCE_DATABASE:?set the isolated source database name}"
: "${JUDGIFY_TEST_TARGET_DATABASE:?set the isolated target database name}"

for database in "$JUDGIFY_TEST_SOURCE_DATABASE" "$JUDGIFY_TEST_TARGET_DATABASE"; do
	case "$database" in
		*_judgify_restore_test) ;;
		*) ops_die "test database names must end in _judgify_restore_test" ;;
	esac
done
[ "$JUDGIFY_TEST_SOURCE_DATABASE" != "$JUDGIFY_TEST_TARGET_DATABASE" ] ||
	ops_die "source and target test databases must differ"
ops_require_absolute_path "$JUDGIFY_BACKUP_RESTORE_DRIVER" "backup/restore fixture driver"
[ -x "$JUDGIFY_BACKUP_RESTORE_DRIVER" ] ||
	ops_die "backup/restore fixture driver is not executable"

ops_require_command age
ops_require_command age-keygen
ops_require_command openssl

work=$(mktemp -d "$(ops_temp_root)/judgify-backup-restore-test.XXXXXX")
cleanup() {
	cleanup_status=$?
	# The backup and restore steps write to logs inside the work directory, which
	# this trap then deletes. Surfacing them on failure keeps the proof harness
	# from discarding the only evidence of its own failure.
	if [ "$cleanup_status" -ne 0 ]; then
		for failed_log in "$work/backup.log" "$work/restore.log"; do
			[ -s "$failed_log" ] || continue
			printf '%s\n' "--- $(basename "$failed_log") ---" >&2
			tail -40 "$failed_log" >&2
		done
	fi
	"$JUDGIFY_BACKUP_RESTORE_DRIVER" cleanup \
		"$JUDGIFY_TEST_SOURCE_PGSERVICE" "$JUDGIFY_TEST_SOURCE_DATABASE" \
		"$JUDGIFY_TEST_TARGET_PGSERVICE" "$JUDGIFY_TEST_TARGET_DATABASE" \
		>/dev/null 2>&1 || true
	rm -rf -- "$work"
}
trap cleanup EXIT HUP INT TERM

mkdir "$work/offhost" "$work/staging"
printf '%s\n' "judgify-offhost-backup-v1" >"$work/offhost/.judgify-offhost"

age-keygen -o "$work/age.identity" >"$work/age-keygen.log" 2>&1
age-keygen -y "$work/age.identity" >"$work/age.recipients"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 \
	-out "$work/signing.key" >"$work/openssl.log" 2>&1
openssl pkey -in "$work/signing.key" -pubout -out "$work/signing.pub" \
	>>"$work/openssl.log" 2>&1
chmod 600 "$work/age.identity" "$work/signing.key"

"$JUDGIFY_BACKUP_RESTORE_DRIVER" prepare \
	"$JUDGIFY_TEST_SOURCE_PGSERVICE" "$JUDGIFY_TEST_SOURCE_DATABASE" \
	"$JUDGIFY_TEST_TARGET_PGSERVICE" "$JUDGIFY_TEST_TARGET_DATABASE"

started_at=$(date -u '+%s')
backup_log="$work/backup.log"
PGSERVICE="$JUDGIFY_TEST_SOURCE_PGSERVICE" \
PGDATABASE="$JUDGIFY_TEST_SOURCE_DATABASE" \
JUDGIFY_BACKUP_RECIPIENTS_FILE="$work/age.recipients" \
JUDGIFY_BACKUP_SIGNING_PRIVATE_KEY_FILE="$work/signing.key" \
JUDGIFY_BACKUP_STAGING_ROOT="$work/staging" \
JUDGIFY_BACKUP_OFFHOST_DIR="$work/offhost" \
"$ROOT/scripts/backup/postgres.sh" >"$backup_log" 2>&1

bundle=$(find "$work/offhost" -maxdepth 1 -type f -name 'judgify-postgres-*.tar' -print)
[ -n "$bundle" ] && [ "$(printf '%s\n' "$bundle" | wc -l | tr -d ' ')" = "1" ] ||
	ops_die "backup test expected exactly one off-host bundle"

restore_log="$work/restore.log"
PGSERVICE="$JUDGIFY_TEST_TARGET_PGSERVICE" \
PGDATABASE="$JUDGIFY_TEST_TARGET_DATABASE" \
JUDGIFY_BACKUP_SIGNING_PUBLIC_KEY_FILE="$work/signing.pub" \
JUDGIFY_BACKUP_IDENTITY_FILE="$work/age.identity" \
JUDGIFY_RESTORE_DISASTER_ACK=RESTORE_TO_BLANK_JUDGIFY \
"$ROOT/scripts/restore/postgres.sh" "$bundle" >"$restore_log" 2>&1

"$JUDGIFY_BACKUP_RESTORE_DRIVER" validate \
	"$JUDGIFY_TEST_SOURCE_PGSERVICE" "$JUDGIFY_TEST_SOURCE_DATABASE" \
	"$JUDGIFY_TEST_TARGET_PGSERVICE" "$JUDGIFY_TEST_TARGET_DATABASE"

secret=$(awk '/^AGE-SECRET-KEY-/ {print; exit}' "$work/age.identity")
if [ -n "$secret" ] && grep -F "$secret" "$backup_log" "$restore_log" >/dev/null; then
	ops_die "backup or restore output exposed the decryption identity"
fi

completed_at=$(date -u '+%s')
duration=$((completed_at - started_at))
[ "$duration" -le 3600 ] || ops_die "observed restore proof exceeded the 60-minute RTO"
printf '%s\n' "backup/blank-restore/domain-invariant proof passed in ${duration}s"
