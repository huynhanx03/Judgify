#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
verifier="$root_dir/scripts/verify/database_baseline.sh"
workflow="$root_dir/.github/workflows/ci.yml"
seed="$root_dir/scripts/seed.sql"

if [[ ! -x "$verifier" ]]; then
  echo "database baseline verifier is missing or not executable: $verifier" >&2
  exit 1
fi
if [[ ! -f "$workflow" ]]; then
  echo "CI workflow is missing: $workflow" >&2
  exit 1
fi

bash -n "$verifier"

require_literal() {
  local file="$1"
  local value="$2"
  if ! grep -Fq -- "$value" "$file"; then
    echo "required contract is missing from $file: $value" >&2
    exit 1
  fi
}

require_absent_literal() {
  local file="$1"
  local value="$2"
  if grep -Fq -- "$value" "$file"; then
    echo "forbidden legacy contract remains in $file: $value" >&2
    exit 1
  fi
}

require_literal "$verifier" 'trap cleanup EXIT'
require_literal "$verifier" 'postgres:16'
require_literal "$verifier" '--pull never'
require_literal "$verifier" 'JUDGIFY_VERIFY_POSTGRES_IMAGE'
require_literal "$verifier" '@sha256:'
require_literal "$verifier" 'PostgreSQL init process complete; ready for start up.'
require_literal "$verifier" 'deploy/postgres/init-roles.sql'
require_literal "$verifier" 'go build -trimpath -o "$migrator_binary" ./cmd/migrate'
require_literal "$verifier" 'run_production_migrator'
require_literal "$verifier" 'production migration command is not idempotent'
require_literal "$verifier" "version = '20260722073038'"
require_literal "$verifier" 'type = 2'
require_literal "$verifier" 'applied = total'
require_literal "$verifier" 'partial_hashes = '\''[]'\''::jsonb'
require_literal "$verifier" 'atlas schema diff'
require_literal "$verifier" 'env://src'
require_literal "$verifier" 'desired relational state'
require_literal "$verifier" 'Atlas OSS does not model our PostgreSQL'
require_literal "$verifier" 'pg_proc'
require_literal "$verifier" 'pg_trigger'
require_literal "$verifier" 'judgify_reject_append_only_mutation'
require_literal "$verifier" 'expect_sqlstate 55000'
require_literal "$verifier" 'expect_sqlstate 42501'
require_literal "$verifier" 'ON CONFLICT (id) DO NOTHING'
require_literal "$verifier" 'ON CONFLICT (id) DO UPDATE'
require_literal "$verifier" '"TRUNCATE TABLE public.$table CASCADE;"'
require_literal "$verifier" 'problem_revisions'
require_literal "$verifier" 'difficulty_revisions'
require_literal "$verifier" 'element_revisions'
require_literal "$verifier" 'tag_revisions'
require_literal "$verifier" 'tag_revision_elements'
require_literal "$verifier" 'test_assets'
require_literal "$verifier" 'testcase_groups'
require_literal "$verifier" 'testcase_revisions'
require_literal "$verifier" 'testset_revisions'
require_literal "$verifier" 'has_schema_privilege'
require_literal "$verifier" 'has_database_privilege'
require_literal "$verifier" 'judgify_migrator'
require_literal "$verifier" 'judgify_api'
require_literal "$verifier" 'judgify_worker'
require_absent_literal "$verifier" 'atlas migrate apply'
require_absent_literal "$verifier" 'scripts/seed.sql'
require_absent_literal "$seed" 'TRUNCATE TABLE "audit_logs"'

require_literal "$workflow" 'permissions:'
require_literal "$workflow" 'contents: read'
require_literal "$workflow" 'push:'
require_literal "$workflow" 'branches: [main]'
require_literal "$workflow" 'JUDGIFY_VERIFY_POSTGRES_IMAGE'
require_literal "$workflow" 'docker pull "$JUDGIFY_VERIFY_POSTGRES_IMAGE"'
require_literal "$workflow" 'if [[ "$result" != "success" ]]; then'
require_absent_literal "$workflow" '"$result" != "success" && "$result" != "skipped"'

echo "database baseline verifier and CI gate contracts: ok"
