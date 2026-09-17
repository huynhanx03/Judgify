#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

umask 077

metrics_file=${JUDGIFY_BACKUP_METRICS_FILE:-}
work=
receipt_partial=
offhost_partial=
backup_lock=
cleanup() {
	status=$?
	trap - EXIT HUP INT TERM
	if [ -n "$work" ]; then
		rm -rf -- "$work" || true
	fi
	if [ -n "$receipt_partial" ]; then
		rm -f -- "$receipt_partial" || true
	fi
	if [ -n "$offhost_partial" ]; then
		rm -rf -- "$offhost_partial" || true
	fi
	if [ -n "$backup_lock" ]; then
		rmdir "$backup_lock" 2>/dev/null || true
	fi
	if [ "$status" -ne 0 ]; then
		(
			ops_increment_metric_counter "$metrics_file" judgify_backup_failures_total
		) >/dev/null 2>&1 || true
	fi
	exit "$status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

ops_require_command pg_dump
ops_require_command psql
ops_require_command age
ops_require_command openssl
ops_require_command tar

: "${PGDATABASE:?set PGDATABASE without embedding credentials in a URL}"
: "${JUDGIFY_BACKUP_RECIPIENTS_FILE:?set the external age recipients file}"
: "${JUDGIFY_BACKUP_SIGNING_PRIVATE_KEY_FILE:?set the external manifest signing key}"

ops_safe_token "$PGDATABASE" 63 || ops_die "PGDATABASE is not a safe PostgreSQL identifier"
ops_require_trusted_file "$JUDGIFY_BACKUP_RECIPIENTS_FILE" "age recipients file"
ops_require_private_file "$JUDGIFY_BACKUP_SIGNING_PRIVATE_KEY_FILE" "backup signing private key"

STAGING_ROOT=${JUDGIFY_BACKUP_STAGING_ROOT:-/var/lib/judgify/backup-staging}
ops_require_secure_directory "$STAGING_ROOT" "backup staging root"
lock_candidate="$STAGING_ROOT/.backup.lock"
if ! mkdir "$lock_candidate" 2>/dev/null; then
	ops_die "another PostgreSQL backup owns the staging lock"
fi
backup_lock=$lock_candidate

created_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
created_epoch=$(date -u '+%s')
backup_id="judgify-postgres-$(date -u '+%Y%m%dT%H%M%SZ')-$$"
ops_safe_path_segment "$backup_id" 128 || ops_die "generated backup identity is invalid"

export PGAPPNAME=judgify-backup
actual_database=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc 'SELECT current_database();')
ops_safe_token "$actual_database" 63 || ops_die "database returned an unsafe identity"
[ "$actual_database" = "$PGDATABASE" ] ||
	ops_die "connected database does not match PGDATABASE"
postgres_server_version=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc 'SHOW server_version_num;')
case "$postgres_server_version" in
	''|*[!0-9]*) ops_die "PostgreSQL returned an invalid server version" ;;
esac
ops_require_matching_postgres_client pg_dump "$postgres_server_version"
revision_table=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc \
	"SELECT COALESCE(to_regclass('public.atlas_schema_revisions')::text, '');")
if [ -n "$revision_table" ]; then
	incomplete_revisions=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc "
		SELECT count(*)
		FROM public.atlas_schema_revisions
		WHERE error <> ''
		   OR error_stmt <> ''
		   OR (type = 2 AND (applied <> total OR total <= 0 OR hash = ''));")
	[ "$incomplete_revisions" = "0" ] ||
		ops_die "database has incomplete or failed migration revisions"
	migration_head=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc \
		"SELECT COALESCE(max(version), 'none') FROM public.atlas_schema_revisions;")
else
	migration_head=none
fi
case "$migration_head" in
	none) ;;
	??????????????)
		case "$migration_head" in *[!0-9]*) ops_die "migration head is malformed" ;; esac
		;;
	*) ops_die "migration head is malformed" ;;
