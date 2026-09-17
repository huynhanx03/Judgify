#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
test -x "$repo_root/scripts/verify/baseline.sh"
"$repo_root/scripts/verify/baseline.sh" --check-tools
