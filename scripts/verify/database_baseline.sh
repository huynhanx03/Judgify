#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
. "$root_dir/scripts/lib/operations.sh"
api_dir="$root_dir/api"
migration_dir="$api_dir/internal/ent/migrate/migrations"
role_bootstrap="$root_dir/deploy/postgres/init-roles.sql"
container_name="judgify-db-verify-$$-${RANDOM}"
work_dir="$(mktemp -d "$(ops_temp_root)/judgify-db-verify.XXXXXX")"
migrator_binary="$work_dir/judgify-migrate"
migrator_log="$work_dir/migrate.log"

database_admin_user="postgres"
database_admin_password="verifyadmin${RANDOM}${RANDOM}"
database_name="judgify"
dev_database_name="judgify_dev"
migrator_password="verifymigrator${RANDOM}${RANDOM}"
api_password="verifyapi${RANDOM}${RANDOM}"
worker_password="verifyworker${RANDOM}${RANDOM}"
database_image="${JUDGIFY_VERIFY_POSTGRES_IMAGE:-postgres:16}"

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT

fail() {
  echo "database baseline verification failed: $*" >&2
  exit 1
}

if [[ "$database_image" != postgres:16 ]] &&
  [[ ! "$database_image" =~ ^[^[:space:]@]+@sha256:[0-9a-f]{64}$ ]]; then
  fail "JUDGIFY_VERIFY_POSTGRES_IMAGE must be postgres:16 for local work or a digest-pinned image"
fi

docker run \
	--pull never \
  --rm \
  --detach \
  --name "$container_name" \
  --env "POSTGRES_USER=$database_admin_user" \
  --env "POSTGRES_PASSWORD=$database_admin_password" \
  --env "POSTGRES_DB=$database_name" \
  --publish "127.0.0.1::5432" \
  "$database_image" >/dev/null

ready=false
for _ in {1..60}; do
  # The official image briefly starts a bootstrap postmaster and then stops
  # it before exec'ing the final server. pg_isready alone can observe that
  # transient instance and race the shutdown, so require the entrypoint's
  # completed-init marker before accepting readiness.
  if docker logs "$container_name" 2>&1 |
      grep -Fq 'PostgreSQL init process complete; ready for start up.' &&
    docker exec "$container_name" pg_isready \
      --username "$database_admin_user" \
      --dbname "$database_name" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != "true" ]]; then
  fail "temporary PostgreSQL did not become ready"
fi

host_port="$(
  docker port "$container_name" 5432/tcp |
    awk -F: 'NR == 1 { print $NF }'
)"
if [[ ! "$host_port" =~ ^[0-9]+$ ]]; then
  fail "could not resolve temporary PostgreSQL port"
fi

# Reproduce the production role bootstrap before any application object is
# created. Passwords are process-local and never printed.
docker exec \
  --interactive \
  --env "POSTGRES_DB=$database_name" \
  --env "JUDGIFY_MIGRATOR_PASSWORD=$migrator_password" \
  --env "JUDGIFY_API_PASSWORD=$api_password" \
  --env "JUDGIFY_WORKER_PASSWORD=$worker_password" \
  "$container_name" \
  psql \
  --no-psqlrc \
  --set ON_ERROR_STOP=1 \
  --username "$database_admin_user" \
  --dbname "$database_name" \
  <"$role_bootstrap" >/dev/null

docker exec "$container_name" \
  createdb \
  --username "$database_admin_user" \
  --owner judgify_migrator \
  "$dev_database_name"

dev_database_url="postgres://judgify_migrator:$migrator_password@127.0.0.1:$host_port/$dev_database_name?sslmode=disable&search_path=public"

# Atlas remains the author-time relational parity check for tables, columns,
# indexes, foreign keys, and checks. Atlas OSS does not model our PostgreSQL
# function/trigger objects, so those objects have separate exact-catalog and
# behavioral verification below.
schema_drift="$(
  cd "$api_dir"
  atlas schema diff \
    --env local \
    --var "dev_url=$dev_database_url" \
    --from "file://$migration_dir" \
    --to "env://src" \
    --format '{{ if .Changes }}{{ sql . }}{{ end }}'
)"
if [[ -n "${schema_drift//[[:space:]]/}" ]]; then
  echo "schema drift detected between checked migrations and desired relational state:" >&2
  echo "$schema_drift" >&2
  exit 1
