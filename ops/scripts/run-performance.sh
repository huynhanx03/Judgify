#!/usr/bin/env bash
set -euo pipefail

profile=$1
test -n "$profile" || { echo "usage: run-performance.sh <smoke|normal|spike|soak>" >&2; exit 2; }
: "$TARGET_URL"

ops_dir=$(cd -- "$(dirname -- "$0")/.." && pwd)
scenario="$ops_dir/performance/k6/scenarios"

case "$profile" in
  smoke) script="$scenario/smoke.js" ;;
  normal) script="$scenario/normal-1k-ccu.js" ;;
  spike) script="$scenario/contest-final-spike.js" ;;
  soak) script="$scenario/soak.js" ;;
  *) echo "unknown profile: $profile" >&2; exit 2 ;;
esac

exec k6 run "$script"

