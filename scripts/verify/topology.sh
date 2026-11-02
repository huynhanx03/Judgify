#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="${TOPOLOGY_COMPOSE_FILE:-${repo_root}/deploy/compose.production.yml}"
env_file="${TOPOLOGY_ENV_FILE:-${repo_root}/deploy/env.production.example}"
runtime_config="${TOPOLOGY_RUNTIME_CONFIG:-${repo_root}/deploy/config.production.example.yaml}"
config_json="${TOPOLOGY_CONFIG_JSON:-}"
temporary_config=""

cleanup() {
  if [[ -n "${temporary_config}" ]]; then
    rm -f "${temporary_config}"
  fi
}
trap cleanup EXIT

if [[ -z "${config_json}" ]]; then
  command -v docker >/dev/null 2>&1 || {
    echo "topology: docker is required" >&2
    exit 2
  }
  docker compose version >/dev/null 2>&1 || {
    echo "topology: the Docker Compose plugin is required" >&2
    exit 2
  }
  [[ -f "${compose_file}" ]] || {
    echo "topology: compose file not found: ${compose_file}" >&2
    exit 2
  }
  [[ -f "${env_file}" ]] || {
    echo "topology: environment file not found: ${env_file}" >&2
    exit 2
  }

  temporary_config="$(mktemp)"
  docker compose \
    --env-file "${env_file}" \
    --file "${compose_file}" \
    config --format json >"${temporary_config}"
  config_json="${temporary_config}"
fi

[[ -f "${config_json}" ]] || {
  echo "topology: rendered config not found: ${config_json}" >&2
  exit 2
}
[[ -f "${runtime_config}" ]] || {
  echo "topology: runtime configuration not found: ${runtime_config}" >&2
  exit 2
}
grep -Eq '^[[:space:]]*api_probe_address:[[:space:]]*"(0[.]0[.]0[.]0|127[.]0[.]0[.]1|localhost):[0-9]+"[[:space:]]*$' "${runtime_config}" || {
  echo "topology: production API probe must bind a private address" >&2
  exit 1
}
grep -Eq '^[[:space:]]*worker_probe_address:[[:space:]]*"(0[.]0[.]0[.]0|127[.]0[.]0[.]1|localhost):[0-9]+"[[:space:]]*$' "${runtime_config}" || {
  echo "topology: production worker probe must bind a private address" >&2
  exit 1
}

python3 - "${config_json}" <<'PY'
import json
import re
import sys
from pathlib import Path


config_path = Path(sys.argv[1])
with config_path.open(encoding="utf-8") as handle:
    config = json.load(handle)

services = config.get("services", {})
errors: list[str] = []


def check(condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)


def service(name: str) -> dict:
    value = services.get(name)
    check(isinstance(value, dict), f"missing required service: {name}")
    return value if isinstance(value, dict) else {}


def environment(item: dict) -> dict[str, str]:
    raw = item.get("environment", {})
    if isinstance(raw, dict):
        return {str(key): "" if value is None else str(value) for key, value in raw.items()}
    result: dict[str, str] = {}
    for entry in raw if isinstance(raw, list) else []:
        key, _, value = str(entry).partition("=")
        result[key] = value
    return result


def mounts(item: dict) -> list[dict]:
    result: list[dict] = []
    for raw in item.get("volumes", []) or []:
        if isinstance(raw, dict):
            result.append(raw)
            continue
        parts = str(raw).split(":")
        result.append(
            {
                "type": "bind" if parts and parts[0].startswith("/") else "volume",
                "source": parts[0] if parts else "",
                "target": parts[1] if len(parts) > 1 else "",
                "read_only": len(parts) > 2 and parts[2] == "ro",
            }
        )
    return result


def secret_targets(item: dict) -> set[str]:
    targets: set[str] = set()
    for raw in item.get("secrets", []) or []:
        if isinstance(raw, dict):
            target = str(raw.get("target") or raw.get("source") or "")
            targets.add(target.removeprefix("/run/secrets/"))
        else:
            targets.add(str(raw).removeprefix("/run/secrets/"))
    return targets


def depends_condition(item: dict, dependency: str) -> str:
    raw = item.get("depends_on", {})
    if isinstance(raw, dict):
        value = raw.get(dependency, {})
        if isinstance(value, dict):
            return str(value.get("condition", ""))
        if value is not None:
            return "service_started"
    return ""


def is_non_root(item: dict) -> bool:
    user = str(item.get("user", "")).strip().lower()
    return bool(user) and user not in {"0", "0:0", "root", "root:root"} and not user.startswith("0:")


def has_no_new_privileges(item: dict) -> bool:
    return any(str(value).lower().startswith("no-new-privileges") for value in item.get("security_opt", []) or [])


def has_all_caps_dropped(item: dict) -> bool:
    return "ALL" in {str(value).upper() for value in item.get("cap_drop", []) or []}