fi

# Build and execute the same finite, embedded migration command shipped to
# production. Atlas CLI is intentionally not used to apply production state.
(
  cd "$api_dir"
  env GOPROXY=off go build -trimpath -o "$migrator_binary" ./cmd/migrate
)

run_production_migrator() {
  (
    cd "$api_dir"
    env \
      GO_ENV=local \
      DATABASE_DRIVER=postgres \
      DATABASE_HOST=127.0.0.1 \
      DATABASE_PORT="$host_port" \
      DATABASE_USERNAME=judgify_migrator \
      DATABASE_PASSWORD="$migrator_password" \
      DATABASE_DATABASE="$database_name" \
      LOGGER_FILE_LOG_NAME="$migrator_log" \
      "$migrator_binary"
  )
}

admin_query() {
  local statement="$1"
  docker exec "$container_name" \
    psql \
    --no-psqlrc \
    --set ON_ERROR_STOP=1 \
    --tuples-only \
    --no-align \
    --username "$database_admin_user" \
    --dbname "$database_name" \
    --command "$statement"
}

role_query() {
  local role="$1"
  local password="$2"
  local statement="$3"
  docker exec \
    --env "PGPASSWORD=$password" \
    "$container_name" \
    psql \
    --no-psqlrc \
    --set ON_ERROR_STOP=1 \
    --tuples-only \
    --no-align \
    --host 127.0.0.1 \
    --username "$role" \
    --dbname "$database_name" \
    --command "$statement"
}

expect_sqlstate() {
  local expected_state="$1"
  local role="$2"
  local password="$3"
  local statement="$4"
  local label="$5"
  local output=""
  local status=0

  if output="$(
    docker exec \
      --env "PGPASSWORD=$password" \
      "$container_name" \
      psql \
      --no-psqlrc \
      --set ON_ERROR_STOP=1 \
      --set VERBOSITY=verbose \
      --host 127.0.0.1 \
      --username "$role" \
      --dbname "$database_name" \
      --command "$statement" 2>&1
  )"; then
    status=0
  else
    status=$?
  fi

  if [[ "$status" -eq 0 ]] || ! grep -Eq "ERROR:.*${expected_state}:" <<<"$output"; then
    echo "$output" >&2
    fail "$label did not fail with SQLSTATE $expected_state"
  fi
}

revision_contract_sql="
  SELECT count(*)::text || '|' ||
         count(*) FILTER (
           WHERE version = '20260722073038'
             AND type = 2
             AND applied = total
             AND total > 0
             AND hash <> ''
             AND error = ''
             AND error_stmt = ''
             AND partial_hashes = '[]'::jsonb
         )::text
  FROM public.atlas_schema_revisions;
"
revision_fingerprint_sql="
  SELECT md5(row_to_json(revision_row)::text)
  FROM (
    SELECT version, description, type, applied, total, executed_at,
           execution_time, error, error_stmt, hash, partial_hashes,
           operator_version
    FROM public.atlas_schema_revisions
    ORDER BY version
  ) AS revision_row;
"

run_production_migrator
revision_contract="$(admin_query "$revision_contract_sql")"
if [[ "$revision_contract" != "1|1" ]]; then
  fail "production migration revision contract is invalid"
fi
revision_before="$(admin_query "$revision_fingerprint_sql")"
if [[ ! "$revision_before" =~ ^[0-9a-f]{32}$ ]]; then
  fail "could not fingerprint the production migration revision"
fi

# The second production run must be a no-op, including revision metadata.
run_production_migrator
revision_contract="$(admin_query "$revision_contract_sql")"
revision_after="$(admin_query "$revision_fingerprint_sql")"
if [[ "$revision_contract" != "1|1" ]] || [[ "$revision_after" != "$revision_before" ]]; then
  fail "production migration command is not idempotent"
fi

role_contract="$(
  admin_query "
    SELECT count(*)
    FROM pg_roles
    WHERE rolname IN ('judgify_migrator', 'judgify_api', 'judgify_worker')
      AND rolcanlogin
      AND NOT rolsuper
      AND NOT rolcreatedb
      AND NOT rolcreaterole
      AND NOT rolreplication
      AND NOT rolbypassrls;
  "
)"
if [[ "$role_contract" != "3" ]]; then
  fail "production database roles have unsafe attributes"
fi

