#!/usr/bin/env bash
set -euo pipefail

default_repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
repo_root="${VERIFY_REPO_ROOT:-$default_repo_root}"
api_root="$repo_root/api"
generated_root="$api_root/internal/ent/generate"
generated_roots=("$generated_root")
mocks_enabled=false

if [[ ! -f "$api_root/go.mod" ]]; then
  echo "generated check: missing $api_root/go.mod" >&2
  exit 2
fi
if [[ ! -d "$generated_root" ]]; then
  echo "generated check: missing $generated_root" >&2
  exit 2
fi
if [[ -f "$repo_root/Makefile" ]]; then
  while IFS= read -r mocks_root; do
    generated_roots+=("$mocks_root")
    mocks_enabled=true
  done < <(
    find "$api_root/internal" -mindepth 2 -maxdepth 2 -type d -name mocks -print |
      LC_ALL=C sort
  )
fi

checksum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
    return
  fi

  echo "generated check: sha256sum or shasum is required" >&2
  exit 2
}

snapshot() {
  find "${generated_roots[@]}" -type f -print |
    LC_ALL=C sort |
    while IFS= read -r file; do
      relative="${file#"$repo_root"/}"
      printf '%s\t%s\n' "$relative" "$(checksum "$file")"
    done
}

workspace="$(mktemp -d)"
trap 'rm -rf "$workspace"' EXIT
before="$workspace/before.txt"
after="$workspace/after.txt"

snapshot >"$before"
(cd "$api_root" && go generate ./internal/ent)
if [[ "$mocks_enabled" == true ]]; then
  make --no-print-directory -C "$repo_root" mocks
fi
snapshot >"$after"

if cmp -s "$before" "$after"; then
  exit 0
fi

echo "generated check: generated files changed" >&2
diff -u "$before" "$after" >&2 || true
exit 1