def has_limits(item: dict) -> bool:
    limits = item.get("deploy", {}).get("resources", {}).get("limits", {})
    return (
        bool(limits.get("cpus"))
        and bool(limits.get("memory"))
        and bool(limits.get("pids") or item.get("pids_limit"))
    )


def digest_pinned(image: object) -> bool:
    return bool(re.search(r"@sha256:[0-9a-f]{64}$", str(image or "")))


def command_is(item: dict, expected: str) -> bool:
    command = item.get("command", [])
    if isinstance(command, str):
        return command.strip() == expected
    return command == [expected]


# Compose normalises durations to Go duration strings, which may carry several
# units ("1m30s"). An unparsable value must read as zero so the caller fails closed.
DURATION_UNITS = {"ns": 1e-9, "us": 1e-6, "µs": 1e-6, "ms": 0.001, "s": 1, "m": 60, "h": 3600}


def seconds(value: object) -> float:
    text = str(value or "")
    parts = re.findall(r"([0-9]+(?:[.][0-9]+)?)(ns|us|µs|ms|s|m|h)", text)
    if not parts or "".join(amount + unit for amount, unit in parts) != text:
        return 0
    return sum(float(amount) * DURATION_UNITS[unit] for amount, unit in parts)


required = {"postgres", "migrate", "api", "worker", "website", "ingress"}
check(required.issubset(services), "production topology must include postgres, migrate, api, worker, website, and ingress")

postgres = service("postgres")
migrate = service("migrate")
api = service("api")
worker = service("worker")
website = service("website")
ingress = service("ingress")

for name, item in services.items():
    check(digest_pinned(item.get("image")), f"{name}: image must be pinned by sha256 digest")
    if name != "ingress":
        published = item.get("ports", []) or []
        allowed_probe = name in {"api", "worker"} and all(
            isinstance(port, dict)
            and str(port.get("host_ip", "")) == "127.0.0.1"
            and int(port.get("target", 0)) in ({"api": {9090}, "worker": {9091}}[name])
            for port in published
        )
        check(not published or allowed_probe, f"{name}: only loopback private probe ports may be published")

check(bool(ingress.get("ports")), "ingress: an HTTPS host port must be published")
check("envoy" in str(ingress.get("image", "")).lower(), "ingress: Envoy must be the public gateway")
ingress_configs = ingress.get("configs", []) or []
check(
    any(
        isinstance(config, dict)
        and str(config.get("target", "")) == "/etc/envoy/envoy.yaml"
        for config in ingress_configs
    ),
    "ingress: rendered Envoy config must be mounted read-only",
)
ingress_ports = ingress.get("ports", []) or []
check(
    len(ingress_ports) == 1
    and isinstance(ingress_ports[0], dict)
    and int(ingress_ports[0].get("target", 0)) == 8443,
    "ingress: exactly one TLS port targeting 8443 is allowed",
)
check(not postgres.get("ports"), "postgres: must not publish a host port")
check("backend" in set(postgres.get("networks", []) or []), "postgres: must join the private backend network")
check("edge" not in set(postgres.get("networks", []) or []), "postgres: must not join the edge network")

database_mounts = [mount for mount in mounts(postgres) if mount.get("target") == "/var/lib/postgresql/data"]
check(
    len(database_mounts) == 1 and database_mounts[0].get("type") == "volume",
    "postgres: data directory must use one durable named volume",
)
check(bool(postgres.get("healthcheck")), "postgres: healthcheck is required")

for name, item in (("migrate", migrate), ("api", api), ("worker", worker), ("website", website), ("ingress", ingress)):
    check(is_non_root(item), f"{name}: explicit non-root user is required")
    check(item.get("read_only") is True, f"{name}: root filesystem must be read-only")
    check(has_no_new_privileges(item), f"{name}: no-new-privileges is required")
    check(has_all_caps_dropped(item), f"{name}: all Linux capabilities must be dropped")
    check(not item.get("privileged", False), f"{name}: privileged mode is forbidden")
    check(not item.get("devices"), f"{name}: host devices are forbidden")
    check(not item.get("cap_add"), f"{name}: adding Linux capabilities is forbidden")
    check(item.get("network_mode") != "host", f"{name}: host networking is forbidden")
    check(item.get("pid") != "host", f"{name}: host PID namespace is forbidden")
    check(item.get("ipc") != "host", f"{name}: host IPC namespace is forbidden")
    if name != "migrate":
        check(bool(item.get("healthcheck")), f"{name}: healthcheck is required")
    check(has_limits(item), f"{name}: CPU, memory, and PID limits are required")
    check(item.get("init") is True, f"{name}: init process is required for signal and child reaping")

check(command_is(migrate, "migrate"), "migrate: command must select only the migrate role")
check(command_is(api, "api"), "api: command must select only the API role")
check(command_is(worker, "judge-worker"), "worker: command must select only the judge-worker role")
check(seconds(api.get("stop_grace_period")) >= 45, "api: shutdown grace must exceed the application drain budget")
check(seconds(worker.get("stop_grace_period")) >= 90, "worker: shutdown grace must cover lease-aware draining")
check(not api.get("expose"), "api: private probe port must not be advertised to networks")
check(not worker.get("expose"), "worker: private probe port must not be advertised to networks")