ownership_contract="$(
  admin_query "
    SELECT
      pg_get_userbyid(datdba) || '|' ||
      pg_get_userbyid((SELECT nspowner FROM pg_namespace WHERE nspname = 'public'))
    FROM pg_database
    WHERE datname = current_database();
  "
)"
if [[ "$ownership_contract" != "judgify_migrator|judgify_migrator" ]]; then
  fail "database or public schema is not owned by judgify_migrator"
fi

unexpected_relation_owners="$(
  admin_query "
    SELECT count(*)
    FROM pg_class AS relation
    JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
    WHERE namespace.nspname = 'public'
      AND relation.relkind IN ('r', 'p', 'S')
      AND pg_get_userbyid(relation.relowner) <> 'judgify_migrator';
  "
)"
if [[ "$unexpected_relation_owners" != "0" ]]; then
  fail "public relations are not exclusively owned by judgify_migrator"
fi

# Derive the append-only table set from the checked-in baseline migration.
# These tables are excluded from the "full DML" check because they carry their
# own stricter append-only ACL verified separately below.
# activated_judge_capabilities is excluded for a different reason: it has a
# dedicated per-role check below (platform_activation_acl_contract) that
# verifies both roles have full DML and neither has TRUNCATE.
#
# The derived set covers every table in the append-only catalog, including:
# audit_logs, content_render_artifacts, content_render_sources,
# difficulty_revisions, element_revisions, judge_worker_capabilities,
# problem_revisions, tag_revision_elements, tag_revisions, test_assets,
# testcase_groups, testcase_revisions, testset_revisions, and 24 others.
migration_sql="$migration_dir/$(ls "$migration_dir" | grep '\.sql$' | tail -1)"
sql_append_only_tables="$(
  grep -E 'CREATE TRIGGER "[a-z_]+_append_only"' "$migration_sql" \
    | sed 's/CREATE TRIGGER "\([a-z_]*\)_append_only".*/\1/' \
    | sort
)"
if [[ -z "$sql_append_only_tables" ]]; then
  fail "could not derive append-only table set from migration SQL — extraction yielded empty result"
fi

# Build the SQL VALUES list from the derived set: one row per table name.
append_only_values="$(printf "%s\n" $sql_append_only_tables | sed "s/.*/('&')/" | paste -sd ',' -)"

runtime_mutable_dml_contract="$(
  admin_query "
    WITH runtime_roles(role_name) AS (
      VALUES ('judgify_api'), ('judgify_worker')
    ), append_only_tables(tablename) AS (
      VALUES $append_only_values
    )
    SELECT role_name || '|' ||
           bool_and(
             has_table_privilege(role_name, format('%I.%I', schemaname, tablename), 'SELECT')
             AND has_table_privilege(role_name, format('%I.%I', schemaname, tablename), 'INSERT')
             AND has_table_privilege(role_name, format('%I.%I', schemaname, tablename), 'UPDATE')
             AND has_table_privilege(role_name, format('%I.%I', schemaname, tablename), 'DELETE')
           )::text
    FROM runtime_roles
    CROSS JOIN pg_tables
    WHERE schemaname = 'public'
      AND tablename NOT IN (SELECT tablename FROM append_only_tables)
      AND tablename <> 'activated_judge_capabilities'
    GROUP BY role_name
    ORDER BY role_name;
  "
)"
if [[ "$runtime_mutable_dml_contract" != $'judgify_api|true\njudgify_worker|true' ]]; then
  fail "runtime roles do not have the expected table DML privileges"
fi

# activated_judge_capabilities is neither append-only nor taxonomy-restricted:
# both runtime roles have full DML. This dedicated check verifies that the table
# carries no unintended privilege restriction (TRUNCATE is reserved for the owner).
platform_activation_acl_contract="$(
  admin_query "
    SELECT role_name || '|' ||
           (
             has_table_privilege(role_name, 'public.activated_judge_capabilities', 'SELECT')
             AND has_table_privilege(role_name, 'public.activated_judge_capabilities', 'INSERT')
             AND has_table_privilege(role_name, 'public.activated_judge_capabilities', 'UPDATE')
             AND has_table_privilege(role_name, 'public.activated_judge_capabilities', 'DELETE')
             AND NOT has_table_privilege(role_name, 'public.activated_judge_capabilities', 'TRUNCATE')
           )::text
    FROM (VALUES ('judgify_api'), ('judgify_worker')) AS runtime_roles(role_name)
    ORDER BY role_name;
  "
)"
if [[ "$platform_activation_acl_contract" != $'judgify_api|true\njudgify_worker|true' ]]; then
  fail "platform activation table does not have the expected runtime DML ACL"
