#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflows="$root/.github/workflows"
ci="$workflows/ci.yml"
integration="$workflows/integration.yml"
release="$workflows/release-evidence.yml"
labeler="$workflows/labeler.yml"

fail() {
  printf 'workflow contract failed: %s\n' "$*" >&2
  exit 1
}

require_literal() {
  local file="$1"
  local literal="$2"
  grep -Fq -- "$literal" "$file" || fail "$(basename "$file") is missing: $literal"
}

for file in "$ci" "$integration" "$release" "$labeler"; do
  [[ -s "$file" ]] || fail "missing workflow: $(basename "$file")"
  require_literal "$file" "permissions:"
  require_literal "$file" "contents: read"
  require_literal "$file" "persist-credentials: false"
  if grep -Fq 'pull_request_target:' "$file"; then
    fail "$(basename "$file") uses pull_request_target"
  fi
  while IFS= read -r action; do
    [[ "$action" =~ @([0-9a-f]{40})([[:space:]]|$) ]] ||
      fail "$(basename "$file") contains an unpinned action: $action"
  done < <(grep -E '^[[:space:]]*uses:[[:space:]]+' "$file")
done

for file in "$ci" "$integration" "$release"; do
  if grep -Eq '(^|[[:space:]])(contents|packages|actions|deployments|id-token|pull-requests):[[:space:]]+write' "$file"; then
    fail "$(basename "$file") grants a write permission"
  fi
done

require_literal "$labeler" 'pull-requests: write'
require_literal "$labeler" 'timeout-minutes: 5'
if grep -Eq '(^|[[:space:]])(contents|packages|actions|deployments|id-token):[[:space:]]+write' "$labeler"; then
  fail "labeler workflow grants a write permission beyond pull-request labels"
fi

for canonical_gate in \
  'scripts/verify/go.sh' \
  'scripts/verify/frontend.sh' \
  'scripts/verify/frontend_e2e.sh' \
  'scripts/verify/contracts.sh' \
  'scripts/verify/supply_chain.sh --check' \
  'scripts/verify/migrations.sh --integration'; do
  require_literal "$ci" "$canonical_gate"
done
for track_gate in api-gate website-gate cross-stack-gate final-gate; do
  require_literal "$ci" "${track_gate}:"
done
require_literal "$ci" 'needs: [api-source, api-unit, api-security, api-database]'
require_literal "$ci" 'needs: [website-source, website-unit, website-security, website-build]'
require_literal "$ci" 'needs: [cross-stack-contracts, cross-stack-e2e, cross-stack-packaging]'
require_literal "$ci" 'needs: [api-gate, website-gate, cross-stack-gate]'
require_literal "$ci" 'syft'
require_literal "$ci" 'grype'
require_literal "$ci" 'gitleaks'
require_literal "$ci" 'cancel-in-progress: true'
require_literal "$ci" 'GO_COMMON_REF'
require_literal "$ci" '@sha256:'

require_literal "$integration" 'workflow_dispatch:'
require_literal "$integration" 'cancel-in-progress: false'
require_literal "$integration" 'environment: judgify-integration'
require_literal "$integration" 'runs-on: [self-hosted, linux, x64, judgify-integration]'
require_literal "$integration" 'DESTROY_ONLY_JUDGIFY_RESTORE_TEST_DATABASES'
require_literal "$integration" 'LOAD_ONLY_JUDGIFY_TEST'
require_literal "$integration" 'CHAOS_ONLY_JUDGIFY_TEST'
require_literal "$integration" 'JUDGIFY_CHAOS_DRIVER'
require_literal "$integration" 'JUDGIFY_CHAOS_MARKER_FILE'
require_literal "$integration" 'JUDGIFY_CHAOS_VOLUME'
require_literal "$integration" 'JUDGIFY_WS_UNREAD_TOPIC'
require_literal "$integration" 'if-no-files-found: error'
require_literal "$integration" 'retention-days: 30'

require_literal "$release" 'workflow_dispatch:'
require_literal "$release" 'environment: judgify-release-evidence'
require_literal "$release" 'runs-on: [self-hosted, linux, x64, judgify-release]'
require_literal "$release" 'GENERATE_UNSIGNED_RELEASE_EVIDENCE'
require_literal "$release" 'JUDGIFY_RELEASE_IMAGES_FILE'
require_literal "$release" 'make verify-release'
require_literal "$release" 'if-no-files-found: error'
require_literal "$release" 'retention-days: 30'
if grep -Eq '(^|[[:space:]])(cosign|gh[[:space:]]+release|docker[[:space:]]+push|scripts/deploy/)' "$release"; then
  fail "release-evidence workflow must not sign, publish, or deploy"
fi

printf '%s\n' 'workflow permissions, pinning, isolation, gates, and evidence retention verified'
