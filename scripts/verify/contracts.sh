#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
. "$ROOT/scripts/lib/operations.sh"

for script in \
	"$ROOT/scripts/verify/baseline_test.sh" \
	"$ROOT/scripts/verify/database_baseline_test.sh" \
	"$ROOT/scripts/verify/generated_test.sh" \
	"$ROOT/scripts/verify/workflows_test.sh" \
	"$ROOT/scripts/verify/observability_test.sh" \
	"$ROOT/scripts/verify/topology_test.sh" \
	"$ROOT/scripts/verify/migrations_test.sh" \
	"$ROOT/scripts/verify/resilience_test.sh" \
	"$ROOT/test/operations/rollout_test.sh"; do
	ops_require_trusted_executable "$script" "verification contract"
	"$script"
done

"$ROOT/scripts/verify/resilience.sh" --check
printf '%s\n' "static, topology, observability, migration, and rollout contracts verified"