fi

# Derive the taxonomy-history subset (tables where judgify_worker also lacks INSERT)
# from the REVOKE INSERT ... FROM judgify_worker statements in the ACL DO block.
sql_taxonomy_tables="$(
  grep -E 'REVOKE INSERT ON TABLE "public"\."[a-z_]+" FROM judgify_worker' "$migration_sql" \
    | sed 's/.*REVOKE INSERT ON TABLE "public"\.\"\([a-z_]*\)\" FROM judgify_worker.*/\1/' \
    | sort
)"
if [[ -z "$sql_taxonomy_tables" ]]; then
  fail "could not derive taxonomy append-only table set from migration SQL — extraction yielded empty result"
fi

# Drift guard: the append-only table set derived from migration SQL must be
# identical to the set the live database reports from pg_trigger. Any mismatch
# means the migration was applied without regenerating the trigger footer.
live_append_only_tables="$(
  admin_query "
    SELECT string_agg(relation.relname, E'\n' ORDER BY relation.relname)
    FROM pg_trigger AS trigger
    JOIN pg_class AS relation ON relation.oid = trigger.tgrelid
    JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
    JOIN pg_proc AS procedure ON procedure.oid = trigger.tgfoid
    WHERE namespace.nspname = 'public'
      AND procedure.proname = 'judgify_reject_append_only_mutation'
      AND NOT trigger.tgisinternal;
  "
)"
if [[ -z "$live_append_only_tables" ]]; then
  fail "live database reports no append-only triggers — pg_trigger query yielded empty result"
fi
if [[ "$sql_append_only_tables" != "$live_append_only_tables" ]]; then
  echo "migration SQL append-only set:" >&2
  echo "$sql_append_only_tables" >&2
  echo "live pg_trigger append-only set:" >&2
  echo "$live_append_only_tables" >&2
  fail "append-only table set from migration SQL does not match live pg_trigger catalog — migration drift detected"
fi

# Build VALUES list annotated with the taxonomy_history flag.
append_only_acl_values="$(
  printf "%s\n" $sql_append_only_tables | while IFS= read -r t; do
    if echo "$sql_taxonomy_tables" | grep -qx "$t"; then
      printf "('%s', true)\n" "$t"
    else
      printf "('%s', false)\n" "$t"
    fi
  done | paste -sd ',' -
)"

runtime_append_only_acl_contract="$(
  admin_query "
    WITH runtime_roles(role_name) AS (
      VALUES ('judgify_api'), ('judgify_worker')
    ), append_only_tables(tablename, taxonomy_history) AS (
      VALUES $append_only_acl_values
    )
    SELECT runtime_roles.role_name || '|' ||
           bool_and(
             has_table_privilege(runtime_roles.role_name, format('%I.%I', 'public', append_only_tables.tablename), 'SELECT')
             AND NOT has_table_privilege(runtime_roles.role_name, format('%I.%I', 'public', append_only_tables.tablename), 'UPDATE')
             AND NOT has_table_privilege(runtime_roles.role_name, format('%I.%I', 'public', append_only_tables.tablename), 'DELETE')
             AND NOT has_table_privilege(runtime_roles.role_name, format('%I.%I', 'public', append_only_tables.tablename), 'TRUNCATE')
             AND (
               runtime_roles.role_name = 'judgify_api'
               OR NOT append_only_tables.taxonomy_history
               OR NOT has_table_privilege(runtime_roles.role_name, format('%I.%I', 'public', append_only_tables.tablename), 'INSERT')
             )
             AND (
               runtime_roles.role_name <> 'judgify_api'
               OR has_table_privilege(runtime_roles.role_name, format('%I.%I', 'public', append_only_tables.tablename), 'INSERT')
             )
           )::text
    FROM runtime_roles
    CROSS JOIN append_only_tables
    GROUP BY runtime_roles.role_name
    ORDER BY runtime_roles.role_name;
  "
)"
if [[ "$runtime_append_only_acl_contract" != $'judgify_api|true\njudgify_worker|true' ]]; then
  fail "runtime roles do not have the expected append-only least-privilege ACL"
