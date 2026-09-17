#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture_root="$repo_root/scripts/verify/testdata/generated"
workspace="$(mktemp -d)"
trap 'rm -rf "$workspace"' EXIT

test -x "$repo_root/scripts/verify/generated.sh"

stable_repo="$workspace/stable"
mkdir -p "$stable_repo"
cp -R "$fixture_root/stable/." "$stable_repo"
VERIFY_REPO_ROOT="$stable_repo" "$repo_root/scripts/verify/generated.sh"

drift_repo="$workspace/drift"
mkdir -p "$drift_repo"
cp -R "$fixture_root/drift/." "$drift_repo"

if output="$(VERIFY_REPO_ROOT="$drift_repo" "$repo_root/scripts/verify/generated.sh" 2>&1)"; then
  echo "expected generated drift check to fail" >&2
  exit 1
fi

case "$output" in
  *"api/internal/ent/generate/value.txt"*) ;;
  *)
    echo "drift output did not name the changed file" >&2
    echo "$output" >&2
    exit 1
    ;;
esac

test "$(sed -n '1p' "$drift_repo/api/internal/ent/generate/value.txt")" = "changed"
