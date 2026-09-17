#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

umask 077

metrics_file=${JUDGIFY_RESTORE_METRICS_FILE:-}
work=
cleanup() {
	status=$?
	trap - EXIT HUP INT TERM
	if [ -n "$work" ]; then
		rm -rf -- "$work" || true
	fi
	if [ "$status" -ne 0 ]; then
		(
			ops_increment_metric_counter "$metrics_file" judgify_restore_failures_total
		) >/dev/null 2>&1 || true
	fi
	exit "$status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

[ "$#" -eq 1 ] || ops_die "usage: postgres.sh /absolute/path/to/backup.tar"
bundle=$1
ops_require_trusted_file "$bundle" "backup bundle"

[ "${JUDGIFY_RESTORE_DISASTER_ACK:-}" = "RESTORE_TO_BLANK_JUDGIFY" ] ||
	ops_die "set JUDGIFY_RESTORE_DISASTER_ACK=RESTORE_TO_BLANK_JUDGIFY"
: "${PGDATABASE:?set the blank target PGDATABASE}"
ops_safe_token "$PGDATABASE" 63 || ops_die "PGDATABASE is not a safe PostgreSQL identifier"

ops_require_command psql
ops_require_command pg_restore

work=$(mktemp -d "$(ops_temp_root)/judgify-restore.XXXXXX")

dump="$work/database.dump"
verified_manifest="$work/manifest.env"
JUDGIFY_VERIFIED_DUMP_OUTPUT="$dump" \
	JUDGIFY_VERIFIED_MANIFEST_OUTPUT="$verified_manifest" \
	"$SCRIPT_DIR/../backup/verify.sh" "$bundle"

source_database=$(ops_manifest_value "$verified_manifest" database)
manifest_schema=$(ops_manifest_value "$verified_manifest" schema_version)
if [ "${GO_ENV:-local}" = "production" ]; then
	[ "$manifest_schema" = "2" ] || ops_die "production restore requires backup manifest schema 2"
	[ "$(ops_manifest_value "$verified_manifest" release_id)" != "unrecorded" ] ||
		ops_die "production restore requires a signed release identity"
	[ "$(ops_manifest_value "$verified_manifest" migration_head)" != "none" ] ||
		ops_die "production restore requires a signed migration head"
fi
if [ -n "${JUDGIFY_RESTORE_EXPECTED_SOURCE_DATABASE:-}" ]; then
	ops_safe_token "$JUDGIFY_RESTORE_EXPECTED_SOURCE_DATABASE" 63 ||
		ops_die "expected source database is not a safe PostgreSQL identifier"
	[ "$source_database" = "$JUDGIFY_RESTORE_EXPECTED_SOURCE_DATABASE" ] ||
		ops_die "backup source database does not match the expected source"
fi
if [ -n "${JUDGIFY_RESTORE_EXPECTED_RELEASE_ID:-}" ]; then
	[ "$manifest_schema" = "2" ] || ops_die "expected release identity requires manifest schema 2"
	ops_safe_path_segment "$JUDGIFY_RESTORE_EXPECTED_RELEASE_ID" 120 ||
		ops_die "expected release identity is malformed"
	[ "$(ops_manifest_value "$verified_manifest" release_id)" = "$JUDGIFY_RESTORE_EXPECTED_RELEASE_ID" ] ||
		ops_die "backup release identity does not match the expected release"
fi
if [ -n "${JUDGIFY_RESTORE_EXPECTED_MIGRATION_HEAD:-}" ]; then
	[ "$manifest_schema" = "2" ] || ops_die "expected migration head requires manifest schema 2"
	case "$JUDGIFY_RESTORE_EXPECTED_MIGRATION_HEAD" in
		??????????????)
			case "$JUDGIFY_RESTORE_EXPECTED_MIGRATION_HEAD" in *[!0-9]*) ops_die "expected migration head is malformed" ;; esac
			;;
		*) ops_die "expected migration head is malformed" ;;
	esac
	[ "$(ops_manifest_value "$verified_manifest" migration_head)" = "$JUDGIFY_RESTORE_EXPECTED_MIGRATION_HEAD" ] ||
		ops_die "backup migration head does not match the expected revision"
fi

export PGAPPNAME=judgify-restore

target_server_version=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc 'SHOW server_version_num;')
case "$target_server_version" in
	''|*[!0-9]*) ops_die "PostgreSQL returned an invalid server version" ;;
