#!/usr/bin/env bash
set -euo pipefail

name=$1
test -n "$name" || { echo "usage: create-report.sh <run-id>" >&2; exit 2; }

ops_dir=$(cd -- "$(dirname -- "$0")/.." && pwd)
destination="$ops_dir/performance/baselines/$name.md"

test ! -e "$destination" || { echo "refusing to overwrite $destination" >&2; exit 1; }
cp "$ops_dir/performance/report-template.md" "$destination"
printf 'created %s\n' "$destination"