esac
release_id=${JUDGIFY_BACKUP_RELEASE_ID:-unrecorded}
ops_safe_path_segment "$release_id" 120 || ops_die "backup release identity is malformed"
if [ "${GO_ENV:-local}" = "production" ]; then
	[ "$migration_head" != "none" ] || ops_die "production backup requires an applied migration head"
	[ "$release_id" != "unrecorded" ] ||
		ops_die "production backup requires JUDGIFY_BACKUP_RELEASE_ID"
fi

work=$(mktemp -d "$STAGING_ROOT/.${backup_id}.XXXXXX")

plain="$work/database.dump"
ciphertext="$work/database.dump.age"
manifest="$work/manifest.env"
signature="$work/manifest.sig"
bundle="$work/${backup_id}.tar"
bundle_checksum="$work/${backup_id}.tar.sha256"

# Credentials come only from libpq's standard environment/service/passfile
# mechanisms. They are neither echoed nor placed in process arguments.
pg_dump \
	--format=custom \
	--compress=9 \
	--serializable-deferrable \
	--no-owner \
	--no-acl \
	--file="$plain"

plain_sha=$(ops_sha256 "$plain")
age --encrypt \
	--recipients-file "$JUDGIFY_BACKUP_RECIPIENTS_FILE" \
	--output "$ciphertext" \
	"$plain"
cipher_sha=$(ops_sha256 "$ciphertext")
plain_bytes=$(ops_file_bytes "$plain")
cipher_bytes=$(ops_file_bytes "$ciphertext")

cat >"$manifest" <<EOF
schema_version=2
backup_id=$backup_id
created_at=$created_at
created_epoch=$created_epoch
database=$actual_database
release_id=$release_id
migration_head=$migration_head
postgres_server_version=$postgres_server_version
format=postgres-custom
encryption=age-v1
plaintext_sha256=$plain_sha
ciphertext_sha256=$cipher_sha
plaintext_bytes=$plain_bytes
ciphertext_bytes=$cipher_bytes
EOF

openssl dgst -sha256 \
	-sign "$JUDGIFY_BACKUP_SIGNING_PRIVATE_KEY_FILE" \
	-out "$signature" \
	"$manifest"

(
	cd "$work"
	tar -cf "$bundle" database.dump.age manifest.env manifest.sig
)
bundle_sha=$(ops_sha256 "$bundle")
printf '%s  %s\n' "$bundle_sha" "$(basename "$bundle")" >"$bundle_checksum"

publish_builtin() {
	: "${JUDGIFY_BACKUP_OFFHOST_DIR:?set an off-host mount or a publisher executable}"
	ops_require_secure_directory "$JUDGIFY_BACKUP_OFFHOST_DIR" "off-host backup directory"
	marker="$JUDGIFY_BACKUP_OFFHOST_DIR/.judgify-offhost"
	ops_require_trusted_file "$marker" "off-host destination marker"
	[ "$(cat "$marker")" = "judgify-offhost-backup-v1" ] ||
		ops_die "off-host destination marker is invalid"
	if [ "${GO_ENV:-local}" = "production" ]; then
		[ "${JUDGIFY_BACKUP_OFFHOST_ACK:-}" = "VERIFIED_SEPARATE_FAILURE_DOMAIN" ] ||
			ops_die "production directory publishing requires explicit off-host acknowledgement"
		[ "$(ops_file_device "$STAGING_ROOT")" != "$(ops_file_device "$JUDGIFY_BACKUP_OFFHOST_DIR")" ] ||
			ops_die "production off-host directory must be mounted from a different filesystem"
	fi

	final="$JUDGIFY_BACKUP_OFFHOST_DIR/${backup_id}.tar"
	final_checksum="${final}.sha256"
	[ ! -e "$final" ] && [ ! -L "$final" ] ||
		ops_die "backup destination already exists"
	[ ! -e "$final_checksum" ] && [ ! -L "$final_checksum" ] ||
		ops_die "backup checksum destination already exists"
	offhost_partial=$(mktemp -d "$JUDGIFY_BACKUP_OFFHOST_DIR/.${backup_id}.partial.XXXXXX")
	cp "$bundle" "$offhost_partial/${backup_id}.tar"
	cp "$bundle_checksum" "$offhost_partial/${backup_id}.tar.sha256"
	[ "$(ops_sha256 "$offhost_partial/${backup_id}.tar")" = "$bundle_sha" ] ||
		ops_die "off-host copy checksum mismatch"
	mv "$offhost_partial/${backup_id}.tar.sha256" "$final_checksum"
	# The bundle is the publication sentinel: once it appears, its adjacent
	# checksum is already durable and a consumer can verify the pair.
	mv "$offhost_partial/${backup_id}.tar" "$final"
	rmdir "$offhost_partial"
	offhost_partial=
}

