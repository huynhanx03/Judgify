#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
dir="$(mktemp -d)"
suffix="${RANDOM}${RANDOM}"
net="judgify-envoy-e2e-${suffix}"
api="judgify-envoy-api-${suffix}"
web="judgify-envoy-web-${suffix}"
envoy="judgify-envoy-${suffix}"
port="$((20000 + RANDOM % 10000))"

cleanup() {
  docker rm --force "$envoy" "$api" "$web" >/dev/null 2>&1 || true
  docker network rm "$net" >/dev/null 2>&1 || true
  rm -rf "$dir"
}
trap cleanup EXIT

python3 "$root/scripts/deploy/render-envoy.py" --template "$root/deploy/envoy/envoy.yaml.tmpl" --output "$dir/envoy.yaml" --host example.test
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=example.test -keyout "$dir/key.pem" -out "$dir/cert.pem" >/dev/null 2>&1
cp "$dir/cert.pem" "$dir/authz_client_certificate"
cp "$dir/key.pem" "$dir/authz_client_private_key"
cp "$dir/cert.pem" "$dir/authz_server_ca_certificate"

docker network create "$net" >/dev/null
docker run -d --rm --name "$api" --network "$net" --network-alias api hashicorp/http-echo@sha256:fcb75f691c8b0414d670ae570240cbf95502cc18a9ba57e982ecac589760a186 -listen :8000 -text api >/dev/null
docker run -d --rm --name "$web" --network "$net" --network-alias website hashicorp/http-echo@sha256:fcb75f691c8b0414d670ae570240cbf95502cc18a9ba57e982ecac589760a186 -listen :3000 -text website >/dev/null
docker run -d --rm --name "$envoy" --network "$net" -p "${port}:8443" \
  -v "$dir/envoy.yaml:/etc/envoy/envoy.yaml:ro" -v "$dir/cert.pem:/run/secrets/tls_certificate:ro" -v "$dir/key.pem:/run/secrets/tls_private_key:ro" \
  -v "$dir/authz_client_certificate:/run/secrets/authz_client_certificate:ro" -v "$dir/authz_client_private_key:/run/secrets/authz_client_private_key:ro" -v "$dir/authz_server_ca_certificate:/run/secrets/authz_server_ca_certificate:ro" \
  envoyproxy/envoy@sha256:6bf0d37dd5e8eac4b37effe89a1aec4760f7f2ce860d1fd6af22147eb87a5058 -c /etc/envoy/envoy.yaml --base-id 11 >/dev/null

for _ in $(seq 1 20); do
  curl --silent --insecure --resolve "example.test:${port}:127.0.0.1" -H 'Host: example.test' "https://example.test:${port}/" >/dev/null && break
  sleep 0.2
done
headers="$dir/headers"
body="$(curl --silent --show-error --insecure --dump-header "$headers" --resolve "example.test:${port}:127.0.0.1" -H 'Host: example.test' -H 'X-Request-ID: 019f6abb-8dd5-7581-8449-2b9ad77873e5' -H 'X-Correlation-ID: attacker-chosen' "https://example.test:${port}/")"
[[ "$body" == "website" ]]
request_id="$(awk 'BEGIN{IGNORECASE=1} /^x-request-id:/ {print $2}' "$headers" | tr -d '\r')"
correlation_id="$(awk 'BEGIN{IGNORECASE=1} /^x-correlation-id:/ {print $2}' "$headers" | tr -d '\r')"
[[ "$request_id" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]
[[ "$request_id" == "$correlation_id" && "$request_id" != "019f6abb-8dd5-7581-8449-2b9ad77873e5" ]]
status="$(curl --silent --output /dev/null --write-out '%{http_code}' --insecure --resolve "example.test:${port}:127.0.0.1" -H 'Host: example.test' "https://example.test:${port}/api/private")"
[[ "$status" == "503" ]]
echo "Envoy gateway E2E passed"