esac
ops_require_matching_postgres_client pg_restore "$target_server_version"
# A dump carries the SET vocabulary of the server that produced it, so a newer
# source cannot be replayed into an older target. Both checks run before the
# target is fenced and emptied, so a doomed restore never touches it.
source_server_version=$(ops_manifest_value "$verified_manifest" postgres_server_version)
case "$source_server_version" in
	''|*[!0-9]*) ops_die "backup manifest carries an invalid source server version" ;;
esac
[ "$((target_server_version / 10000))" -ge "$((source_server_version / 10000))" ] ||
	ops_die "backup came from PostgreSQL major version $((source_server_version / 10000)) and cannot be restored into major version $((target_server_version / 10000))"

restore_role=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc 'SELECT current_user;')
ops_safe_token "$restore_role" 63 || ops_die "restore role has an unsupported identity"
runtime_roles=${JUDGIFY_RESTORE_RUNTIME_ROLES:-"judgify_api judgify_worker"}
set -f
for runtime_role in $runtime_roles; do
	ops_safe_token "$runtime_role" 63 || ops_die "runtime database role is malformed"
	[ "$runtime_role" != "$restore_role" ] ||
		ops_die "restore must use a dedicated owner or migrator role, never a runtime role"
done

# Fence new runtime connections before terminating existing pools. The restore
# operator keeps an explicit CONNECT grant so pg_restore can open its own
# transaction after PUBLIC is revoked. Runtime grants remain revoked after a
# successful restore and are restored only by the controlled recovery rollout.
psql --no-psqlrc -X -v ON_ERROR_STOP=1 \
	-v target_database="$PGDATABASE" \
	-v restore_role="$restore_role" >/dev/null <<'RESTORE_FENCE'
GRANT CONNECT ON DATABASE :"target_database" TO :"restore_role";
REVOKE CONNECT ON DATABASE :"target_database" FROM PUBLIC;
RESTORE_FENCE
for runtime_role in $runtime_roles; do
	role_exists=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atq \
		-v runtime_role="$runtime_role" <<'RESTORE_ROLE_EXISTS'
SELECT count(*) FROM pg_roles WHERE rolname = :'runtime_role';
RESTORE_ROLE_EXISTS
	)
	if [ "$role_exists" = "1" ]; then
		psql --no-psqlrc -X -v ON_ERROR_STOP=1 -q \
			-v target_database="$PGDATABASE" \
			-v runtime_role="$runtime_role" <<'RESTORE_REVOKE_RUNTIME'
REVOKE CONNECT ON DATABASE :"target_database" FROM :"runtime_role";
RESTORE_REVOKE_RUNTIME
	fi
done
psql --no-psqlrc -X -v ON_ERROR_STOP=1 -q -c "
	SELECT pg_terminate_backend(pid)
	FROM pg_stat_activity
	WHERE datname = current_database()
	  AND pid <> pg_backend_pid()
	  AND backend_type = 'client backend';" >/dev/null

writer_count=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc "
	SELECT count(*)
	FROM pg_stat_activity
	WHERE datname = current_database()
	  AND pid <> pg_backend_pid()
	  AND backend_type = 'client backend';")
[ "$writer_count" = "0" ] ||
	ops_die "target database has active client sessions; stop writers before restore"

table_count=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc "
	SELECT count(*)
	FROM information_schema.tables
	WHERE table_schema NOT IN ('pg_catalog', 'information_schema');")
[ "$table_count" = "0" ] ||
	ops_die "target database is not blank"

started_at=$(date -u '+%s')
pg_restore \
	--exit-on-error \
	--single-transaction \
	--no-owner \
	--no-acl \
	--dbname="$PGDATABASE" \
	"$dump"

"$SCRIPT_DIR/validate.sh"
completed_at=$(date -u '+%s')
duration=$((completed_at - started_at))

failure_count=$(ops_metric_counter_value "$metrics_file" judgify_restore_failures_total)
metrics_body="judgify_restore_last_success_timestamp_seconds $completed_at
judgify_restore_last_duration_seconds $duration
judgify_restore_failures_total $failure_count"
ops_write_metric_file "$metrics_file" "$metrics_body"

printf '%s\n' "restore validated in ${duration}s; runtime database roles remain fenced until controlled recovery rollout"