fi

runtime_ddl_capabilities="$(
  admin_query "
    WITH runtime_roles(role_name) AS (
      VALUES ('judgify_api'), ('judgify_worker')
    )
    SELECT count(*)
    FROM runtime_roles
    WHERE has_schema_privilege(role_name, 'public', 'CREATE')
       OR has_database_privilege(role_name, current_database(), 'CREATE')
       OR has_database_privilege(role_name, current_database(), 'TEMP')
       OR pg_has_role(role_name, 'judgify_migrator', 'MEMBER');
  "
)"
if [[ "$runtime_ddl_capabilities" != "0" ]]; then
  fail "runtime roles unexpectedly have DDL, temporary-table, or migrator membership capability"
fi

append_only_function_contract="$(
  admin_query "
    SELECT count(*)::text || '|' ||
           count(*) FILTER (
             WHERE procedure.prokind = 'f'
               AND procedure.pronargs = 0
               AND procedure.prorettype = 'trigger'::regtype
               AND language.lanname = 'plpgsql'
               AND NOT procedure.prosecdef
               AND NOT procedure.proleakproof
               AND procedure.provolatile = 'v'
               AND procedure.proparallel = 'u'
               AND pg_get_userbyid(procedure.proowner) = 'judgify_migrator'
           )::text
    FROM pg_proc AS procedure
    JOIN pg_namespace AS namespace ON namespace.oid = procedure.pronamespace
    JOIN pg_language AS language ON language.oid = procedure.prolang
    WHERE namespace.nspname = 'public'
      AND procedure.proname = 'judgify_reject_append_only_mutation';
  "
)"
if [[ "$append_only_function_contract" != "1|1" ]]; then
  fail "append-only trigger function catalog contract is invalid"
fi

# Expected trigger contract: every table in the migration-derived append-only set
# must have a trigger named <table>_append_only with tgtype=58 (BEFORE STATEMENT,
# UPDATE|DELETE|TRUNCATE), tgenabled=O (always fires), bound to the guard function.
expected_trigger_contract="$(
  printf "%s\n" $sql_append_only_tables | while IFS= read -r t; do
    printf "%s_append_only:%s:58:O:judgify_reject_append_only_mutation\n" "$t" "$t"
  done | sort | paste -sd ',' -
)"
append_only_trigger_contract="$(
  admin_query "
    SELECT string_agg(
      trigger.tgname || ':' || relation.relname || ':' ||
      trigger.tgtype::text || ':' || trigger.tgenabled::text || ':' ||
      procedure.proname,
      ',' ORDER BY trigger.tgname
    )
    FROM pg_trigger AS trigger
    JOIN pg_class AS relation ON relation.oid = trigger.tgrelid
    JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
    JOIN pg_proc AS procedure ON procedure.oid = trigger.tgfoid
    WHERE namespace.nspname = 'public'
      AND procedure.proname = 'judgify_reject_append_only_mutation'
      AND NOT trigger.tgisinternal;
  "
)"
if [[ "$append_only_trigger_contract" != "$expected_trigger_contract" ]]; then
  echo "expected trigger contract:" >&2
  echo "$expected_trigger_contract" | tr ',' '\n' >&2
  echo "actual trigger contract:" >&2
  echo "$append_only_trigger_contract" | tr ',' '\n' >&2
  fail "append-only trigger catalog contract is invalid"
fi

source_id="019c0000-0000-7000-8000-000000000101"
source_revision_id="019c0000-0000-7000-8000-000000000102"
artifact_id="019c0000-0000-7000-8000-000000000201"
audit_id="019c0000-0000-7000-8000-000000000301"
taxonomy_difficulty_id="019c0000-0000-7000-8000-000000000401"
taxonomy_difficulty_revision_id="019c0000-0000-7000-8000-000000000402"
taxonomy_element_id="019c0000-0000-7000-8000-000000000403"
taxonomy_element_revision_id="019c0000-0000-7000-8000-000000000404"
taxonomy_tag_id="019c0000-0000-7000-8000-000000000405"
taxonomy_tag_revision_id="019c0000-0000-7000-8000-000000000406"
taxonomy_tag_element_id="019c0000-0000-7000-8000-000000000407"

