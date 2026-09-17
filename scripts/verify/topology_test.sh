#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="${TOPOLOGY_COMPOSE_FILE:-${repo_root}/deploy/compose.production.yml}"
env_file="${TOPOLOGY_ENV_FILE:-${repo_root}/deploy/env.production.example}"
rendered="$(mktemp)"
insecure="$(mktemp)"

cleanup() {
  rm -f "${rendered}" "${insecure}"
}
trap cleanup EXIT

docker compose \
  --env-file "${env_file}" \
  --file "${compose_file}" \
  config --format json >"${rendered}"

TOPOLOGY_CONFIG_JSON="${rendered}" "${repo_root}/scripts/verify/topology.sh"

python3 - "${rendered}" "${insecure}" <<'PY'
import json
import sys

source, destination = sys.argv[1:]
with open(source, encoding="utf-8") as handle:
    config = json.load(handle)
config["services"]["api"].setdefault("volumes", []).append(
    {
        "type": "bind",
        "source": "/var/run/docker.sock",
        "target": "/var/run/docker.sock",
        "bind": {},
    }
)
with open(destination, "w", encoding="utf-8") as handle:
    json.dump(config, handle)
PY

if TOPOLOGY_CONFIG_JSON="${insecure}" "${repo_root}/scripts/verify/topology.sh" >/dev/null 2>&1; then
  echo "topology test: verifier accepted an API Docker socket" >&2
  exit 1
fi

echo "topology verifier self-test passed"
