#!/usr/bin/env bash
set -euo pipefail

ops_dir=$(cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ops_dir"

docker compose --project-name judgify-ops --file stack/docker-compose.yml config >/dev/null

find stack/grafana/dashboards -name '*.json' -print0 | xargs -0 -r -n1 jq empty
find performance/k6 -name '*.js' -print0 | xargs -0 -r -n1 node --check

docker run --rm --entrypoint promtool \
  -v "$ops_dir/stack/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro" \
  -v "$ops_dir/provisioning/prometheus-rules:/etc/prometheus/rules:ro" \
  prom/prometheus:v3.5.0 check config /etc/prometheus/prometheus.yml

docker run --rm --entrypoint promtool \
  -v "$ops_dir/provisioning/prometheus-rules:/rules:ro" \
  prom/prometheus:v3.5.0 check rules /rules/judgify.yml

printf 'ops validation passed\n'