# Runtime writes use the API role. These inserts prove authentication,
# privileges, relational constraints, and the allowed append-only paths.
role_query judgify_api "$api_password" "
  BEGIN;
  INSERT INTO public.content_render_sources (
    id, owner_revision_id, source_kind, source_checksum
  ) VALUES (
    '$source_id', '$source_revision_id', 'problem_statement',
    decode(repeat('11', 32), 'hex')
  );
  INSERT INTO public.content_render_artifacts (
    id, render_source_kind, source_checksum, renderer_version,
    sanitizer_policy_version, status, diagnostics, render_source_id
  ) VALUES (
    '$artifact_id', 'problem_statement', decode(repeat('11', 32), 'hex'),
    'verify-renderer-v1', 'verify-sanitizer-v1', 'rejected',
    '[{\"code\":\"render_rejected\"}]'::jsonb, '$source_id'
  );
  INSERT INTO public.audit_logs (
    id, action, resource, resource_id, reason, before_data, after_data,
    correlation_id, occurred_at
  ) VALUES (
    '$audit_id', 'verify.append_only', 'database', 'baseline',
    'production database contract verification', '{}'::jsonb, '{}'::jsonb,
    'verify-database-baseline', CURRENT_TIMESTAMP
  );
  COMMIT;
" >/dev/null

# Idempotent producers may deliberately ignore a duplicate, which must not
# enter the UPDATE trigger path.
role_query judgify_api "$api_password" "
  INSERT INTO public.content_render_sources (
    id, owner_revision_id, source_kind, source_checksum
  ) VALUES (
    '$source_id', '$source_revision_id', 'problem_statement',
    decode(repeat('11', 32), 'hex')
  ) ON CONFLICT (id) DO NOTHING;
  INSERT INTO public.content_render_artifacts (
    id, render_source_kind, source_checksum, renderer_version,
    sanitizer_policy_version, status, diagnostics, render_source_id
  ) VALUES (
    '$artifact_id', 'problem_statement', decode(repeat('11', 32), 'hex'),
    'verify-renderer-v1', 'verify-sanitizer-v1', 'rejected',
    '[{\"code\":\"render_rejected\"}]'::jsonb, '$source_id'
  ) ON CONFLICT (id) DO NOTHING;
  INSERT INTO public.audit_logs (
    id, action, resource, resource_id, reason, before_data, after_data,
    correlation_id, occurred_at
  ) VALUES (
    '$audit_id', 'verify.append_only', 'database', 'baseline',
    'production database contract verification', '{}'::jsonb, '{}'::jsonb,
    'verify-database-baseline', CURRENT_TIMESTAMP
  ) ON CONFLICT (id) DO NOTHING;
" >/dev/null

inserted_counts="$(
  role_query judgify_api "$api_password" "
    SELECT
      (SELECT count(*) FROM public.content_render_sources WHERE id = '$source_id') ||
      '|' ||
      (SELECT count(*) FROM public.content_render_artifacts WHERE id = '$artifact_id') ||
      '|' ||
      (SELECT count(*) FROM public.audit_logs WHERE id = '$audit_id');
  "
)"
if [[ "$inserted_counts" != "1|1|1" ]]; then
  fail "append-only insert or ON CONFLICT DO NOTHING behavior is invalid"
fi

# Seed one valid immutable taxonomy graph as the migrator. It lets the owner
# trigger checks exercise the actual ON CONFLICT DO UPDATE path, rather than
# failing earlier on a missing row or a row-shape constraint.
role_query judgify_migrator "$migrator_password" "
  BEGIN;
  INSERT INTO public.difficulties (id, name, level, exp_reward, version)
    VALUES ('$taxonomy_difficulty_id', 'verify-taxonomy-difficulty', 900001, 0, 1);
  INSERT INTO public.elements (id, name, code, version)
    VALUES ('$taxonomy_element_id', 'verify-taxonomy-element', 'verify-taxonomy-element', 1);
  INSERT INTO public.tags (id, name, version)
    VALUES ('$taxonomy_tag_id', 'verify-taxonomy-tag', 1);
  INSERT INTO public.difficulty_revisions
    (id, difficulty_id, revision, name, level, exp_reward, checksum)
    VALUES ('$taxonomy_difficulty_revision_id', '$taxonomy_difficulty_id', 1,
      'verify-taxonomy-difficulty', 900001, 0, decode(repeat('41', 32), 'hex'));
  INSERT INTO public.element_revisions
    (id, element_id, revision, name, code, checksum)
    VALUES ('$taxonomy_element_revision_id', '$taxonomy_element_id', 1,
      'verify-taxonomy-element', 'verify-taxonomy-element', decode(repeat('42', 32), 'hex'));
  INSERT INTO public.tag_revisions
    (id, tag_id, revision, name, checksum)
    VALUES ('$taxonomy_tag_revision_id', '$taxonomy_tag_id', 1,
      'verify-taxonomy-tag', decode(repeat('43', 32), 'hex'));
  INSERT INTO public.tag_revision_elements
    (id, tag_revision_id, element_id, element_revision)
    VALUES ('$taxonomy_tag_element_id', '$taxonomy_tag_revision_id', '$taxonomy_element_id', 1);
  COMMIT;