api_environment = environment(api)
worker_environment = environment(worker)
migrate_environment = environment(migrate)
api_mounts = mounts(api)
check(
    not any("docker.sock" in str(mount.get("source", "")) or "docker.sock" in str(mount.get("target", "")) for mount in api_mounts),
    "api: Docker socket access is forbidden",
)
check(not any("DOCKER" in key.upper() for key in api_environment), "api: Docker environment access is forbidden")

worker_socket_mounts = [mount for mount in mounts(worker) if mount.get("target") == "/run/docker.sock"]
check(len(worker_socket_mounts) == 1, "worker: exactly one Docker socket must be mounted at /run/docker.sock")
if worker_socket_mounts:
    socket_source = str(worker_socket_mounts[0].get("source", ""))
    rootless_socket = re.fullmatch(r"/run/user/([1-9][0-9]*)/docker\.sock", socket_source)
    check(rootless_socket is not None, "worker: socket must use /run/user/<non-root-uid>/docker.sock")
    if rootless_socket is not None:
        worker_uid = str(worker.get("user", "")).split(":", 1)[0]
        check(worker_uid == rootless_socket.group(1), "worker: container UID must own the rootless Docker socket")
    check(worker_socket_mounts[0].get("read_only") is not True, "worker: Docker socket must be writable")
check(environment(worker).get("DOCKER_HOST") == "unix:///run/docker.sock", "worker: DOCKER_HOST must use the isolated socket mount")
check(digest_pinned(environment(worker).get("JUDGE_DOCKER_IMAGE")), "worker: sandbox image must be pinned by sha256 digest")
check("backend" in set(worker.get("networks", []) or []), "worker: must join the private backend network")
check("edge" not in set(worker.get("networks", []) or []), "worker: must not join the edge network")

database_users = {
    api_environment.get("DATABASE_USERNAME"),
    worker_environment.get("DATABASE_USERNAME"),
    migrate_environment.get("DATABASE_USERNAME"),
}
check(None not in database_users and "" not in database_users and len(database_users) == 3, "api, worker, and migrator must use distinct database roles")

password_files = {
    api_environment.get("DATABASE_PASSWORD_FILE"),
    worker_environment.get("DATABASE_PASSWORD_FILE"),
    migrate_environment.get("DATABASE_PASSWORD_FILE"),
}
check(None not in password_files and "" not in password_files and len(password_files) == 3, "api, worker, and migrator must use distinct database secrets")

for name, item, env in (
    ("api", api, api_environment),
    ("worker", worker, worker_environment),
    ("migrate", migrate, migrate_environment),
):
    password_file = env.get("DATABASE_PASSWORD_FILE", "")
    check(password_file.startswith("/run/secrets/"), f"{name}: database password must be loaded from a Compose secret")
    check(password_file.removeprefix("/run/secrets/") in secret_targets(item), f"{name}: referenced database secret must be mounted")
    check("DATABASE_PASSWORD" not in env, f"{name}: direct database password environment is forbidden")

for direct_secret in ("JWT_SECRET", "GOOGLE_CLIENT_SECRET", "GITHUB_CLIENT_SECRET", "RESEND_API_KEY"):
    check(direct_secret not in api_environment, f"api: direct {direct_secret} environment is forbidden")
    file_key = f"{direct_secret}_FILE"
    check(api_environment.get(file_key, "").startswith("/run/secrets/"), f"api: {file_key} must use a mounted secret")

check(depends_condition(migrate, "postgres") == "service_healthy", "migrate: must wait for healthy postgres")
check(depends_condition(api, "migrate") == "service_completed_successfully", "api: must wait for successful migrations")
check(depends_condition(worker, "migrate") == "service_completed_successfully", "worker: must wait for successful migrations")
check(depends_condition(ingress, "api") == "service_healthy", "ingress: must wait for healthy api")
check(depends_condition(ingress, "website") == "service_healthy", "ingress: must wait for healthy website")
check(
    {"jwt_secret", "jwt_private_key", "jwt_public_key"}.issubset(secret_targets(api)),
    "api: JWT secret and RSA key pair must be mounted as secrets",
)
check(
    {"tls_certificate", "tls_private_key"}.issubset(secret_targets(ingress)),
    "ingress: TLS certificate and private key must be mounted as secrets",
)

networks = config.get("networks", {})
check(networks.get("backend", {}).get("internal") is True, "backend network must be internal")
check("postgres_data" in (config.get("volumes", {}) or {}), "postgres_data named volume must be declared")

if errors:
    print("production topology verification failed:", file=sys.stderr)
    for error in errors:
        print(f"  - {error}", file=sys.stderr)
    raise SystemExit(1)

print("production topology verification passed")
PY
