#!/bin/sh
# Fixture driver for test/operations/backup_restore_test.sh.
# Subcommands: prepare | validate | cleanup
# Usage: backup_restore_driver.sh prepare  <src_svc> <src_db> <tgt_svc> <tgt_db>
#         backup_restore_driver.sh validate <src_svc> <src_db> <tgt_svc> <tgt_db>
#         backup_restore_driver.sh cleanup  <src_svc> <src_db> <tgt_svc> <tgt_db>
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"

# Homebrew ships pg_dump, pg_restore, psql, and pg_isready at a path not on the
# default macOS PATH. Prepend it so every ops_require_command call and every
# direct invocation resolves the correct binary. The keg is pinned to the same
# major version as the postgres:16 fixture this driver boots, because a client
# from a newer major emits SET commands the older server cannot parse and the
# backup scripts now refuse that combination outright.
LIBPQ_BIN="/opt/homebrew/opt/postgresql@16/bin"
export PATH="$LIBPQ_BIN:$PATH"


# ---------------------------------------------------------------------------
# State file helpers
# ---------------------------------------------------------------------------
STATE_FILE="$(ops_temp_root)/judgify-brt-driver-state.env"

# pg_service.conf: libpq resolves PGSERVICEFILE from the environment. The test
# harness does not export PGSERVICEFILE into the backup/restore script
# invocations, so both services must be registered in the default per-user
# service file (~/.pg_service.conf). This is the only file libpq will consult
# when PGSERVICEFILE is absent from the environment. The driver writes only its
# own named sections and cleanup removes them. This is disclosed in the report.
PG_SERVICE_CONF="$HOME/.pg_service.conf"

_state_set() {
    key=$1; val=$2
    # Write or update a single key=value line in the state file.
    if [ -f "$STATE_FILE" ]; then
        grep -v "^${key}=" "$STATE_FILE" >"${STATE_FILE}.tmp" 2>/dev/null || true
        mv "${STATE_FILE}.tmp" "$STATE_FILE"
    fi
    printf '%s=%s\n' "$key" "$val" >>"$STATE_FILE"
}

_state_get() {
    key=$1
    if [ ! -f "$STATE_FILE" ]; then printf ''; return 0; fi
    awk -F= -v k="$key" '$1 == k { print substr($0, length(k)+2) }' "$STATE_FILE"
}