" >/dev/null

# Taxonomy revision history is authored only by the API/admin transaction.
# The worker must fail the ACL check before row validation can matter.
for table in difficulty_revisions element_revisions tag_revisions tag_revision_elements; do
  expect_sqlstate 42501 judgify_worker "$worker_password" \
    "INSERT INTO public.$table DEFAULT VALUES;" \
    "judgify_worker $table INSERT privilege"
done

expect_taxonomy_upsert_acl() {
  local role="$1"
  local password="$2"
  local table="$3"
  local statement
  case "$table" in
    difficulty_revisions)
      statement="INSERT INTO public.difficulty_revisions (id, difficulty_id, revision, name, level, exp_reward, checksum) VALUES ('$taxonomy_difficulty_revision_id', '$taxonomy_difficulty_id', 1, 'verify-taxonomy-difficulty', 900001, 0, decode(repeat('41', 32), 'hex')) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;"
      ;;
    element_revisions)
      statement="INSERT INTO public.element_revisions (id, element_id, revision, name, code, checksum) VALUES ('$taxonomy_element_revision_id', '$taxonomy_element_id', 1, 'verify-taxonomy-element', 'verify-taxonomy-element', decode(repeat('42', 32), 'hex')) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;"
      ;;
    tag_revisions)
      statement="INSERT INTO public.tag_revisions (id, tag_id, revision, name, checksum) VALUES ('$taxonomy_tag_revision_id', '$taxonomy_tag_id', 1, 'verify-taxonomy-tag', decode(repeat('43', 32), 'hex')) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;"
      ;;
    tag_revision_elements)
      statement="INSERT INTO public.tag_revision_elements (id, tag_revision_id, element_id, element_revision) VALUES ('$taxonomy_tag_element_id', '$taxonomy_tag_revision_id', '$taxonomy_element_id', 1) ON CONFLICT (id) DO UPDATE SET element_revision = EXCLUDED.element_revision;"
      ;;
    *) fail "unknown taxonomy append-only table $table" ;;
  esac
  expect_sqlstate 42501 "$role" "$password" "$statement" "$role $table ON CONFLICT DO UPDATE"
}

for table in difficulty_revisions element_revisions tag_revisions tag_revision_elements; do
  expect_taxonomy_upsert_acl judgify_api "$api_password" "$table"
  expect_taxonomy_upsert_acl judgify_worker "$worker_password" "$table"
done

for table in $sql_append_only_tables; do
  expect_sqlstate 42501 judgify_api "$api_password" \
    "UPDATE public.$table SET id = id WHERE false;" \
    "$table UPDATE"
  expect_sqlstate 42501 judgify_api "$api_password" \
    "DELETE FROM public.$table WHERE false;" \
    "$table DELETE"
done

for table in $sql_append_only_tables; do
  expect_sqlstate 42501 judgify_worker "$worker_password" \
    "UPDATE public.$table SET id = id WHERE false;" \
    "worker $table UPDATE"
  expect_sqlstate 42501 judgify_worker "$worker_password" \
    "DELETE FROM public.$table WHERE false;" \
    "worker $table DELETE"
done

# The owner bypasses runtime ACLs, so these statements prove the database
# trigger itself protects every immutable table even when no row matches.
for table in $sql_append_only_tables; do
  expect_sqlstate 55000 judgify_migrator "$migrator_password" \
    "UPDATE public.$table SET id = id WHERE false;" \
    "migrator $table UPDATE"
  expect_sqlstate 55000 judgify_migrator "$migrator_password" \
    "DELETE FROM public.$table WHERE false;" \
    "migrator $table DELETE"
done

