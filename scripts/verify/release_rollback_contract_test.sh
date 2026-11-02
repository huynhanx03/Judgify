#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
release="$root/scripts/deploy/release.sh"
rollback="$root/scripts/deploy/rollback.sh"

grep -Fq 'JUDGIFY_DEPLOY_ACK' "$release"
grep -Fq 'verify-migration-compatibility' "$release"
grep -Fq 'JUDGIFY_ROLLBACK_ACK' "$rollback"
grep -Fq 'rollback target must be the immediately previous N-1 release' "$rollback"
grep -Fq 'Database rollback is deliberately absent' "$rollback"
grep -Fq 'verify-current-schema-compatible' "$rollback"
if grep -Eq 'migrate|down|drop database|rollback.*schema' "$rollback"; then
  # The script may mention schema compatibility, but must never execute a DB rollback.
  if grep -Eq 'docker compose[^\n]*(down|migrate)|atlas[^\n]*(down|rollback)|DROP DATABASE' "$rollback"; then
    echo "rollback contract: database rollback command detected" >&2
    exit 1
  fi
fi

echo "release/rollback contract passed"