# ---------------------------------------------------------------------------
# prepare <src_svc> <src_db> <tgt_svc> <tgt_db>
# ---------------------------------------------------------------------------
cmd_prepare() {
    src_svc=$1; src_db=$2; tgt_svc=$3; tgt_db=$4

    ops_require_command docker
    ops_require_command "$LIBPQ_BIN/psql"
    ops_require_command "$LIBPQ_BIN/pg_isready"

    # Generate passwords that are never printed.
    admin_pw="brtadmin$(openssl rand -hex 12)"
    migrator_pw="brtmigr$(openssl rand -hex 12)"
    api_pw="brtapi$(openssl rand -hex 12)"
    worker_pw="brtworker$(openssl rand -hex 12)"

    # Unique container name: script PID + random hex to avoid any collision.
    rand_suffix=$(openssl rand -hex 4)
    container="judgify-brt-$$-${rand_suffix}"

    _state_set container "$container"
    _state_set admin_pw "$admin_pw"
    _state_set migrator_pw "$migrator_pw"
    _state_set api_pw "$api_pw"
    _state_set worker_pw "$worker_pw"
    _state_set src_svc "$src_svc"
    _state_set src_db "$src_db"
    _state_set tgt_svc "$tgt_svc"
    _state_set tgt_db "$tgt_db"

    docker run \
        --pull never \
        --rm \
        --detach \
        --name "$container" \
        --env "POSTGRES_USER=postgres" \
        --env "POSTGRES_PASSWORD=$admin_pw" \
        --env "POSTGRES_DB=$src_db" \
        --publish "127.0.0.1::5432" \
        postgres:16 >/dev/null

    # Wait for the init marker AND pg_isready to avoid the bootstrap-postmaster race.
    ready=false
    i=0
    while [ "$i" -lt 60 ]; do
        if docker logs "$container" 2>&1 | \
                grep -Fq 'PostgreSQL init process complete; ready for start up.' && \
            "$LIBPQ_BIN/pg_isready" \
                --host 127.0.0.1 \
                --port "$(docker port "$container" 5432/tcp | awk -F: 'NR==1{print $NF}')" \
                --username postgres >/dev/null 2>&1; then
            ready=true
            break
        fi
        sleep 1
        i=$((i + 1))
    done
    [ "$ready" = "true" ] || ops_die "throwaway PostgreSQL did not become ready in 60 s"

    host_port=$(docker port "$container" 5432/tcp | awk -F: 'NR==1{print $NF}')
    _state_set host_port "$host_port"

    # Bootstrap roles from the production init-roles.sql.
    docker exec \
        --interactive \
        --env "POSTGRES_DB=$src_db" \
        --env "JUDGIFY_MIGRATOR_PASSWORD=$migrator_pw" \
        --env "JUDGIFY_API_PASSWORD=$api_pw" \
        --env "JUDGIFY_WORKER_PASSWORD=$worker_pw" \
        "$container" \
        psql \
            --no-psqlrc \
            --set ON_ERROR_STOP=1 \
            --username postgres \
            --dbname "$src_db" \
            <"$ROOT/deploy/postgres/init-roles.sql" >/dev/null

    # Create the target database empty (restore proves blank-target DR).
    docker exec "$container" \
        createdb \
            --username postgres \
            --owner judgify_migrator \
            "$tgt_db" >/dev/null

    # Build and run the production migrate binary against the source database.
    migrator_binary="$(ops_temp_root)/judgify-brt-migrate-$$"
    _state_set migrator_binary "$migrator_binary"
    migrator_log="$(ops_temp_root)/judgify-brt-migrate-$$.log"

    (
        cd "$ROOT/api"
        env GOPROXY=off go build -trimpath \
            -o "$migrator_binary" \
            ./cmd/migrate
    )

    (
        cd "$ROOT/api"
        env \
            GO_ENV=local \
            DATABASE_DRIVER=postgres \
            DATABASE_HOST=127.0.0.1 \
            DATABASE_PORT="$host_port" \
            DATABASE_USERNAME=judgify_migrator \
            DATABASE_PASSWORD="$migrator_pw" \
            DATABASE_DATABASE="$src_db" \
            LOGGER_FILE_LOG_NAME="$migrator_log" \
            "$migrator_binary"
    )

    # Run bootstrap-admin so the source carries real authorization state,
    # system attribute definitions, profile schema state, roles, casbin rules
    # and an audit record.
    admin_binary="$(ops_temp_root)/judgify-brt-bootstrap-admin-$$"
    _state_set admin_binary "$admin_binary"

    (
        cd "$ROOT/api"
        env GOPROXY=off go build -trimpath \
            -o "$admin_binary" \
            ./cmd/bootstrap-admin
    )

    admin_log="$(ops_temp_root)/judgify-brt-bootstrap-admin-$$.log"
    admin_stdout="$(ops_temp_root)/judgify-brt-bootstrap-admin-stdout-$$.log"

    # Redirect stdout to a regular file so that zap's logger.Sync() can fsync
    # it. On macOS, fsync on /dev/stdout (a character device) returns ENODEV
    # which the go-common isStdoutSyncErr helper does not filter, causing the
    # binary to exit non-zero even though the bootstrap succeeded. Writing to
    # a regular file avoids the device fsync error entirely.
    (
        cd "$ROOT/api"
        env \
            GO_ENV=local \
            DATABASE_DRIVER=postgres \
            DATABASE_HOST=127.0.0.1 \
            DATABASE_PORT="$host_port" \
            DATABASE_USERNAME=judgify_migrator \
            DATABASE_PASSWORD="$migrator_pw" \
            DATABASE_DATABASE="$src_db" \
            JUDGIFY_BOOTSTRAP_ADMIN_USERNAME=brtadmin \
            JUDGIFY_BOOTSTRAP_ADMIN_PASSWORD="$admin_pw" \
            LOGGER_FILE_LOG_NAME="$admin_log" \
            "$admin_binary" >"$admin_stdout"
    )

    # Write a pgpassfile so psql/pg_dump can authenticate without a prompt.
    # Format: hostname:port:database:username:password
    pgpassfile="$(ops_temp_root)/judgify-brt-pgpass-$$"
    _state_set pgpassfile "$pgpassfile"
    printf '*:%s:*:judgify_migrator:%s\n' "$host_port" "$migrator_pw" >"$pgpassfile"
    printf '*:%s:*:postgres:%s\n' "$host_port" "$admin_pw" >>"$pgpassfile"
    chmod 600 "$pgpassfile"

    # Register both libpq services in ~/.pg_service.conf.
    # PGSERVICEFILE is not in the test harness's environment for the backup and
    # restore script invocations, so the only file libpq will consult is the
    # default per-user service file. The driver writes only its own named
    # sections and cleanup removes them. See report for disclosure.
    {
        printf '\n[%s]\n' "$src_svc"
        printf 'host=127.0.0.1\n'
        printf 'port=%s\n' "$host_port"
        printf 'user=judgify_migrator\n'
        printf 'sslmode=disable\n'
        printf 'passfile=%s\n' "$pgpassfile"

        printf '\n[%s]\n' "$tgt_svc"
        printf 'host=127.0.0.1\n'
        printf 'port=%s\n' "$host_port"
        printf 'user=judgify_migrator\n'
        printf 'sslmode=disable\n'
        printf 'passfile=%s\n' "$pgpassfile"
    } >>"$PG_SERVICE_CONF"

    printf 'backup_restore_driver: prepare complete (container=%s port=%s)\n' \
        "$container" "$host_port"
}