if [ -n "${JUDGIFY_BACKUP_PUBLISHER:-}" ]; then
	ops_require_trusted_executable "$JUDGIFY_BACKUP_PUBLISHER" "backup publisher"
	: "${JUDGIFY_BACKUP_RECEIPT_DIR:?set the durable backup receipt directory}"
	ops_require_secure_directory "$JUDGIFY_BACKUP_RECEIPT_DIR" "backup receipt directory"
	receipt="$work/publish.receipt"
	final_receipt="$JUDGIFY_BACKUP_RECEIPT_DIR/${backup_id}.receipt"
	[ ! -e "$final_receipt" ] && [ ! -L "$final_receipt" ] ||
		ops_die "backup publisher receipt already exists"
	"$JUDGIFY_BACKUP_PUBLISHER" publish "$bundle" "$bundle_checksum" >"$receipt"
	[ -s "$receipt" ] || ops_die "backup publisher did not return a durable receipt"
	[ "$(ops_file_bytes "$receipt")" -le 4096 ] || ops_die "backup publisher receipt is too large"
	[ "$(ops_manifest_value "$receipt" schema_version)" = "1" ] ||
		ops_die "backup publisher returned an unsupported receipt"
	[ "$(ops_manifest_value "$receipt" backup_id)" = "$backup_id" ] ||
		ops_die "backup publisher receipt names a different backup"
	[ "$(ops_manifest_value "$receipt" bundle_sha256)" = "$bundle_sha" ] ||
		ops_die "backup publisher receipt names a different checksum"
	[ "$(ops_manifest_value "$receipt" durable)" = "true" ] ||
		ops_die "backup publisher did not attest durable publication"
	receipt_partial=$(mktemp "$JUDGIFY_BACKUP_RECEIPT_DIR/.${backup_id}.receipt.XXXXXX")
	cp "$receipt" "$receipt_partial"
	chmod 600 "$receipt_partial"
	mv "$receipt_partial" "$final_receipt"
	receipt_partial=
else
	publish_builtin
fi

failure_count=$(ops_metric_counter_value "$metrics_file" judgify_backup_failures_total)
metrics_body="judgify_backup_last_success_timestamp_seconds $created_epoch
judgify_backup_last_success_bytes $cipher_bytes
judgify_backup_failures_total $failure_count"
ops_write_metric_file "$metrics_file" "$metrics_body"

# Retention applies only to the attested destination and only to this exact
# filename namespace. A remote publisher owns its own retention policy.
if [ -z "${JUDGIFY_BACKUP_PUBLISHER:-}" ]; then
	retention_days=${JUDGIFY_BACKUP_RETENTION_DAYS:-35}
	case "$retention_days" in
		''|*[!0-9]*) ops_die "backup retention days must be a positive integer" ;;
	esac
	[ "$retention_days" -ge 7 ] || ops_die "backup retention shorter than seven days is forbidden"
	expired_list="$work/expired-backups.list"
	find "$JUDGIFY_BACKUP_OFFHOST_DIR" -maxdepth 1 -type f \
		-name 'judgify-postgres-*.tar' -mtime "+$retention_days" -print >"$expired_list"
	while IFS= read -r expired; do
		case "$(basename "$expired")" in
			judgify-postgres-*.tar)
				rm -f -- "$expired" "${expired}.sha256"
				;;
		esac
	done <"$expired_list"
fi

printf '%s\n' "backup published: $backup_id"
