#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

check_tools() {
  command -v git >/dev/null
  command -v go >/dev/null
  command -v npm >/dev/null
}

if [[ "${1:-}" == "--check-tools" ]]; then
  check_tools
  exit 0
fi

check_tools
git -C "$repo_root" status --short
git -C "$repo_root/../go-common" status --short
(cd "$repo_root/../go-common" && go test ./...)
(cd "$repo_root/api" && go test ./...)
(cd "$repo_root/website" && npm run lint)
(cd "$repo_root/website" && npm run build)
