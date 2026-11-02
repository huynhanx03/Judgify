#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
verifier="$root/scripts/verify/migrations.sh"
[[ -x "$verifier" ]] || { echo "migration verifier is not executable" >&2; exit 1; }

required=(
  'atlas migrate validate'
  'runtime auto-migration is forbidden'
  'database_baseline.sh'
  'atlas.sum inventory differs from SQL inventory'
)
for contract in "${required[@]}"; do
  grep -Fq "$contract" "$verifier" || {
    echo "migration verifier is missing contract: $contract" >&2
    exit 1
  }
done
"$verifier" --check
printf '%s\n' "migration verifier contract passed"
