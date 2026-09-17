#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
renderer="${repo_root}/scripts/deploy/render-envoy.py"
template="${repo_root}/deploy/envoy/envoy.yaml.tmpl"
output="$(mktemp)"
trap 'rm -f "$output"' EXIT

if python3 "$renderer" --template "$template" --output "$output" --host 'bad host' >/dev/null 2>&1; then
  echo "envoy renderer: accepted invalid hostname" >&2
  exit 1
fi

python3 "$renderer" --template "$template" --output "$output" --host 'judgify.example.com'
grep -Fq 'judgify.example.com' "$output"
if grep -Fq '__JUDGIFY_PUBLIC_HOST__' "$output"; then
  echo "envoy renderer: left an unresolved template value" >&2
  exit 1
fi
if ! grep -Fq 'pack_trace_reason: false' "$output"; then
  echo "envoy renderer: request IDs may be trace-mutated" >&2
  exit 1
fi
if ! grep -Fq 'preserve_external_request_id: false' "$output"; then
  echo "envoy renderer: browser request IDs may be trusted" >&2
  exit 1
fi

echo "envoy renderer contract passed"