# ---------------------------------------------------------------------------
# validate <src_svc> <src_db> <tgt_svc> <tgt_db>
# Prove the restored target is a faithful copy of the source.
# ---------------------------------------------------------------------------
cmd_validate() {
    src_db=$2; tgt_db=$4

    host_port=$(_state_get host_port)
    pgpassfile=$(_state_get pgpassfile)

    [ -n "$host_port" ] || ops_die "validate: no host_port in state"

    psql_admin() {
        db=$1; shift
        PGPASSFILE="$pgpassfile" \
        "$LIBPQ_BIN/psql" \
            --no-psqlrc \
            -X \
            -v ON_ERROR_STOP=1 \
            --host 127.0.0.1 \
            --port "$host_port" \
            --username postgres \
            --dbname "$db" \
            --tuples-only \
            --no-align \
            "$@"
    }

    psql_migrator() {
        db=$1; shift
        PGPASSFILE="$pgpassfile" \
        "$LIBPQ_BIN/psql" \
            --no-psqlrc \
            -X \
            -v ON_ERROR_STOP=1 \
            --host 127.0.0.1 \
            --port "$host_port" \
            --username judgify_migrator \
            --dbname "$db" \
            --tuples-only \
            --no-align \
            "$@"
    }

    # 1. Identical set of tables in public schema.
    tables_sql="SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename"
    src_tables=$(psql_admin "$src_db" -c "$tables_sql")
    tgt_tables=$(psql_admin "$tgt_db" -c "$tables_sql")
    if [ "$src_tables" != "$tgt_tables" ]; then
        ops_die "validate: table set mismatch; source has tables not present on target or vice versa"
    fi

    # 2. Identical row counts per table and order-independent content checksum.
    # Build per-table count+checksum query dynamically, writing it to a temp file
    # to avoid POSIX sh subshell variable-capture issues with piped while loops.
    count_query_file="$(ops_temp_root)/judgify-brt-validate-query-$$.sql"
    {
        printf 'SELECT tablename, cnt, chk FROM (\n'
        first_inner=1
        printf '%s\n' "$src_tables" | while IFS= read -r tbl; do
            [ -n "$tbl" ] || continue
            if [ "$first_inner" = "1" ]; then
                first_inner=0
            else
                printf ' UNION ALL '
            fi
            printf "SELECT '%s' AS tablename, count(*)::text AS cnt, coalesce(md5(string_agg(t::text, '' ORDER BY t::text)),'(empty)') AS chk FROM public.\"%s\" AS t\n" \
                "$tbl" "$tbl"
        done
        printf ') sub ORDER BY tablename;\n'
    } >"$count_query_file"

    src_counts=$(psql_migrator "$src_db" -f "$count_query_file")
    tgt_counts=$(psql_migrator "$tgt_db" -f "$count_query_file")
    rm -f "$count_query_file"

    if [ "$src_counts" != "$tgt_counts" ]; then
        ops_die "validate: row count or content checksum mismatch between source and target"
    fi

    # 3. Append-only triggers, trigger function, and check constraints exist on target.
    trigger_fn_sql="
SELECT count(*)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = 'public'
  AND p.proname = 'judgify_reject_append_only_mutation'
  AND p.prokind = 'f';"
    src_fn=$(psql_admin "$src_db" -c "$trigger_fn_sql")
    tgt_fn=$(psql_admin "$tgt_db" -c "$trigger_fn_sql")
    if [ "$src_fn" != "$tgt_fn" ] || [ "$tgt_fn" = "0" ]; then
        ops_die "validate: append-only trigger function missing on target (source=$src_fn target=$tgt_fn)"
    fi

    triggers_sql="
SELECT t.tgname || '|' || r.relname || '|' || t.tgtype::text || '|' || p.proname
FROM pg_trigger t
JOIN pg_class r ON r.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = r.relnamespace
JOIN pg_proc p ON p.oid = t.tgfoid
WHERE n.nspname = 'public'
  AND p.proname = 'judgify_reject_append_only_mutation'
  AND NOT t.tgisinternal
ORDER BY t.tgname;"
    src_triggers=$(psql_admin "$src_db" -c "$triggers_sql")
    tgt_triggers=$(psql_admin "$tgt_db" -c "$triggers_sql")
    if [ "$src_triggers" != "$tgt_triggers" ]; then
        ops_die "validate: append-only triggers differ between source and target"
    fi

    # Check that at least some triggers exist.
    [ -n "$src_triggers" ] ||
        ops_die "validate: no append-only triggers found on source (schema not migrated?)"

    check_constraints_sql="
SELECT conname || '|' || conrelid::regclass::text
FROM pg_constraint
WHERE connamespace = 'public'::regnamespace
  AND contype = 'c'
ORDER BY conname;"
    src_checks=$(psql_admin "$src_db" -c "$check_constraints_sql")
    tgt_checks=$(psql_admin "$tgt_db" -c "$check_constraints_sql")
    if [ "$src_checks" != "$tgt_checks" ]; then
        ops_die "validate: check constraints differ between source and target"
    fi

    # 4. The restored database must not be reachable by the runtime roles yet.
    # scripts/restore/postgres.sh restores with --no-acl and leaves CONNECT
    # revoked on purpose: a recovered database only becomes live through the
    # controlled recovery rollout. Asserting the grants match the source would
    # therefore assert the opposite of the design. The source count is checked
    # too, so a query that silently matches nothing cannot pass this step.
    grants_sql="
SELECT count(*)
FROM information_schema.role_table_grants
WHERE table_schema = 'public'
  AND grantee IN ('judgify_api','judgify_worker');"
    src_grants=$(psql_admin "$src_db" -c "$grants_sql")
    tgt_grants=$(psql_admin "$tgt_db" -c "$grants_sql")
    case "$src_grants" in
        ''|0|*[!0-9]*) ops_die "validate: source database carries no runtime table grants to compare against" ;;
    esac
    if [ "$tgt_grants" != "0" ]; then
        ops_die "validate: restored database already grants runtime roles table access ($tgt_grants grants)"
    fi

    # 5. Migration revision history matches.
    revisions_sql="
