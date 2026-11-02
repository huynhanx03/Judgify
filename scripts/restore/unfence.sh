#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

umask 077
admit_sql=
cleanup() {
	cleanup_status=$?
	trap - EXIT HUP INT TERM
	[ -z "$admit_sql" ] || rm -f -- "$admit_sql"
	exit "$cleanup_status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

[ "${JUDGIFY_RESTORE_ADMIT_ACK:-}" = "ADMIT_VALIDATED_JUDGIFY_RUNTIME" ] ||
	ops_die "set JUDGIFY_RESTORE_ADMIT_ACK=ADMIT_VALIDATED_JUDGIFY_RUNTIME"
: "${PGDATABASE:?set the validated restored PGDATABASE}"
: "${JUDGIFY_RESTORE_ADMIT_RELEASE_ID:?set the release selected for recovery}"
: "${JUDGIFY_RESTORE_ADMIT_MIGRATION_HEAD:?set the exact restored migration head}"
ops_safe_token "$PGDATABASE" 63 || ops_die "PGDATABASE is not a safe PostgreSQL identifier"
ops_safe_path_segment "$JUDGIFY_RESTORE_ADMIT_RELEASE_ID" 120 ||
	ops_die "recovery release identity is malformed"
case "$JUDGIFY_RESTORE_ADMIT_MIGRATION_HEAD" in
	??????????????)
		case "$JUDGIFY_RESTORE_ADMIT_MIGRATION_HEAD" in
			*[!0-9]*) ops_die "recovery migration head is malformed" ;;
		esac
		;;
	*) ops_die "recovery migration head is malformed" ;;
esac
ops_require_command psql

export PGAPPNAME=judgify-restore-unfence
"$SCRIPT_DIR/validate.sh"
actual_head=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atqc \
	"SELECT max(version) FROM public.atlas_schema_revisions;")
[ "$actual_head" = "$JUDGIFY_RESTORE_ADMIT_MIGRATION_HEAD" ] ||
	ops_die "restored migration head differs from the admitted release"

runtime_roles=${JUDGIFY_RESTORE_RUNTIME_ROLES:-"judgify_api judgify_worker"}
set -f
for runtime_role in $runtime_roles; do
	ops_safe_token "$runtime_role" 63 || ops_die "runtime database role is malformed"
	role_exists=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atq \
		-v runtime_role="$runtime_role" \
		-c "SELECT count(*) FROM pg_roles WHERE rolname = :'runtime_role';")
	[ "$role_exists" = "1" ] || ops_die "required runtime database role does not exist"
	active_sessions=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atq \
		-v runtime_role="$runtime_role" \
		-c "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND usename = :'runtime_role';")
	[ "$active_sessions" = "0" ] || ops_die "runtime role connected before recovery admission"
	database_connect=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atq \
		-v runtime_role="$runtime_role" \
		-c "SELECT has_database_privilege(:'runtime_role', current_database(), 'CONNECT');")
	[ "$database_connect" = "f" ] || ops_die "runtime database role was not fenced"
done

admit_sql=$(mktemp "$(ops_temp_root)/judgify-restore-admit.XXXXXX")
for runtime_role in $runtime_roles; do
	printf 'GRANT CONNECT ON DATABASE "%s" TO "%s";\n' "$PGDATABASE" "$runtime_role" >>"$admit_sql"
done
psql --no-psqlrc -X -v ON_ERROR_STOP=1 -q --single-transaction --file "$admit_sql"
rm -f -- "$admit_sql"
admit_sql=

for runtime_role in $runtime_roles; do
	database_connect=$(psql --no-psqlrc -X -v ON_ERROR_STOP=1 -Atq \
		-v runtime_role="$runtime_role" \
		-c "SELECT has_database_privilege(:'runtime_role', current_database(), 'CONNECT');")
	[ "$database_connect" = "t" ] || ops_die "runtime database role admission did not persist"
done

printf '%s\n' "runtime database roles admitted for release $JUDGIFY_RESTORE_ADMIT_RELEASE_ID; keep ingress closed until private readiness and smoke pass"