expect_sqlstate 55000 judgify_migrator "$migrator_password" "
  INSERT INTO public.difficulty_revisions
    (id, difficulty_id, revision, name, level, exp_reward, checksum)
    VALUES ('$taxonomy_difficulty_revision_id', '$taxonomy_difficulty_id', 1,
      'verify-taxonomy-difficulty', 900001, 0, decode(repeat('41', 32), 'hex'))
  ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
" "migrator difficulty_revisions ON CONFLICT DO UPDATE"
expect_sqlstate 55000 judgify_migrator "$migrator_password" "
  INSERT INTO public.element_revisions
    (id, element_id, revision, name, code, checksum)
    VALUES ('$taxonomy_element_revision_id', '$taxonomy_element_id', 1,
      'verify-taxonomy-element', 'verify-taxonomy-element', decode(repeat('42', 32), 'hex'))
  ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
" "migrator element_revisions ON CONFLICT DO UPDATE"
expect_sqlstate 55000 judgify_migrator "$migrator_password" "
  INSERT INTO public.tag_revisions
    (id, tag_id, revision, name, checksum)
    VALUES ('$taxonomy_tag_revision_id', '$taxonomy_tag_id', 1,
      'verify-taxonomy-tag', decode(repeat('43', 32), 'hex'))
  ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
" "migrator tag_revisions ON CONFLICT DO UPDATE"
expect_sqlstate 55000 judgify_migrator "$migrator_password" "
  INSERT INTO public.tag_revision_elements
    (id, tag_revision_id, element_id, element_revision)
    VALUES ('$taxonomy_tag_element_id', '$taxonomy_tag_revision_id', '$taxonomy_element_id', 1)
  ON CONFLICT (id) DO UPDATE SET element_revision = EXCLUDED.element_revision;
" "migrator tag_revision_elements ON CONFLICT DO UPDATE"

expect_sqlstate 42501 judgify_api "$api_password" "
  INSERT INTO public.content_render_sources (
    id, owner_revision_id, source_kind, source_checksum
  ) VALUES (
    '$source_id', '$source_revision_id', 'problem_statement',
    decode(repeat('11', 32), 'hex')
  ) ON CONFLICT (id) DO UPDATE SET source_kind = EXCLUDED.source_kind;
" "content_render_sources ON CONFLICT DO UPDATE"
expect_sqlstate 42501 judgify_api "$api_password" "
  INSERT INTO public.content_render_artifacts (
    id, render_source_kind, source_checksum, renderer_version,
    sanitizer_policy_version, status, diagnostics, render_source_id
  ) VALUES (
    '$artifact_id', 'problem_statement', decode(repeat('11', 32), 'hex'),
    'verify-renderer-v1', 'verify-sanitizer-v1', 'rejected',
    '[{\"code\":\"render_rejected\"}]'::jsonb, '$source_id'
  ) ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status;
" "content_render_artifacts ON CONFLICT DO UPDATE"
expect_sqlstate 42501 judgify_api "$api_password" "
  INSERT INTO public.audit_logs (
    id, action, resource, resource_id, reason, before_data, after_data,
    correlation_id, occurred_at
  ) VALUES (
    '$audit_id', 'verify.append_only', 'database', 'baseline',
    'production database contract verification', '{}'::jsonb, '{}'::jsonb,
    'verify-database-baseline', CURRENT_TIMESTAMP
  ) ON CONFLICT (id) DO UPDATE SET reason = EXCLUDED.reason;
" "audit_logs ON CONFLICT DO UPDATE"

# Runtime roles intentionally lack TRUNCATE. Exercise every trigger as the
# owning migrator, then independently prove API privilege denial. CASCADE
# avoids a foreign-key precondition hiding the statement trigger.
for table in $sql_append_only_tables; do
  expect_sqlstate 55000 judgify_migrator "$migrator_password" \
    "TRUNCATE TABLE public.$table CASCADE;" \
    "$table TRUNCATE"
done
for role in judgify_api judgify_worker; do
  password="$api_password"
  if [[ "$role" == "judgify_worker" ]]; then
    password="$worker_password"
  fi
  for table in $sql_append_only_tables; do
    expect_sqlstate 42501 "$role" "$password" \
      "TRUNCATE TABLE public.$table;" \
      "$role $table TRUNCATE privilege"
  done
done

echo "database baseline verification: ok; production migrator, roles, relational parity, and append-only objects verified"
