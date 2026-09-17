#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
template="$root/deploy/envoy/envoy.yaml.tmpl"
compose="$root/deploy/compose.production.yml"
api="$root/api/internal/infrastructure/authz/check_server.go"

grep -Fq 'envoy.filters.http.ext_authz' "$template"
grep -Fq 'failure_mode_allow: false' "$template"
grep -Fq 'transport_api_version: V3' "$template"
grep -Fq 'authz_client_certificate' "$template"
grep -Fq 'authz_server_ca_certificate' "$template"
grep -Fq 'grpcserver.LoadServerMTLS' "$root/api/internal/infrastructure/authz/tls.go"
grep -Fq 'grpcserver.New' "$root/api/internal/infrastructure/api.go"
grep -Fq 'RequireAndVerifyClientCert' "$root/../go-common/pkg/common/grpc/server/tls.go"
grep -Fq 'StatusCode_Unauthorized' "$api"
grep -Fq 'StatusCode_Forbidden' "$api"
grep -Fq 'StatusCode_ServiceUnavailable' "$api"
grep -Fq 'authz_server_certificate' "$compose"

echo "Envoy ext_authz contract passed"