SELECT version || '|' || type::text || '|' || applied::text || '|' || total::text || '|' || hash
FROM public.atlas_schema_revisions
ORDER BY version;"
    src_revisions=$(psql_migrator "$src_db" -c "$revisions_sql")
    tgt_revisions=$(psql_migrator "$tgt_db" -c "$revisions_sql")
    if [ "$src_revisions" != "$tgt_revisions" ]; then
        ops_die "validate: migration revision history differs between source and target"
    fi
    [ -n "$src_revisions" ] ||
        ops_die "validate: no migration revisions on source"

    printf 'backup_restore_driver: validate passed\n'
}

# ---------------------------------------------------------------------------
# cleanup <src_svc> <src_db> <tgt_svc> <tgt_db>
# Safe to call twice; must never fail the test run (called with || true).
# ---------------------------------------------------------------------------
cmd_cleanup() {
    src_svc=$1; tgt_svc=$3

    container=$(_state_get container) || true
    pgpassfile=$(_state_get pgpassfile) || true
    migrator_binary=$(_state_get migrator_binary) || true
    admin_binary=$(_state_get admin_binary) || true

    # Stop container.
    if [ -n "$container" ]; then
        docker rm -f "$container" >/dev/null 2>&1 || true
    fi

    # Remove temporary binaries and the pgpass file.
    for f in "$pgpassfile" "$migrator_binary" "$admin_binary"; do
        [ -n "$f" ] && rm -f "$f" 2>/dev/null || true
    done

    # Remove only the driver's own sections from ~/.pg_service.conf.
    # Handles the case where the file does not exist.
    if [ -f "$PG_SERVICE_CONF" ] && [ -n "$src_svc" ] && [ -n "$tgt_svc" ]; then
        pg_service_tmp="${PG_SERVICE_CONF}.brt.tmp"
        awk -v s1="$src_svc" -v s2="$tgt_svc" '
            /^\[/ {
                sec = substr($0, 2, index($0, "]") - 2)
                skip = (sec == s1 || sec == s2)
            }
            !skip { print }
        ' "$PG_SERVICE_CONF" >"$pg_service_tmp" 2>/dev/null || true
        if [ -f "$pg_service_tmp" ]; then
            mv "$pg_service_tmp" "$PG_SERVICE_CONF" 2>/dev/null || true
            # Whitespace survives a size check, so test for real content and
            # never leave a stray file in the operator's home directory.
            if [ -z "$(tr -d '[:space:]' <"$PG_SERVICE_CONF" 2>/dev/null)" ]; then
                rm -f "$PG_SERVICE_CONF" 2>/dev/null || true
            fi
        fi
    fi

    # Remove state file.
    rm -f "$STATE_FILE" 2>/dev/null || true

    printf 'backup_restore_driver: cleanup complete\n'
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------
[ "$#" -eq 5 ] || ops_die "usage: backup_restore_driver.sh prepare|validate|cleanup <src_svc> <src_db> <tgt_svc> <tgt_db>"
subcommand=$1
shift

case "$subcommand" in
    prepare)  cmd_prepare  "$@" ;;
    validate) cmd_validate "$@" ;;
    cleanup)  cmd_cleanup  "$@" ;;
    *) ops_die "unknown subcommand: $subcommand" ;;
esac
