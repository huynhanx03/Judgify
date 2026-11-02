#!/usr/bin/env bash
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
website="$root/website"
stack="$root/test/e2e/stack.sh"

cleanup() {
  bash "$stack" down || true
}
trap cleanup EXIT

(
  cd "$website"
  npm ci
  npm run build
)

bash "$stack" up
env_file="$(bash "$stack" env)"
[[ -f "$env_file" ]] || {
  echo "frontend E2E verification: stack did not produce an environment file" >&2
  exit 1
}

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

(
  cd "$website"
  npm run test:e2e:release
)

printf 'Frontend E2E verification passed\n'
