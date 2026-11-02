#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/../lib/operations.sh"

: "${PGDATABASE:?set the restored PGDATABASE}"
ops_safe_token "$PGDATABASE" 63 || ops_die "PGDATABASE is not a safe PostgreSQL identifier"
ops_require_command psql

export PGAPPNAME=judgify-restore-validator

psql --no-psqlrc -X -v ON_ERROR_STOP=1 <<'SQL'
DO $validation$
DECLARE
    missing text[];
    invalid_constraints bigint;
    invalid_jobs bigint;
    invalid_outbox bigint;
    authorization_rows bigint;
    migration_rows bigint;
    invalid_migrations bigint;
BEGIN
    SELECT array_agg(required.name ORDER BY required.name)
    INTO missing
    FROM (
        VALUES
            ('users'),
            ('atlas_schema_revisions'),
            ('auth_sessions'),
            ('casbin_rules'),
            ('authorization_states'),
            ('problems'),
            ('submissions'),
            ('judge_jobs'),
            ('outbox_events'),
            ('audit_logs')
    ) AS required(name)
    WHERE to_regclass('public.' || required.name) IS NULL;

    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'restore is missing required tables: %', missing;
    END IF;

    SELECT count(*)
    INTO invalid_constraints
    FROM pg_constraint
    WHERE connamespace = 'public'::regnamespace
      AND NOT convalidated;
    IF invalid_constraints <> 0 THEN
        RAISE EXCEPTION 'restore contains % unvalidated constraints', invalid_constraints;
    END IF;

    SELECT count(*)
    INTO invalid_jobs
    FROM judge_jobs
    WHERE attempts > max_attempts
       OR (state = 'leased' AND (lease_until IS NULL OR heartbeat_at IS NULL))
       OR (state <> 'leased' AND lease_until IS NOT NULL);
    IF invalid_jobs <> 0 THEN
        RAISE EXCEPTION 'restore contains % invalid judge jobs', invalid_jobs;
    END IF;

    SELECT count(*)
    INTO invalid_outbox
    FROM outbox_events
    WHERE attempt_count > 100000
       OR (state = 'leased' AND leased_until IS NULL)
       OR (state = 'published' AND published_at IS NULL)
       OR (state = 'dead' AND dead_at IS NULL);
    IF invalid_outbox <> 0 THEN
        RAISE EXCEPTION 'restore contains % invalid outbox events', invalid_outbox;
    END IF;

    SELECT count(*) INTO authorization_rows FROM authorization_states;
    IF authorization_rows <> 1 THEN
        RAISE EXCEPTION 'authorization state cardinality is %, expected 1', authorization_rows;
    END IF;

    SELECT count(*), count(*) FILTER (
        WHERE error <> ''
           OR error_stmt <> ''
           OR (type = 2 AND (applied <> total OR total <= 0 OR hash = ''))
    )
    INTO migration_rows, invalid_migrations
    FROM atlas_schema_revisions;
    IF migration_rows = 0 THEN
        RAISE EXCEPTION 'restore contains no migration revision evidence';
    END IF;
    IF invalid_migrations <> 0 THEN
        RAISE EXCEPTION 'restore contains % incomplete or failed migration revisions', invalid_migrations;
    END IF;
END
$validation$;

-- Force PostgreSQL to read every foreign-key definition and surface any
-- invalid catalog state before traffic is admitted.
SELECT conname
FROM pg_constraint
WHERE connamespace = 'public'::regnamespace
  AND contype = 'f'
ORDER BY conname;
SQL

printf '%s\n' "restored database invariants validated"
