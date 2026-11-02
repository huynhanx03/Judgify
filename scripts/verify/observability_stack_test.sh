#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
alloy="$root/ops/stack/alloy/config.alloy"
loki="$root/ops/stack/loki/config.yml"
compose="$root/ops/stack/docker-compose.yml"
prometheus="$root/ops/stack/prometheus/prometheus.yml"

grep -Fq 'relabel_rules = loki.relabel.judgify.rules' "$alloy"
grep -Fq 'target_label  = "service"' "$alloy"
grep -Fq 'environment = "production"' "$alloy"
grep -Fq 'delete_request_store: filesystem' "$loki"
grep -Fq 'otelcol.receiver.otlp "judgify"' "$alloy"
grep -Fq 'grafana/alloy@sha256:' "$compose"
grep -Fq 'grafana/loki@sha256:' "$compose"
grep -Fq 'grafana/tempo@sha256:' "$compose"
grep -Fq 'job_name: judgify-api' "$prometheus"
grep -Fq 'job_name: judgify-worker' "$prometheus"
grep -Fq 'host.docker.internal:19092' "$prometheus"
grep -Fq 'host.docker.internal:19093' "$prometheus"
if rg -n 'target_label\s*=\s*"(request_id|correlation_id|trace_id|span_id|user_id)"' "$alloy"; then
  echo "observability stack: high-cardinality identity promoted to Loki label" >&2
  exit 1
fi

echo "observability stack contract passed"
