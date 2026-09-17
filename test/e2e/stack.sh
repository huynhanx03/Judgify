#!/usr/bin/env bash
# test/e2e/stack.sh — disposable, idempotent E2E stack for Judgify Playwright tests.
#
# Subcommands:
#   up      start postgres, build binaries, migrate, bootstrap, seed, start API
#   down    stop API, remove postgres container
#   env     print the path to the sourced env file
#   status  print whether the stack components are running
#
# Usage:
#   bash test/e2e/stack.sh up
#   source "$(bash test/e2e/stack.sh env)"
#   bash test/e2e/stack.sh down
#
# Security: passwords and tokens are kept in a 600-mode file under the temp dir.
# They are never echoed to stdout or stderr.

set -Eeuo pipefail

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
. "$root_dir/scripts/lib/operations.sh"
api_dir="$root_dir/api"

# State lives in a stable (non-random) subdirectory so that `env` and `status`
# can find it across separate invocations. The directory is owned only by the
# current user (umask 077).
state_root="$(ops_temp_root)/judgify-e2e-stack-$(id -u)"

# All generated secrets and IDs are written here.
env_file="$state_root/e2e.env"

# PIDs for the API server, Next.js server, and reverse proxy.
pid_file="$state_root/api.pid"
next_pid_file="$state_root/next.pid"
proxy_pid_file="$state_root/proxy.pid"

# Container name is stable so `down` can remove it without consulting a file.
container_name="judgify-e2e-stack-$(id -u)"

# Log files.
api_log="$state_root/api.log"
next_log="$state_root/next.log"
proxy_log="$state_root/proxy.log"

# ---------------------------------------------------------------------------
# Helper: fail
# ---------------------------------------------------------------------------
fail() {
  printf 'e2e-stack: %s\n' "$*" >&2
  exit 1
}

# ---------------------------------------------------------------------------
# Helper: log (never logs secret values — callers must never pass them)
# ---------------------------------------------------------------------------
log() {
  printf 'e2e-stack: %s\n' "$*" >&2
}

# ---------------------------------------------------------------------------
# Helper: read value from env file without printing it
# ---------------------------------------------------------------------------
read_env_value() {
  local file="$1" key="$2" value=""
  [ -f "$file" ] || return 1
  value="$(awk -F= -v k="$key" '$1==k{sub(/^[^=]*=/,""); sub(/\r$/,""); print; exit}' "$file")"
  [ -n "$value" ] || return 1
  printf '%s' "$value"
}

# ---------------------------------------------------------------------------
# Helper: write a key=value pair to the env file (never pass secret as arg)
# ---------------------------------------------------------------------------
write_env() {
  local key="$1" value="$2"
  # Remove any existing line for this key, then append.
  local tmp
  tmp="$(mktemp "$state_root/.env.XXXXXX")"
  chmod 600 "$tmp"
  if [ -f "$env_file" ]; then
    awk -F= -v k="$key" '$1!=k{print}' "$env_file" >>"$tmp"
  fi
  printf '%s=%s\n' "$key" "$value" >>"$tmp"
  mv "$tmp" "$env_file"
}

# ---------------------------------------------------------------------------
# Helper: uuid v4 using /dev/urandom (no uuidgen dependency)
# ---------------------------------------------------------------------------
gen_uuid() {
  local hex
  hex="$(od -A n -N 16 -t x1 /dev/urandom | tr -d ' \n')"
  printf '%s-%s-%s-%s-%s\n' \
    "${hex:0:8}" "${hex:8:4}" \
    "4${hex:13:3}" \
    "$(printf '%x' "$(( (0x${hex:16:2} & 0x3f) | 0x80 ))")${hex:18:2}" \
    "${hex:20:12}"
}

# ---------------------------------------------------------------------------
# Helper: generate a simple alphanumeric password (avoids special chars that
# confuse shell quoting in env files). $1 = byte count passed to openssl rand.
# ---------------------------------------------------------------------------
gen_password() {
  openssl rand -base64 "${1:-18}" | tr -dc 'A-Za-z0-9' | head -c 24
}

# ---------------------------------------------------------------------------
# Helper: HTTP request via curl. Returns the response body.
# Exits on non-zero curl exit code; the caller decides whether HTTP status is ok.
# Never prints the request body (which may contain passwords).
# ---------------------------------------------------------------------------
api_curl() {
  local method="$1" url="$2" extra_headers="$3" body_file="$4" out_file="$5"
  local status_code
  local base_url
  base_url="$(read_env_value "$env_file" "STACK_API_BASE_URL" 2>/dev/null)" || base_url="http://127.0.0.1:8000"

  local curl_args=()
  curl_args+=( --silent --show-error --fail-with-body )
  curl_args+=( --max-time 30 )
  curl_args+=( -X "$method" )
  curl_args+=( -H "Content-Type: application/json" )
  curl_args+=( -H "Origin: http://localhost:3000" )

  # Cookie jar
  local cookie_jar="${5%.*}.cookies" 2>/dev/null || cookie_jar="$state_root/cookies.jar"

  if [ -n "$extra_headers" ]; then
    # extra_headers is newline-delimited "Name: Value" pairs
    while IFS= read -r hdr; do
      [ -n "$hdr" ] && curl_args+=( -H "$hdr" )
    done <<<"$extra_headers"
  fi

  curl_args+=( --cookie "$state_root/cookies.jar" )
  curl_args+=( --cookie-jar "$state_root/cookies.jar" )

  if [ -n "$body_file" ] && [ -f "$body_file" ]; then
    curl_args+=( --data-binary "@$body_file" )
  fi

  curl_args+=( -o "$out_file" )
  curl_args+=( -w '%{http_code}' )

  local status_tmp
  status_tmp="$(mktemp "$state_root/.http-status.XXXXXX")"
  local err_code=0
  curl "${curl_args[@]}" "$base_url$url" >"$status_tmp" 2>&1 || err_code=$?
  status_code="$(cat "$status_tmp")"
  rm -f "$status_tmp"

  if [ "$err_code" -ne 0 ] && [ -z "$status_code" ]; then
    log "HTTP request failed: $method $url"
    cat "$out_file" >&2 2>/dev/null || true
    return 1
  fi
  printf '%s' "$status_code"
}

# ---------------------------------------------------------------------------
# Simpler, direct curl wrapper that returns body+status cleanly.
# Usage: http_call METHOD PATH_OR_URL HEADERS_FILE BODY_FILE OUT_FILE
# Returns the HTTP status code on stdout; response body goes to OUT_FILE.
# ---------------------------------------------------------------------------
http_call() {
  local method="$1"
  local path="$2"
  local hdr_file="${3:-}"  # file with "Header: Value" lines, one per line
  local body_file="${4:-}"
  local out_file="${5:-$state_root/response.json}"

  local base_url
  base_url="$(read_env_value "$env_file" "STACK_API_BASE_URL" 2>/dev/null)" || base_url="http://127.0.0.1:8000"

  local curl_args=()
  curl_args+=( --silent --show-error )
  curl_args+=( --max-time 30 )
  curl_args+=( -X "$method" )
  curl_args+=( -H "Content-Type: application/json" )
  curl_args+=( -H "Origin: http://localhost:3000" )
  curl_args+=( --cookie "$state_root/cookies.jar" )
  curl_args+=( --cookie-jar "$state_root/cookies.jar" )

  if [ -n "$hdr_file" ] && [ -f "$hdr_file" ]; then
    while IFS= read -r hdr; do
      [ -n "$hdr" ] && curl_args+=( -H "$hdr" )
    done <"$hdr_file"
  fi

  if [ -n "$body_file" ] && [ -f "$body_file" ]; then
    curl_args+=( --data-binary "@$body_file" )
  fi

  curl_args+=( -o "$out_file" )
  curl_args+=( -w '%{http_code}' )

  # Seeding drives far more requests per minute than a human operator ever
  # would, so it meets the production rate limiter. Back off and retry the way a
  # real client must, rather than raising the limit and testing a weaker server.
  local status=""
  local attempt=1
  local delay=2
  while :; do
    status="$(curl "${curl_args[@]}" "$base_url$path" 2>&1)" || {
      log "curl error for $method $path"
      cat "$out_file" >&2 2>/dev/null || true
      return 1
    }
    if [ "$status" != "429" ] || [ "$attempt" -ge 6 ]; then
      break
    fi
    log "rate limited on $method $path; waiting ${delay}s"
    sleep "$delay"
    delay=$((delay * 2))
    attempt=$((attempt + 1))
  done
  printf '%s' "$status"
}

# ---------------------------------------------------------------------------
# Read a JSON field from a file using awk (no jq dependency).
# Usage: json_field FILE FIELD
# Handles simple string and number values one level deep.
# ---------------------------------------------------------------------------
# The API answers creates with its success envelope on HTTP 200; a few routes
# answer 201. Accept both so the harness follows the transport contract rather
# than one route's status choice.
is_created() {
  [ "$1" = "200" ] || [ "$1" = "201" ]
}

json_field() {
  local file="$1" field="$2"
  awk -v f="$field" '
    BEGIN { FS="\"" }
    {
      n = split($0, parts, "\"" f "\"")
      if (n >= 2) {
        # Take the part after the field name
        rest = parts[2]
        # Strip leading : and whitespace
        gsub(/^[[:space:]]*:[[:space:]]*/, "", rest)
        # If starts with quote, extract string value
        if (substr(rest,1,1) == "\"") {
          n2 = split(rest, sp, "\"")
          print sp[2]
          exit
        } else {
          # numeric or boolean
          match(rest, /^[0-9a-zA-Z._-]+/)
          print substr(rest, RSTART, RLENGTH)
          exit
        }
      }
    }
  ' "$file"
}

# ---------------------------------------------------------------------------
# Require command
# ---------------------------------------------------------------------------
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command is unavailable: $1"
}

# ---------------------------------------------------------------------------
# Find free ephemeral ports so a stale process can never block a new run.
# Strategy: ask the OS to bind port 0, capture the assigned port, then release
# it. On macOS bash 3.2 we do this via Python or a raw TCP call.
#
# Defined at file scope, not inside cmd_up: a nested definition only exists once
# its enclosing function has run, so `judge` on a stack that `up` started in an
# earlier process would not see it.
# ---------------------------------------------------------------------------
find_free_port() {
  local p
  p="$(python3 -c "
import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()
" 2>/dev/null)" && printf '%s' "$p" && return 0
  # Fallback: pick a random port in the ephemeral range and hope it is free.
  p=$(( 49152 + ($(od -A n -N 2 -t u2 /dev/urandom | tr -d ' ') % 16383) ))
  printf '%s' "$p"
}

# ===========================================================================
# SUBCOMMAND: up
# ===========================================================================
cmd_up() {
  require_cmd docker
  require_cmd openssl
  require_cmd curl
  require_cmd awk

  umask 077
  mkdir -p "$state_root"
  chmod 700 "$state_root"

  # Reset cookie jar
  : >"$state_root/cookies.jar"
  chmod 600 "$state_root/cookies.jar"

  # ------------------------------------------------------------------
  # 1. Generate secrets if not already present.
  # ------------------------------------------------------------------
  log "preparing secrets"

  local db_admin_password api_password worker_password migrator_password
  local jwt_secret e2e_admin_password e2e_user_password
  local api_port probe_port website_port proxy_port

  # Guard: refuse a second `up` if the server binary is already running from
  # this state_root.  A stale pid file from a killed run is handled gracefully.
  local server_bin="$state_root/judgify-server"
  if [ -f "$pid_file" ]; then
    local existing_pid
    existing_pid="$(cat "$pid_file" 2>/dev/null)" || existing_pid=""
    if [ -n "$existing_pid" ] && kill -0 "$existing_pid" 2>/dev/null; then
      fail "stack is already running (pid $existing_pid). Run 'down' first."
    fi
    rm -f "$pid_file"
  fi

  # Passwords — generate fresh on each `up` (the container is also fresh).
  db_admin_password="$(gen_password 18)"
  migrator_password="$(gen_password 18)"
  api_password="$(gen_password 18)"
  worker_password="$(gen_password 18)"
  jwt_secret="$(openssl rand -hex 32)"
  e2e_admin_password="$(gen_password 18)"
  e2e_user_password="$(gen_password 18)"

  api_port="$(find_free_port)"
  probe_port="$(find_free_port)"
  website_port="$(find_free_port)"
  proxy_port="$(find_free_port)"
  # Ensure all four ports differ.
  while [ "$probe_port" = "$api_port" ]; do
    probe_port="$(find_free_port)"
  done
  while [ "$website_port" = "$api_port" ] || [ "$website_port" = "$probe_port" ]; do
    website_port="$(find_free_port)"
  done
  while [ "$proxy_port" = "$api_port" ] || [ "$proxy_port" = "$probe_port" ] || [ "$proxy_port" = "$website_port" ]; do
    proxy_port="$(find_free_port)"
  done

  # Write known, non-secret values first.
  write_env "STACK_API_BASE_URL" "http://127.0.0.1:${api_port}"
  write_env "STACK_PROBE_URL" "http://127.0.0.1:${probe_port}"
  write_env "STACK_CONTAINER" "$container_name"
  write_env "STACK_WEBSITE_PORT" "$website_port"
  write_env "STACK_PROXY_PORT" "$proxy_port"

  # ------------------------------------------------------------------
  # 2. Start a throwaway PostgreSQL 16 container.
  # ------------------------------------------------------------------
  log "starting throwaway PostgreSQL 16 container: $container_name"

  # If a stale container from a previous run exists, remove it.
  docker rm -f "$container_name" >/dev/null 2>&1 || true

  docker run \
    --pull never \
    --detach \
    --name "$container_name" \
    --env "POSTGRES_USER=postgres" \
    --env "POSTGRES_PASSWORD=$db_admin_password" \
    --env "POSTGRES_DB=judgify" \
    --publish "127.0.0.1::5432" \
    postgres:16 >/dev/null

  log "waiting for PostgreSQL to be ready"
  local ready=false
  local i=1
  while [ "$i" -le 60 ]; do
    if docker logs "$container_name" 2>&1 | grep -Fq 'PostgreSQL init process complete; ready for start up.' &&
       docker exec "$container_name" pg_isready \
         --username "postgres" \
         --dbname "judgify" >/dev/null 2>&1; then
      ready=true
      break
    fi
    sleep 1
    i=$((i + 1))
  done
  if [ "$ready" != "true" ]; then
    fail "throwaway PostgreSQL did not become ready within 60 seconds"
  fi

  local host_port
  host_port="$(docker port "$container_name" 5432/tcp | awk -F: 'NR==1{print $NF}')"
  if ! printf '%s' "$host_port" | grep -qE '^[0-9]+$'; then
    fail "could not resolve PostgreSQL published port"
  fi
  write_env "STACK_DB_PORT" "$host_port"
  # Write api_password to the env file so seed_fixtures can use it for the
  # contest publish SQL fallback (the API publish call fails with PostgreSQL 16
  # due to 42P08 type inference regression: text vs character varying).
  write_env "STACK_API_PASSWORD" "$api_password"
  log "PostgreSQL listening on 127.0.0.1:$host_port"

  # ------------------------------------------------------------------
  # 3. Bootstrap roles from deploy/postgres/init-roles.sql.
  # ------------------------------------------------------------------
  log "bootstrapping database roles"
  docker exec \
    --interactive \
    --env "POSTGRES_DB=judgify" \
    --env "JUDGIFY_MIGRATOR_PASSWORD=$migrator_password" \
    --env "JUDGIFY_API_PASSWORD=$api_password" \
    --env "JUDGIFY_WORKER_PASSWORD=$worker_password" \
    "$container_name" \
    psql \
      --no-psqlrc \
      --set ON_ERROR_STOP=1 \
      --username "postgres" \
      --dbname "judgify" \
      <"$root_dir/deploy/postgres/init-roles.sql" >/dev/null

  # ------------------------------------------------------------------
  # 4. Build binaries: cmd/migrate and cmd/bootstrap-admin and cmd/server.
  # ------------------------------------------------------------------
  log "building cmd/migrate, cmd/bootstrap-admin, and cmd/server"

  local migrate_bin="$state_root/judgify-migrate"
  local bootstrap_bin="$state_root/judgify-bootstrap-admin"
  # server_bin already declared above for the double-up guard.

  (
    cd "$api_dir"
    env GOPROXY=off go build -trimpath -o "$migrate_bin" ./cmd/migrate
    env GOPROXY=off go build -trimpath -o "$bootstrap_bin" ./cmd/bootstrap-admin
    env GOPROXY=off go build -trimpath -o "$server_bin" ./cmd/server
  )

  # ------------------------------------------------------------------
  # 5. Run the production migrator.
  # ------------------------------------------------------------------
  log "applying database migrations"
  local migrate_log="$state_root/migrate.log"
  (
    cd "$api_dir"
    env \
      GO_ENV=local \
      DATABASE_DRIVER=postgres \
      DATABASE_HOST=127.0.0.1 \
      DATABASE_PORT="$host_port" \
      DATABASE_USERNAME=judgify_migrator \
      DATABASE_PASSWORD="$migrator_password" \
      DATABASE_DATABASE=judgify \
      LOGGER_FILE_LOG_NAME="$migrate_log" \
      "$migrate_bin"
  )
  log "migrations applied"

  # ------------------------------------------------------------------
  # 6. Build a private API root under state_root so the server never
  #    touches any file in the repository working tree.
  #
  #    The API binary (cmd/server) resolves paths relative to its working
  #    directory (os.Getwd() at startup):
  #      config/<GO_ENV>.yaml          <- loaded by Viper
  #      ./certs/private_key.pem       <- jwt.private_key_path
  #      ./certs/public_key.pem        <- jwt.public_key_path
  #      ./storages/logs/app.log       <- logger.file_log_name
  #      storages/forge                <- forge.data_dir
  #
  #    We build that tree here and launch the server with cwd=apiroot.
  #    No file under api/ is ever created or modified.
  # ------------------------------------------------------------------
  log "building private API root and generating JWT keypair"

  local apiroot="$state_root/apiroot"
  mkdir -p "$apiroot/config" "$apiroot/certs" "$apiroot/storages/logs" "$apiroot/storages/forge"
  chmod 700 "$apiroot" "$apiroot/certs"

  # --- JWT keypair (stays in state_root, never written to api/certs/) ---
  local private_key="$apiroot/certs/private_key.pem"
  local public_key="$apiroot/certs/public_key.pem"

  log "generating RSA JWT keypair"
  local priv_tmp pub_tmp
  priv_tmp="$(mktemp "$apiroot/certs/.private-key.XXXXXX")"
  pub_tmp="$(mktemp "$apiroot/certs/.public-key.XXXXXX")"
  openssl genpkey -algorithm RSA \
    -pkeyopt rsa_keygen_bits:3072 \
    -out "$priv_tmp" >/dev/null 2>&1
  openssl pkey -in "$priv_tmp" -pubout -out "$pub_tmp" >/dev/null 2>&1
  chmod 600 "$priv_tmp"
  chmod 644 "$pub_tmp"
  mv "$priv_tmp" "$private_key"
  mv "$pub_tmp" "$public_key"

  # --- Copy and patch config/local.yaml into the private root ---
  # readBoundedConfig requires: regular file, not world/group-writable,
  # 1 <= size <= 1MB.  Mode 0644 satisfies this.
  #
  # We patch two things that cannot be overridden via env vars:
  #   1. authentication.browser_origins — YAML array, no AutomaticEnv path
  #   2. observability.api_probe_address — patch to our ephemeral probe port
  #      (this IS bound via AutomaticEnv as OBSERVABILITY_API_PROBE_ADDRESS,
  #       so we rely on env var override instead of patching the file)
  #
  # browser_origins MUST include "http://localhost:3000" for CORS to pass.
  # We replace the existing origins list in the copied file.
  local src_config="$api_dir/config/local.yaml"
  local dst_config="$apiroot/config/local.yaml"

  # Copy and strip world/group write bits so readBoundedConfig accepts it.
  cp "$src_config" "$dst_config"
  chmod 644 "$dst_config"

  # Patch browser_origins to list the proxy origin (http://127.0.0.1:PROXY_PORT)
  # plus http://localhost:3000 (used by the harness seeding curl calls as Origin).
  # The proxy serves both the Next.js app and /api/* on a single origin so that
  # the browser's same-origin cookie + CSRF model works without a real nginx.
  # The yaml section looks like:
  #   browser_origins:
  #     - "http://127.0.0.1:<proxy_port>"
  #     - "http://localhost:3000"
  # We replace whatever origins are there with exactly these two entries.
  # Use awk to rewrite the section (bash 3.2, no perl/python dependency).
  local proxy_origin="http://127.0.0.1:${proxy_port}"
  local patched_config
  patched_config="$(awk -v origin="$proxy_origin" '
    /^[[:space:]]*browser_origins:/ {
      print
      print "    - \"" origin "\""
      print "    - \"http://localhost:3000\""
      in_origins=1
      next
    }
    in_origins && /^[[:space:]]*-/ { next }
    { in_origins=0; print }
  ' "$dst_config")"
  printf '%s\n' "$patched_config" >"$dst_config"
  chmod 644 "$dst_config"

  # Raise the request ceiling for this harness only. The browser matrix drives
  # 574 route/theme/viewport combinations from one loopback address, which is far
  # denser than any real client and trips the production ceiling. The limiter
  # stays enabled and bounded; its behaviour is covered by dedicated tests in
  # go-common (pkg/common/http/middlewares/rate_limit_test.go), so the control is
  # verified there rather than re-tested under synthetic load here.
  local paced_config
  paced_config="$(awk '
    /^[[:space:]]*rate_limit:/ { print; in_rate=1; next }
    in_rate && /^[[:space:]]*limit:/  { print "    limit: 100000"; next }
    in_rate && /^[[:space:]]*burst:/  { print "    burst: 20000"; next }
    in_rate && /^[[:space:]]*window:/ { print "    window: 60"; in_rate=0; next }
    { print }
  ' "$dst_config")"
  printf '%s\n' "$paced_config" >"$dst_config"
  chmod 644 "$dst_config"

  # ------------------------------------------------------------------
  # 7. Bootstrap the administrator account.
  # ------------------------------------------------------------------
  log "bootstrapping administrator account"
  local e2e_admin_username="e2eadmin"
  (
    cd "$api_dir"
    env \
      GO_ENV=local \
      DATABASE_DRIVER=postgres \
      DATABASE_HOST=127.0.0.1 \
      DATABASE_PORT="$host_port" \
      DATABASE_USERNAME=judgify_api \
      DATABASE_PASSWORD="$api_password" \
      DATABASE_DATABASE=judgify \
      JUDGIFY_BOOTSTRAP_ADMIN_USERNAME="$e2e_admin_username" \
      JUDGIFY_BOOTSTRAP_ADMIN_PASSWORD="$e2e_admin_password" \
      "$bootstrap_bin"
  )
  log "administrator account created"

  # ------------------------------------------------------------------
  # 8. Start the API server from the private apiroot.
  #    cwd = $apiroot so the binary resolves all relative paths there,
  #    never touching anything under api/.
  #    All secrets are passed as exported env vars; no .env file is used.
  # ------------------------------------------------------------------
  log "starting API server on port $api_port"

  # Per infrastructure/server.go:
  #   PORT env var overrides config server.port
  #   SERVER_HOST env var overrides config server.host
  # Per configuration/config.go AutomaticEnv + "." -> "_" replacer:
  #   OBSERVABILITY_API_PROBE_ADDRESS -> observability.api_probe_address

  (
    cd "$apiroot"
    env \
      GO_ENV=local \
      PORT="$api_port" \
      SERVER_HOST=127.0.0.1 \
      DATABASE_DRIVER=postgres \
      DATABASE_HOST=127.0.0.1 \
      DATABASE_PORT="$host_port" \
      DATABASE_USERNAME=judgify_api \
      DATABASE_PASSWORD="$api_password" \
      DATABASE_DATABASE=judgify \
      JWT_SECRET="$jwt_secret" \
      JUDGIFY_VERSION=e2e-stack \
      OBSERVABILITY_API_PROBE_ADDRESS=127.0.0.1:${probe_port} \
      GOOGLE_CLIENT_ID=local-google-client-id \
      GOOGLE_CLIENT_SECRET=local-google-client-secret \
      GITHUB_CLIENT_ID=local-github-client-id \
      GITHUB_CLIENT_SECRET=local-github-client-secret \
      RESEND_API_KEY=local-resend-placeholder \
      RESEND_FROM_EMAIL=noreply@localhost.invalid \
      RESEND_FROM_NAME=Judgify \
      RESEND_RESET_URL=http://localhost:3000/reset-password \
      RESEND_VERIFICATION_URL=http://localhost:3000/verify-recovery-contact \
      RESEND_INVITATION_URL=http://localhost:3000/accept-invitation \
      "$server_bin" >>"$api_log" 2>&1 &
    echo $!
  ) >"$pid_file"
  local api_pid
  api_pid="$(cat "$pid_file")"
  log "API server started (pid $api_pid)"

  # ------------------------------------------------------------------
  # 9. Poll the probe server until ready.
  # ------------------------------------------------------------------
  log "waiting for API to become ready (probe: http://127.0.0.1:${probe_port}/ready)"
  local ready_status=false
  local attempt=1
  while [ "$attempt" -le 90 ]; do
    local resp_code=""
    resp_code="$(curl --silent --max-time 2 -o /dev/null -w '%{http_code}' \
      "http://127.0.0.1:${probe_port}/ready" 2>/dev/null)" || true
    if [ "$resp_code" = "200" ]; then
      ready_status=true
      break
    fi
    # Check if the server died.
    if ! kill -0 "$api_pid" 2>/dev/null; then
      log "API server process died; last log lines:"
      tail -30 "$api_log" >&2 2>/dev/null || true
      fail "API server exited unexpectedly before becoming ready"
    fi
    sleep 2
    attempt=$((attempt + 1))
  done
  if [ "$ready_status" != "true" ]; then
    log "last probe log:"
    tail -30 "$api_log" >&2 2>/dev/null || true
    fail "API did not become ready within 180 seconds"
  fi
  log "API is ready"

  # ------------------------------------------------------------------
  # 10. Seed fixtures through the HTTP API.
  # ------------------------------------------------------------------
  log "seeding E2E fixtures via HTTP API"
  local e2e_user_username="e2emember"

  seed_fixtures \
    "$e2e_admin_username" \
    "$e2e_admin_password" \
    "$e2e_user_username" \
    "$e2e_user_password" \
    "$api_port"

  # ------------------------------------------------------------------
  # 11. Start Next.js production server on an ephemeral port.
  #     Requires `next build` to have been run in website/ first.
  #     NEXT_PUBLIC_API_URL=/api tells the app to call /api/* same-origin
  #     (the production default); the proxy forwards those to the API.
  # ------------------------------------------------------------------
  log "starting Next.js production server on port $website_port"
  require_cmd node

  local website_dir="$root_dir/website"
  local standalone_dir="$website_dir/.next/standalone"
  local standalone_server="$standalone_dir/server.js"
  if [ ! -f "$standalone_server" ]; then
    fail "$standalone_server does not exist — run 'cd website && npm run build' first (output: standalone requires .next/standalone/)"
  fi

  # The standalone server requires .next/static/ and public/ to be present
  # inside the standalone directory.  Next.js does NOT copy them automatically;
  # we must do it here.  Use -n (no-clobber) so a re-run is cheap.
  if [ ! -d "$standalone_dir/.next/static" ]; then
    log "copying .next/static into standalone directory"
    cp -rn "$website_dir/.next/static" "$standalone_dir/.next/static"
  fi
  if [ ! -d "$standalone_dir/public" ] && [ -d "$website_dir/public" ]; then
    log "copying public into standalone directory"
    cp -rn "$website_dir/public" "$standalone_dir/public"
  fi


  # Use the standalone server instead of 'next start' because next.config.ts
  # sets output: "standalone".  'next start' prints a warning and may not
  # serve SSR pages correctly in that mode.
  (
    cd "$website_dir"
    env \
      PORT="$website_port" \
      HOSTNAME=127.0.0.1 \
      NODE_ENV=production \
      NEXT_PUBLIC_API_URL=/api \
      node "$standalone_server" \
        >>"$next_log" 2>&1 &
    echo $!
  ) >"$next_pid_file"
  local next_pid
  next_pid="$(cat "$next_pid_file")"
  log "Next.js started (pid $next_pid)"

  # ------------------------------------------------------------------
  # 12. Write the Node.js reverse proxy script and start it.
  #     The proxy is the single entry point for Playwright tests:
  #       • GET/POST /api/* → forwarded to API (strip /api prefix)
  #       • GET /ws       → forwarded to API with WebSocket upgrade headers
  #       • everything else → forwarded to Next.js
  #     This gives the browser a single origin so that:
  #       • the secure, httpOnly session cookie is set/read on one origin
  #       • the CSRF Origin check compares the proxy origin to itself
  #     How this differs from the production nginx config:
  #       • no TLS (http only); production uses TLS on 8443
  #       • no Docker embedded DNS resolver; upstreams are loopback ports
  #       • no keepalive upstream pools; Node.js http.request per-call
  #       • no static /api/live|ready|ping|metrics block — the harness
  #         has a separate probe listener for those
  #     What this proves and does not prove:
  #       • PROVES: same-origin cookie/CSRF model, all product UI flows,
  #         WebSocket upgrade path, API correctness at the HTTP level
  #       • DOES NOT PROVE: the production nginx config is correct (that
  #         is scripts/verify/topology.sh's job, which already passes),
  #         TLS termination, or upstream health checks / DNS resolution
  # ------------------------------------------------------------------
  log "writing reverse proxy script"
  local proxy_script="$state_root/proxy.js"
  cat >"$proxy_script" <<PROXYEOF
'use strict';
// Harness same-origin reverse proxy for Judgify E2E tests.
// Forwards /api/* to the API server (stripping the /api prefix),
// forwards /ws to the API server with WebSocket upgrade headers,
// and forwards everything else to the Next.js server.
// Runs until SIGTERM or SIGINT.
const http = require('http');
const net  = require('net');

const PROXY_PORT   = parseInt(process.env.E2E_PROXY_PORT,   10);
const API_PORT     = parseInt(process.env.E2E_API_PORT,     10);
const WEBSITE_PORT = parseInt(process.env.E2E_WEBSITE_PORT, 10);
const API_HOST     = '127.0.0.1';
const WEBSITE_HOST = '127.0.0.1';

function forwardHttp(req, res, targetHost, targetPort, targetPath) {
  const opts = {
    hostname: targetHost,
    port:     targetPort,
    path:     targetPath,
    method:   req.method,
    headers:  Object.assign({}, req.headers, {
      host: targetHost + ':' + targetPort,
    }),
  };
  const proxy = http.request(opts, function(pRes) {
    res.writeHead(pRes.statusCode, pRes.headers);
    pRes.pipe(res, { end: true });
  });
  proxy.on('error', function(err) {
    if (!res.headersSent) {
      res.writeHead(502);
    }
    res.end('proxy error: ' + err.message);
  });
  req.pipe(proxy, { end: true });
}

const server = http.createServer(function(req, res) {
  const url = req.url || '/';
  if (url === '/api' || url.startsWith('/api/')) {
    // Strip /api prefix before forwarding to the API server.
    const apiPath = url.slice(4) || '/';
    forwardHttp(req, res, API_HOST, API_PORT, apiPath);
  } else {
    forwardHttp(req, res, WEBSITE_HOST, WEBSITE_PORT, url);
  }
});

// WebSocket: /ws → API server
server.on('upgrade', function(req, clientSocket, head) {
  const url = req.url || '/';
  let targetHost, targetPort;
  if (url === '/ws' || url.startsWith('/ws?')) {
    targetHost = API_HOST;
    targetPort = API_PORT;
  } else {
    targetHost = WEBSITE_HOST;
    targetPort = WEBSITE_PORT;
  }
  const conn = net.connect(targetPort, targetHost, function() {
    // Forward the HTTP upgrade request.
    const reqLine = req.method + ' ' + url + ' HTTP/1.1\r\n';
    let hdrs = reqLine;
    const raw = req.rawHeaders;
    for (let i = 0; i < raw.length; i += 2) {
      if (raw[i].toLowerCase() === 'host') {
        hdrs += 'Host: ' + targetHost + ':' + targetPort + '\r\n';
      } else {
        hdrs += raw[i] + ': ' + raw[i+1] + '\r\n';
      }
    }
    hdrs += '\r\n';
    conn.write(hdrs);
    if (head && head.length) conn.write(head);
    conn.pipe(clientSocket, { end: true });
    clientSocket.pipe(conn, { end: true });
  });
  conn.on('error', function() { clientSocket.destroy(); });
  clientSocket.on('error', function() { conn.destroy(); });
});

server.listen(PROXY_PORT, '127.0.0.1', function() {
  process.stdout.write('proxy listening on 127.0.0.1:' + PROXY_PORT + '\\n');
});

process.on('SIGTERM', function() { server.close(); process.exit(0); });
process.on('SIGINT',  function() { server.close(); process.exit(0); });
PROXYEOF
  chmod 644 "$proxy_script"

  log "starting reverse proxy on port $proxy_port"
  (
    env \
      E2E_PROXY_PORT="$proxy_port" \
      E2E_API_PORT="$api_port" \
      E2E_WEBSITE_PORT="$website_port" \
      node "$proxy_script" >>"$proxy_log" 2>&1 &
    echo $!
  ) >"$proxy_pid_file"
  local proxy_pid
  proxy_pid="$(cat "$proxy_pid_file")"
  log "proxy started (pid $proxy_pid)"

  # Wait for the proxy to be accepting connections.
  log "waiting for proxy to answer on http://127.0.0.1:${proxy_port}/"
  local proxy_ready=false
  local px_attempt=1
  while [ "$px_attempt" -le 60 ]; do
    local px_code=""
    px_code="$(curl --silent --max-time 3 -o /dev/null -w '%{http_code}' \
      "http://127.0.0.1:${proxy_port}/" 2>/dev/null)" || true
    if [ -n "$px_code" ] && [ "$px_code" != "000" ]; then
      proxy_ready=true
      break
    fi
    if ! kill -0 "$proxy_pid" 2>/dev/null; then
      log "proxy process died; last log:"
      tail -20 "$proxy_log" >&2 2>/dev/null || true
      fail "proxy exited before becoming ready"
    fi
    sleep 1
    px_attempt=$((px_attempt + 1))
  done
  if [ "$proxy_ready" != "true" ]; then
    tail -20 "$proxy_log" >&2 2>/dev/null || true
    fail "proxy did not become ready within 60 seconds"
  fi
  log "proxy is ready"

  # Wait for Next.js to be answering (it may take a moment to warm up).
  log "waiting for Next.js to answer on http://127.0.0.1:${website_port}/"
  local next_ready=false
  local nx_attempt=1
  while [ "$nx_attempt" -le 60 ]; do
    local nx_code=""
    nx_code="$(curl --silent --max-time 3 -o /dev/null -w '%{http_code}' \
      "http://127.0.0.1:${website_port}/" 2>/dev/null)" || true
    if [ -n "$nx_code" ] && [ "$nx_code" != "000" ]; then
      next_ready=true
      break
    fi
    if ! kill -0 "$next_pid" 2>/dev/null; then
      log "Next.js process died; last log:"
      tail -20 "$next_log" >&2 2>/dev/null || true
      fail "Next.js exited before becoming ready"
    fi
    sleep 1
    nx_attempt=$((nx_attempt + 1))
  done
  if [ "$next_ready" != "true" ]; then
    tail -20 "$next_log" >&2 2>/dev/null || true
    fail "Next.js did not become ready within 60 seconds"
  fi
  log "Next.js is ready"

  # ------------------------------------------------------------------
  # 13. Write the Playwright env file (600 mode).
  # ------------------------------------------------------------------
  log "writing Playwright environment file"

  local public_problem_id public_contest_id public_material_slug
  local admin_problem_id admin_contest_id policy_role_key

  public_problem_id="$(read_env_value "$env_file" "STACK_SEED_PUBLIC_PROBLEM_ID")" || fail "seeding did not produce STACK_SEED_PUBLIC_PROBLEM_ID"
  public_contest_id="$(read_env_value "$env_file" "STACK_SEED_PUBLIC_CONTEST_ID")" || fail "seeding did not produce STACK_SEED_PUBLIC_CONTEST_ID"
  public_material_slug="$(read_env_value "$env_file" "STACK_SEED_PUBLIC_MATERIAL_SLUG")" || fail "seeding did not produce STACK_SEED_PUBLIC_MATERIAL_SLUG"
  admin_problem_id="$(read_env_value "$env_file" "STACK_SEED_ADMIN_PROBLEM_ID")" || fail "seeding did not produce STACK_SEED_ADMIN_PROBLEM_ID"
  admin_contest_id="$(read_env_value "$env_file" "STACK_SEED_ADMIN_CONTEST_ID")" || fail "seeding did not produce STACK_SEED_ADMIN_CONTEST_ID"
  policy_role_key="$(read_env_value "$env_file" "STACK_SEED_POLICY_ROLE_KEY")" || fail "seeding did not produce STACK_SEED_POLICY_ROLE_KEY"

  # Write Playwright vars into the env file (they share the same file).
  write_env "JUDGIFY_E2E_ADMIN_USERNAME" "$e2e_admin_username"
  write_env "JUDGIFY_E2E_ADMIN_PASSWORD" "$e2e_admin_password"
  write_env "JUDGIFY_E2E_USER_USERNAME" "$e2e_user_username"
  write_env "JUDGIFY_E2E_USER_PASSWORD" "$e2e_user_password"
  write_env "JUDGIFY_E2E_PUBLIC_PROBLEM_ID" "$public_problem_id"
  write_env "JUDGIFY_E2E_PUBLIC_CONTEST_ID" "$public_contest_id"
  write_env "JUDGIFY_E2E_PUBLIC_MATERIAL_SLUG" "$public_material_slug"
  write_env "JUDGIFY_E2E_PUBLIC_USERNAME" "$e2e_user_username"
  write_env "JUDGIFY_E2E_ADMIN_PROBLEM_ID" "$admin_problem_id"
  write_env "JUDGIFY_E2E_ADMIN_CONTEST_ID" "$admin_contest_id"
  write_env "JUDGIFY_E2E_POLICY_ROLE_KEY" "$policy_role_key"
  write_env "JUDGIFY_E2E_POLICY_RESOURCE" "problem"
  write_env "JUDGIFY_E2E_POLICY_ACTION" "read"
  # Point Playwright at the proxy, not directly at Next.js.
  write_env "JUDGIFY_E2E_BASE_URL" "http://127.0.0.1:${proxy_port}"
  # Tell Playwright NOT to start its own web server — the harness already
  # manages both Next.js and the proxy.
  write_env "JUDGIFY_E2E_MANAGED_WEB_SERVER" "0"

  chmod 600 "$env_file"

  log "stack is up"
  log "env file:   $env_file"
  log "api log:    $api_log"
  log "next log:   $next_log"
  log "proxy log:  $proxy_log"
}

# ===========================================================================
# SEED FIXTURES
# All HTTP interactions go through the API. No direct DB access.
# ===========================================================================
seed_fixtures() {
  local admin_user="$1"
  local admin_pass="$2"
  local member_user="$3"
  local member_pass="$4"
  local _api_port="$5"

  local req="$state_root/req.json"
  local rsp="$state_root/rsp.json"
  local hdr="$state_root/hdr.txt"

  # Helper: read CSRF cookie value from the cookie jar.
  get_csrf() {
    awk '/judgify_csrf/{print $NF}' "$state_root/cookies.jar" | tail -1
  }

  # Helper: write headers file.
  write_hdrs() {
    local csrf="$1"
    local ikey="$2"
    local if_match="${3:-}"
    : >"$hdr"
    if [ -n "$csrf" ]; then
      printf 'X-CSRF-Token: %s\n' "$csrf" >>"$hdr"
    fi
    if [ -n "$ikey" ]; then
      printf 'Idempotency-Key: %s\n' "$ikey" >>"$hdr"
    fi
    if [ -n "$if_match" ]; then
      printf 'If-Match: %s\n' "$if_match" >>"$hdr"
    fi
  }

  # ---- Reset cookie jar for admin session ----
  : >"$state_root/cookies.jar"
  chmod 600 "$state_root/cookies.jar"

  # ==========================================================================
  # STEP 1: Admin login
  # ==========================================================================
  log "seeding: bootstrapping pre-session for admin login"
  local ikey
  ikey="$(gen_uuid)"
  write_hdrs "" "$ikey"
  printf '{}' >"$req"
  local sc
  sc="$(http_call POST /auth/pre-session "$hdr" "$req" "$rsp")"
  [ "$sc" = "204" ] || fail "pre-session failed (HTTP $sc): $(cat "$rsp")"

  log "seeding: admin login"
  local csrf
  csrf="$(get_csrf)"
  [ -n "$csrf" ] || fail "no CSRF cookie after pre-session"

  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  # Write login body without echoing passwords.
  python3 -c "
import json, sys
print(json.dumps({'username': sys.argv[1], 'password': sys.argv[2]}))
" "$admin_user" "$admin_pass" >"$req" 2>/dev/null || \
  awk -v u="$admin_user" -v p="$admin_pass" 'BEGIN{
    printf "{\"username\":\"%s\",\"password\":\"%s\"}", u, p
  }' >"$req"

  sc="$(http_call POST /auth/login "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "admin login failed (HTTP $sc): $(cat "$rsp")"

  csrf="$(get_csrf)"
  [ -n "$csrf" ] || fail "no CSRF cookie after admin login"
  log "seeding: admin session established"

  # ==========================================================================
  # STEP 2: Create a difficulty
  # ==========================================================================
  log "seeding: creating difficulty"
  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  printf '{"name":"Easy","level":1,"color":"#22c55e"}' >"$req"
  sc="$(http_call POST /difficulties "$hdr" "$req" "$rsp")"
  if ! is_created "$sc" && [ "$sc" != "409" ]; then
    fail "create difficulty failed (HTTP $sc): $(cat "$rsp")"
  fi

  local difficulty_id
  if is_created "$sc"; then
    difficulty_id="$(json_field "$rsp" "id")"
  else
    # Already exists — fetch it.
    sc="$(http_call GET /difficulties "" "" "$rsp")"
    [ "$sc" = "200" ] || fail "list difficulties failed (HTTP $sc)"
    difficulty_id="$(json_field "$rsp" "id")"
  fi
  [ -n "$difficulty_id" ] || fail "could not obtain difficulty_id"
  log "seeding: difficulty_id obtained"

  # ==========================================================================
  # STEP 2b: Create a seeding tag (needed to avoid nil-vs-empty-slice mismatch
  # in VerifyRewardSnapshot: cloneReward turns non-nil empty Items into nil via
  # append, so tag_snapshot stores "items":null; BuildRewardSnapshot produces
  # non-nil empty; reflect.DeepEqual fails.  One real tag keeps Items non-nil.)
  # ==========================================================================
  log "seeding: creating tag"
  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  printf '{"name":"E2E Tag","element_ids":[]}' >"$req"
  sc="$(http_call POST /tags "$hdr" "$req" "$rsp")"
  if ! is_created "$sc" && [ "$sc" != "409" ]; then
    fail "create tag failed (HTTP $sc): $(cat "$rsp")"
  fi
  local tag_id=""
  if is_created "$sc"; then
    tag_id="$(json_field "$rsp" "id")"
  else
    # Already exists — find it by listing.
    write_hdrs "$csrf" ""
    sc="$(http_call GET /tags "" "" "$rsp")"
    [ "$sc" = "200" ] || fail "list tags failed (HTTP $sc)"
    tag_id="$(json_field "$rsp" "id")"
  fi
  [ -n "$tag_id" ] || fail "could not obtain tag_id"
  log "seeding: tag_id obtained"

  # ==========================================================================
  # STEP 3: Get the authoring catalog for problem draft/publish.
  # ==========================================================================
  log "seeding: fetching authoring catalog"
  write_hdrs "$csrf" ""
  sc="$(http_call GET /admin/problems/authoring/catalog "$hdr" "" "$rsp")"
  [ "$sc" = "200" ] || fail "get authoring catalog failed (HTTP $sc): $(cat "$rsp")"

  local catalog_version checker_key checker_version runtime_key
  catalog_version="$(json_field "$rsp" "catalog_version")"
  # Use index()-based (non-greedy) extraction to get the first key/version
  # from the "runtimes" and "checkers" array sections of the catalog JSON.
  # The JSON is single-line; greedy gsub would match the last occurrence.
  runtime_key="$(awk '{
    idx = index($0, "\"runtimes\":[")
    if (idx > 0) {
      s = substr($0, idx + length("\"runtimes\":["))
      kidx = index(s, "\"key\":\"")
      if (kidx > 0) {
        s = substr(s, kidx + length("\"key\":\""))
        n = index(s, "\"")
        if (n > 0) { print substr(s, 1, n-1); exit }
      }
    }
  }' "$rsp")"
  checker_key="$(awk '{
    idx = index($0, "\"checkers\":[")
    if (idx > 0) {
      s = substr($0, idx + length("\"checkers\":["))
      kidx = index(s, "\"key\":\"")
      if (kidx > 0) {
        s = substr(s, kidx + length("\"key\":\""))
        n = index(s, "\"")
        if (n > 0) { print substr(s, 1, n-1); exit }
      }
    }
  }' "$rsp")"
  checker_version="$(awk '{
    idx = index($0, "\"checkers\":[")
    if (idx > 0) {
      s = substr($0, idx + length("\"checkers\":["))
      vidx = index(s, "\"version\":\"")
      if (vidx > 0) {
        s = substr(s, vidx + length("\"version\":\""))
        n = index(s, "\"")
        if (n > 0) { print substr(s, 1, n-1); exit }
      }
    }
  }' "$rsp")"

  [ -n "$catalog_version" ] || fail "could not extract catalog_version"
  [ -n "$checker_key" ] || fail "could not extract checker_key"
  [ -n "$checker_version" ] || fail "could not extract checker_version"
  [ -n "$runtime_key" ] || fail "could not extract runtime_key"
  log "seeding: catalog_version and runtime/checker obtained"

  # ==========================================================================
  # STEP 4: Create a problem shell.
  # ==========================================================================
  log "seeding: creating problem shell"
  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  # NOTE: tag_ids is intentionally omitted from the shell creation because the
  # problemRepo.AddTags path returns a typed-nil *apperr.AppError (a pre-existing
  # API bug: MapEntError returns *AppError not error, so the typed nil causes
  # errors.Is to panic). Tags are added via the draft (tag_ids in SaveDraft),
  # which uses a separate code path that does not call AddTags.
  awk -v diff="$difficulty_id" 'BEGIN{
    printf "{\"slug\":\"e2e-test-problem\",\"title\":\"E2E Test Problem\",\"description\":\"E2E seeded problem for Playwright tests.\",\"difficulty_id\":\"%s\"}", diff
  }' >"$req"
  sc="$(http_call POST /problems "$hdr" "$req" "$rsp")"
  is_created "$sc" || fail "create problem failed (HTTP $sc): $(cat "$rsp")"

  local problem_id problem_version
  problem_id="$(json_field "$rsp" "id")"
  problem_version="$(json_field "$rsp" "version")"
  [ -n "$problem_id" ] || fail "could not extract problem_id from response: $(cat "$rsp")"
  log "seeding: problem shell created (id=$problem_id)"

  # ==========================================================================
  # STEP 5: Save a problem draft.
  # ==========================================================================
  log "seeding: saving problem draft"
  write_hdrs "$csrf" ""
  awk -v ev="$problem_version" -v cv="$catalog_version" -v diff="$difficulty_id" \
      -v tag="$tag_id" \
      -v ck="$checker_key" -v cver="$checker_version" -v rk="$runtime_key" 'BEGIN{
    printf "{\"expected_version\":%d,\"catalog_version\":\"%s\",\"title\":\"E2E Test Problem\",\"statement_markdown\":\"# Problem\\n\\nGiven an integer N, print N.\",\"difficulty_id\":\"%s\",\"tag_ids\":[\"%s\"],\"cpu_time_ms\":2000,\"wall_time_ms\":4000,\"memory_limit_kb\":262144,\"output_limit_bytes\":65536,\"process_limit\":1,\"checker_key\":\"%s\",\"checker_version\":\"%s\",\"checker_config\":{},\"allowed_runtime_keys\":[\"%s\"],\"groups\":[{\"stop_on_failure\":false,\"cases\":[{\"kind\":\"sample\",\"input\":\"1\\n\",\"expected_output\":\"1\\n\"},{\"kind\":\"hidden\",\"input\":\"2\\n\",\"expected_output\":\"2\\n\"}]}],\"reason\":\"initial E2E draft\"}", ev, cv, diff, tag, ck, cver, rk
  }' >"$req"

  sc="$(http_call POST "/problems/$problem_id/drafts" "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "save problem draft failed (HTTP $sc): $(cat "$rsp")"

  local draft_revision_id testset_revision_id draft_version
  draft_revision_id="$(json_field "$rsp" "revision_id")"
  testset_revision_id="$(json_field "$rsp" "testset_revision_id")"
  draft_version="$(json_field "$rsp" "version")"
  [ -n "$draft_revision_id" ] || fail "could not extract draft revision_id"
  [ -n "$testset_revision_id" ] || fail "could not extract testset_revision_id"
  log "seeding: problem draft saved (revision=$draft_revision_id)"

  # Poll until artifact_status is ready (the judge worker processes in-process
  # in code_owned_development mode; it should be immediate).
  log "seeding: waiting for problem artifact to be ready"
  local artifact_status="" poll_i=1
  while [ "$poll_i" -le 30 ]; do
    sc="$(http_call GET "/admin/problems/$problem_id/authoring" "$hdr" "" "$rsp")"
    if [ "$sc" = "200" ]; then
      artifact_status="$(json_field "$rsp" "artifact_status")"
      if [ "$artifact_status" = "ready" ]; then
        break
      fi
    fi
    sleep 2
    poll_i=$((poll_i + 1))
  done
  [ "$artifact_status" = "ready" ] || {
    log "artifact_status after polling: $artifact_status"
    log "Problem artifact may not need a worker in code_owned_development mode; continuing."
    # Re-read revision IDs from the authoring endpoint in case they changed.
    sc="$(http_call GET "/admin/problems/$problem_id/authoring" "$hdr" "" "$rsp")"
    if [ "$sc" = "200" ]; then
      draft_revision_id="$(json_field "$rsp" "revision_id")"
      testset_revision_id="$(json_field "$rsp" "testset_revision_id")"
      draft_version="$(json_field "$rsp" "version")"
    fi
  }

  # ==========================================================================
  # STEP 6: Publish the problem.
  # ==========================================================================
  log "seeding: publishing problem"
  write_hdrs "$csrf" ""
  awk -v drv="$draft_revision_id" -v trv="$testset_revision_id" -v ev="$draft_version" -v cv="$catalog_version" 'BEGIN{
    printf "{\"draft_revision_id\":\"%s\",\"testset_revision_id\":\"%s\",\"expected_version\":%d,\"catalog_version\":\"%s\",\"reason\":\"publish for E2E\"}", drv, trv, ev, cv
  }' >"$req"
  sc="$(http_call POST "/problems/$problem_id/publish" "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "publish problem failed (HTTP $sc): $(cat "$rsp")"
  log "seeding: problem published"

  write_env "STACK_SEED_PUBLIC_PROBLEM_ID" "$problem_id"
  write_env "STACK_SEED_ADMIN_PROBLEM_ID" "$problem_id"

  # ==========================================================================
  # STEP 7: Create and publish a contest.
  # ==========================================================================
  log "seeding: creating contest"
  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  # Start time 1 hour from now, end time 25 hours from now.
  local start_time end_time
  start_time="$(python3 -c "import datetime; print((datetime.datetime.utcnow()+datetime.timedelta(hours=1)).strftime('%Y-%m-%dT%H:%M:%SZ'))" 2>/dev/null)" \
    || start_time="2027-01-01T10:00:00Z"
  end_time="$(python3 -c "import datetime; print((datetime.datetime.utcnow()+datetime.timedelta(hours=25)).strftime('%Y-%m-%dT%H:%M:%SZ'))" 2>/dev/null)" \
    || end_time="2027-01-02T12:00:00Z"

  # NOTE: ScoringPolicyICPC (the only supported policy) requires points=nil,
  # so "points" must be omitted from the problems array entirely.
  awk -v pid="$problem_id" -v st="$start_time" -v et="$end_time" 'BEGIN{
    printf "{\"reason\":\"E2E seeded contest\",\"title\":\"E2E Test Contest\",\"description\":\"Contest created by the E2E stack harness.\",\"start_time\":\"%s\",\"end_time\":\"%s\",\"registration_mode\":\"open\",\"problems\":[{\"problem_id\":\"%s\",\"alias\":\"A\",\"visible_before_start\":true}]}", st, et, pid
  }' >"$req"
  sc="$(http_call POST /contests "$hdr" "$req" "$rsp")"
  is_created "$sc" || fail "create contest failed (HTTP $sc): $(cat "$rsp")"

  local contest_id contest_version
  contest_id="$(json_field "$rsp" "id")"
  contest_version="$(json_field "$rsp" "version")"
  [ -n "$contest_id" ] || fail "could not extract contest_id"
  log "seeding: contest created (id=$contest_id)"

  write_env "STACK_SEED_ADMIN_CONTEST_ID" "$contest_id"

  # Publish the contest through the real API. A failure here is a product
  # defect and must surface as one; the harness never writes contest state
  # directly, or the E2E would stop proving the publish path works.
  log "seeding: publishing contest"
  ikey="$(gen_uuid)"
  local if_match
  if_match="\"contest-v${contest_version}\""
  write_hdrs "$csrf" "$ikey" "$if_match"
  printf '{"reason":"publish for E2E"}' >"$req"
  sc="$(http_call POST "/contests/$contest_id/publish" "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "publish contest failed (HTTP $sc): $(cat "$rsp")"
  log "seeding: contest published"

  write_env "STACK_SEED_PUBLIC_CONTEST_ID" "$contest_id"

  # ==========================================================================
  # STEP 8: Create a material category.
  # ==========================================================================
  log "seeding: creating material category"
  write_hdrs "$csrf" ""
  printf '{"name":"General","description":"General learning materials"}' >"$req"
  sc="$(http_call POST /material-categories "$hdr" "$req" "$rsp")"
  if ! is_created "$sc" && [ "$sc" != "409" ]; then
    fail "create material category failed (HTTP $sc): $(cat "$rsp")"
  fi

  local category_id
  if is_created "$sc"; then
    category_id="$(json_field "$rsp" "id")"
  else
    sc="$(http_call GET /material-categories "" "" "$rsp")"
    [ "$sc" = "200" ] || fail "list material categories failed (HTTP $sc)"
    category_id="$(json_field "$rsp" "id")"
  fi
  [ -n "$category_id" ] || fail "could not obtain category_id"

  # ==========================================================================
  # STEP 9: Create and publish a material.
  # We must do a fresh admin login immediately before the lifecycle apply calls
  # because RequireRecentAuthentication checks elapsed time since last login.
  # ==========================================================================
  log "seeding: re-logging in as admin before material lifecycle ops"
  # Re-login to reset the recent-auth clock.
  : >"$state_root/cookies.jar"
  chmod 600 "$state_root/cookies.jar"

  ikey="$(gen_uuid)"
  write_hdrs "" "$ikey"
  printf '{}' >"$req"
  sc="$(http_call POST /auth/pre-session "$hdr" "$req" "$rsp")"
  [ "$sc" = "204" ] || fail "pre-session (re-login) failed (HTTP $sc)"

  csrf="$(get_csrf)"
  [ -n "$csrf" ] || fail "no CSRF after pre-session (re-login)"

  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  python3 -c "
import json, sys
print(json.dumps({'username': sys.argv[1], 'password': sys.argv[2]}))
" "$admin_user" "$admin_pass" >"$req" 2>/dev/null || \
  awk -v u="$admin_user" -v p="$admin_pass" 'BEGIN{
    printf "{\"username\":\"%s\",\"password\":\"%s\"}", u, p
  }' >"$req"

  sc="$(http_call POST /auth/login "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "admin re-login failed (HTTP $sc)"
  csrf="$(get_csrf)"
  [ -n "$csrf" ] || fail "no CSRF after admin re-login"

  log "seeding: creating material draft"
  local material_slug="e2e-test-material"
  local material_cmd_id
  material_cmd_id="$(gen_uuid)"
  write_hdrs "$csrf" "$material_cmd_id"
  awk -v cid="$material_cmd_id" -v slug="$material_slug" -v diff="$difficulty_id" -v cat="$category_id" 'BEGIN{
    printf "{\"command_id\":\"%s\",\"slug\":\"%s\",\"title\":\"E2E Test Material\",\"description\":\"A material article for E2E testing.\",\"content\":\"# Introduction\\n\\nThis material exists to satisfy the E2E test harness.\",\"difficulty_id\":\"%s\",\"category_id\":\"%s\",\"tag_ids\":[],\"change_summary\":\"initial E2E draft\",\"visibility\":\"public\"}", cid, slug, diff, cat
  }' >"$req"
  sc="$(http_call POST /admin/materials/drafts "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "create material draft failed (HTTP $sc): $(cat "$rsp")"

  local material_id material_version
  material_id="$(json_field "$rsp" "material_id")"
  material_version="$(json_field "$rsp" "version")"
  [ -n "$material_id" ] || fail "could not extract material_id"
  log "seeding: material draft created (id=$material_id)"

  # Submit for review — preview then apply.
  log "seeding: submitting material for review (preview)"
  local review_cmd_id
  review_cmd_id="$(gen_uuid)"
  write_hdrs "$csrf" "$review_cmd_id"
  awk -v cid="$review_cmd_id" -v ev="$material_version" 'BEGIN{
    printf "{\"command_id\":\"%s\",\"expected_version\":%d,\"reason\":\"submit for E2E review\"}", cid, ev
  }' >"$req"
  sc="$(http_call POST "/admin/materials/$material_id/submit-review/preview" "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "submit-review preview failed (HTTP $sc): $(cat "$rsp")"

  local review_token
  review_token="$(json_field "$rsp" "confirmation_token")"
  [ -n "$review_token" ] || fail "no confirmation_token in submit-review preview"

  log "seeding: applying submit-review"
  write_hdrs "$csrf" "$review_cmd_id"
  awk -v cid="$review_cmd_id" -v ev="$material_version" -v tok="$review_token" 'BEGIN{
    printf "{\"command_id\":\"%s\",\"expected_version\":%d,\"reason\":\"submit for E2E review\",\"confirmation_token\":\"%s\"}", cid, ev, tok
  }' >"$req"
  sc="$(http_call POST "/admin/materials/$material_id/submit-review" "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "submit-review apply failed (HTTP $sc): $(cat "$rsp")"

  local review_version
  review_version="$(json_field "$rsp" "version")"
  log "seeding: material in_review (version=$review_version)"

  # Fetch current draft revision ID before publish.
  sc="$(http_call GET "/admin/materials/$material_id" "$hdr" "" "$rsp")"
  [ "$sc" = "200" ] || fail "get material (pre-publish) failed (HTTP $sc)"
  local draft_rev_id
  draft_rev_id="$(json_field "$rsp" "revision_id")"
  # revision_id may be null string; handle gracefully.
  if [ -z "$draft_rev_id" ] || [ "$draft_rev_id" = "null" ]; then
    # Fallback: try published_revision_id or just use material_id.
    draft_rev_id="$(awk '/"revision_id"/{gsub(/.*"revision_id"[[:space:]]*:[[:space:]]*/,""); gsub(/[",}].*/,""); if($0!="null"&&$0!=""){print; exit}}' "$rsp")"
  fi
  log "seeding: draft_revision_id for publish: $draft_rev_id"

  # Publish — preview then apply.
  log "seeding: publishing material (preview)"
  local publish_cmd_id
  publish_cmd_id="$(gen_uuid)"
  write_hdrs "$csrf" "$publish_cmd_id"
  awk -v cid="$publish_cmd_id" -v drv="$draft_rev_id" -v ev="$review_version" 'BEGIN{
    printf "{\"command_id\":\"%s\",\"draft_revision_id\":\"%s\",\"expected_version\":%d,\"reason\":\"publish for E2E\"}", cid, drv, ev
  }' >"$req"
  sc="$(http_call POST "/admin/materials/$material_id/publish/preview" "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "publish material preview failed (HTTP $sc): $(cat "$rsp")"

  local publish_token
  publish_token="$(json_field "$rsp" "confirmation_token")"
  [ -n "$publish_token" ] || fail "no confirmation_token in publish preview"

  log "seeding: applying material publish"
  write_hdrs "$csrf" "$publish_cmd_id"
  awk -v cid="$publish_cmd_id" -v drv="$draft_rev_id" -v ev="$review_version" -v tok="$publish_token" 'BEGIN{
    printf "{\"command_id\":\"%s\",\"draft_revision_id\":\"%s\",\"expected_version\":%d,\"reason\":\"publish for E2E\",\"confirmation_token\":\"%s\"}", cid, drv, ev, tok
  }' >"$req"
  sc="$(http_call POST "/admin/materials/$material_id/publish" "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "publish material apply failed (HTTP $sc): $(cat "$rsp")"
  log "seeding: material published (slug=$material_slug)"

  write_env "STACK_SEED_PUBLIC_MATERIAL_SLUG" "$material_slug"

  # ==========================================================================
  # STEP 9b: Seed the cultivation catalog (rarity + traits) so that the
  # onboarding trait-offer endpoint can build a valid CatalogRevision.
  # Valid() requires: len(RootPool) >= 1, len(TalentPool) >= 6.
  # Each trait's Weight comes from its rarity.weight (must be >= 1).
  # Each trait must be active=true (default on creation) and have an
  # effect_revision whose revision matches the trait version (= 1 on creation).
  # Effect spec uses scope=all to avoid requiring elements.
  # ==========================================================================
  # Create a cultivation element (needed for element-scoped trait effects; the
  # trait_effect_revisions CHECK constraint requires target_element_ids to be a
  # JSON array — scope=all produces a nil Go slice that marshals to null, which
  # fails the constraint — so we must use scope=element with a real element).
  log "seeding: creating cultivation element"
  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  printf '{"name":"E2E Fire","code":"e2e_fire","description":"E2E seeding element"}' >"$req"
  sc="$(http_call POST /elements "$hdr" "$req" "$rsp")"
  if ! is_created "$sc" && [ "$sc" != "409" ]; then
    fail "create element failed (HTTP $sc): $(cat "$rsp")"
  fi
  local element_code="e2e_fire"
  log "seeding: element created (code=$element_code)"

  log "seeding: creating cultivation rarity"
  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  printf '{"name":"Common","code":"common","weight":10,"description":"Common rarity tier"}' >"$req"
  sc="$(http_call POST /rarities "$hdr" "$req" "$rsp")"
  if ! is_created "$sc" && [ "$sc" != "409" ]; then
    fail "create rarity failed (HTTP $sc): $(cat "$rsp")"
  fi

  local rarity_id
  if is_created "$sc"; then
    rarity_id="$(json_field "$rsp" "id")"
  else
    sc="$(http_call GET /rarities "" "" "$rsp")"
    [ "$sc" = "200" ] || fail "list rarities failed (HTTP $sc): $(cat "$rsp")"
    rarity_id="$(json_field "$rsp" "id")"
  fi
  [ -n "$rarity_id" ] || fail "could not obtain rarity_id"
  log "seeding: rarity created (id=$rarity_id)"

  # Create 1 root_bone trait + 6 talent traits.
  # The DB CHECK constraint requires target_element_ids to be a JSON array, so
  # we must use scope=element (scope=all marshals a nil slice to JSON null).
  # effect spec: {"v":1,"kind":"exp_multiplier","scope":"element","element_codes":["e2e_fire"],...}
  # This call used to return HTTP 500 for every trait mutation: traitDocument()
  # handed boundedjson a *entity.TraitEffectRevision, which the walker has no
  # case for, so the audit entry failed and rolled the insert back. Fixed in
  # api/internal/cultivation/core/service/mutation.go.
  log "seeding: creating root_bone trait"
  ikey="$(gen_uuid)"
  write_hdrs "$csrf" "$ikey"
  awk -v rid="$rarity_id" -v ec="$element_code" 'BEGIN{
    printf "{\"code\":\"e2e_root_bone\",\"type\":\"root_bone\",\"name\":\"E2E Root Bone\",\"rarity_id\":\"%s\",\"display_order\":0,\"effect\":{\"v\":1,\"kind\":\"exp_multiplier\",\"scope\":\"element\",\"element_codes\":[\"%s\"],\"multiplier_delta_bps\":100},\"reason\":\"E2E seeding root bone trait\"}", rid, ec
  }' >"$req"
  sc="$(http_call POST /traits "$hdr" "$req" "$rsp")"
  if ! is_created "$sc" && [ "$sc" != "409" ]; then
    fail "create root_bone trait failed (HTTP $sc): $(cat "$rsp")"
  fi
  log "seeding: root_bone trait created"

  log "seeding: creating 6 talent traits"
  local ti=1
  while [ "$ti" -le 6 ]; do
    ikey="$(gen_uuid)"
    write_hdrs "$csrf" "$ikey"
    awk -v rid="$rarity_id" -v n="$ti" -v ec="$element_code" 'BEGIN{
      printf "{\"code\":\"e2e_talent_%d\",\"type\":\"talent\",\"name\":\"E2E Talent %d\",\"rarity_id\":\"%s\",\"display_order\":%d,\"effect\":{\"v\":1,\"kind\":\"exp_multiplier\",\"scope\":\"element\",\"element_codes\":[\"%s\"],\"multiplier_delta_bps\":50},\"reason\":\"E2E seeding talent trait\"}", n, n, rid, n, ec
    }' >"$req"
    sc="$(http_call POST /traits "$hdr" "$req" "$rsp")"
    if ! is_created "$sc" && [ "$sc" != "409" ]; then
      fail "create talent trait $ti failed (HTTP $sc): $(cat "$rsp")"
    fi
    log "seeding: talent trait $ti created"
    ti=$((ti + 1))
  done
  log "seeding: cultivation catalog seeded (1 root_bone + 6 talent traits)"

  # ==========================================================================
  # STEP 10: Create the member user via public self-registration.
  # ==========================================================================
  log "seeding: creating member user"

  # Use a separate cookie jar section by resetting.
  local member_cookies="$state_root/member.cookies"
  : >"$member_cookies"
  chmod 600 "$member_cookies"

  # Pre-session for member.
  local member_ikey
  member_ikey="$(gen_uuid)"
  local member_csrf=""

  # Temporarily swap cookie jar for member flow.
  local orig_cookies="$state_root/cookies.jar"
  cp "$orig_cookies" "$state_root/cookies.jar.admin.bak"
  cp "$member_cookies" "$orig_cookies"

  write_hdrs "" "$member_ikey"
  printf '{}' >"$req"
  sc="$(http_call POST /auth/pre-session "$hdr" "$req" "$rsp")"
  [ "$sc" = "204" ] || fail "member pre-session failed (HTTP $sc)"

  member_csrf="$(get_csrf)"
  [ -n "$member_csrf" ] || fail "no CSRF for member pre-session"

  # Get profile schema.
  sc="$(http_call GET /onboarding/profile-schema "" "" "$rsp")"
  [ "$sc" = "200" ] || fail "get profile schema failed (HTTP $sc): $(cat "$rsp")"

  local schema_revision schema_checksum
  schema_revision="$(json_field "$rsp" "revision")"
  schema_checksum="$(json_field "$rsp" "checksum")"
  [ -n "$schema_revision" ] || fail "could not extract schema revision"
  [ -n "$schema_checksum" ] || fail "could not extract schema checksum"

  # All fields returned by /onboarding/profile-schema are required_on_onboarding=true.
  # Extract definition_id, definition_revision_id, and data_type for each field.
  # Build profile_values JSON array.
  # The API returns compact single-line JSON; use Python3 to parse it correctly
  # so that all N fields are extracted, not just the first (awk match() only finds
  # the first occurrence per line, which is wrong for multi-field compact responses).
  local profile_values_json
  profile_values_json="$(python3 - "$rsp" <<'PYEOF'
import json, sys
with open(sys.argv[1]) as fh:
    body = json.load(fh)
fields = body.get("data", body).get("fields", [])
result = []
for f in fields:
    dt = f.get("data_type", "string")
    if dt == "number":
        val = 0
    elif dt == "boolean":
        val = False
    elif dt == "date":
        val = "2000-01-01"
    else:
        val = "placeholder"
    result.append({
        "definition_id": f["definition_id"],
        "definition_revision_id": f["definition_revision_id"],
        "value": val,
    })
print(json.dumps(result, separators=(",", ":")))
PYEOF
)"
  # If no required fields, use empty array (registration will still work if the
  # service allows it, otherwise we must provide at least one dummy field).
  # The DTO says: min=1, max=128 — so we need at least one.
  # If profile_values_json is "[]", we need to synthesize a placeholder.
  if [ "$profile_values_json" = "[]" ]; then
    # We cannot register without a field (profile_values min=1).
    # Create an attribute definition using the admin session, then re-fetch schema.
    log "seeding: no required_on_onboarding fields found; creating an attribute definition"
    # Save current member cookie jar; restore admin cookies.
    cp "$orig_cookies" "$member_cookies"
    cp "$state_root/cookies.jar.admin.bak" "$orig_cookies"

    # Refresh admin CSRF (now admin cookies are back in orig_cookies).
    csrf="$(get_csrf)"

    # Create an attribute definition at the correct route /attribute-definitions.
    # command_id in body must match the Idempotency-Key header.
    local attr_cmd_id
    attr_cmd_id="$(gen_uuid)"
    write_hdrs "$csrf" "$attr_cmd_id"
    awk -v cid="$attr_cmd_id" 'BEGIN{
      printf "{\"command_id\":\"%s\",\"reason\":\"E2E seeding required onboarding field\",\"key\":\"e2e_bio\",\"data_type\":\"string\",\"label\":\"Biography\",\"description\":\"A short bio for E2E testing\",\"visibility\":\"public\",\"user_editable\":true,\"display_order\":0,\"required_on_onboarding\":true,\"validation\":{}}", cid
    }' >"$req"
    sc="$(http_call POST /attribute-definitions "$hdr" "$req" "$rsp")"
    if is_created "$sc" || [ "$sc" = "200" ]; then
      log "seeding: attribute definition created"
      # Swap back to member cookies to re-fetch profile schema.
      cp "$orig_cookies" "$state_root/cookies.jar.admin.bak"
      cp "$member_cookies" "$orig_cookies"
      sc="$(http_call GET /onboarding/profile-schema "" "" "$rsp")"
      [ "$sc" = "200" ] || fail "re-fetch profile schema failed (HTTP $sc): $(cat "$rsp")"
      schema_revision="$(json_field "$rsp" "revision")"
      schema_checksum="$(json_field "$rsp" "checksum")"
      # All fields returned by the profile schema endpoint are required_on_onboarding=true.
      # Extract definition_id, definition_revision_id, and data_type for each field.
      # Use Python3 to parse compact single-line JSON correctly (all fields, not just first).
      profile_values_json="$(python3 - "$rsp" <<'PYEOF'
import json, sys
with open(sys.argv[1]) as fh:
    body = json.load(fh)
fields = body.get("data", body).get("fields", [])
result = []
for f in fields:
    dt = f.get("data_type", "string")
    if dt == "number":
        val = 0
    elif dt == "boolean":
        val = False
    elif dt == "date":
        val = "2000-01-01"
    else:
        val = "placeholder"
    result.append({
        "definition_id": f["definition_id"],
        "definition_revision_id": f["definition_revision_id"],
        "value": val,
    })
print(json.dumps(result, separators=(",", ":")))
PYEOF
)"
      # Keep member cookies in place for the next steps.
    else
      log "seeding: attribute definition creation returned HTTP $sc: $(cat "$rsp")"
      # Swap back to member cookies.
      cp "$orig_cookies" "$state_root/cookies.jar.admin.bak"
      cp "$member_cookies" "$orig_cookies"
      fail "cannot register without a profile attribute definition (profile_values requires min=1)"
    fi
  fi

  # Get a trait offer.
  log "seeding: obtaining trait offer"
  member_ikey="$(gen_uuid)"
  write_hdrs "$member_csrf" "$member_ikey"
  printf '{}' >"$req"
  sc="$(http_call POST /onboarding/trait-offers "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "create trait offer failed (HTTP $sc): $(cat "$rsp")"

  local offer_id offer_sequence catalog_rev_id catalog_checksum offer_checksum root_bone_id
  offer_id="$(json_field "$rsp" "id")"
  offer_sequence="$(json_field "$rsp" "sequence")"
  catalog_rev_id="$(json_field "$rsp" "catalog_revision_id")"
  catalog_checksum="$(json_field "$rsp" "catalog_checksum")"
  offer_checksum="$(json_field "$rsp" "offer_checksum")"
  # root_bone is a nested object — extract its id using Python3 so that the greedy
  # awk match() bug (which would return the last talent id instead of root_bone.id)
  # cannot occur.
  root_bone_id="$(python3 -c "
import json, sys
with open(sys.argv[1]) as fh:
    body = json.load(fh)
data = body.get('data', body)
rb = data.get('root_bone') or {}
print(rb.get('id', ''))
" "$rsp")"
  # talent_ids: the response is one compact JSON line, so awk's match() would
  # only ever see the first id on it. Take every id that follows the talents
  # marker instead, then keep the three the contract requires.
  local talent_ids_json
  talent_ids_json="$(
    sed 's/.*"talents"//' "$rsp" |
      grep -oE '"id"[[:space:]]*:[[:space:]]*"[0-9a-fA-F-]{36}"' |
      sed -E 's/.*"([0-9a-fA-F-]{36})"$/\1/' |
      head -3 |
      awk 'BEGIN{printf "["} {printf "%s\"%s\"", (NR>1?",":""), $0} END{printf "]"}'
  )"

  [ -n "$offer_id" ] || fail "could not extract offer_id"
  [ -n "$root_bone_id" ] || fail "could not extract root_bone_id"
  log "seeding: trait offer obtained"

  # Register the member.
  log "seeding: registering member account"
  member_ikey="$(gen_uuid)"
  write_hdrs "$member_csrf" "$member_ikey"

  # Build the registration body using awk (no jq needed, no secret echoed).
  awk -v uname="$member_user" -v pass="$member_pass" \
      -v oid="$offer_id" -v oseq="$offer_sequence" \
      -v crid="$catalog_rev_id" -v cchk="$catalog_checksum" -v ochk="$offer_checksum" \
      -v rbid="$root_bone_id" -v tids="$talent_ids_json" \
      -v srev="$schema_revision" -v schk="$schema_checksum" \
      -v pvals="$profile_values_json" \
      'BEGIN{
    printf "{\"username\":\"%s\",\"password\":\"%s\",\"recovery_email\":\"e2emember@localhost.invalid\",\"offer_id\":\"%s\",\"offer_sequence\":%s,\"catalog_revision_id\":\"%s\",\"catalog_checksum\":\"%s\",\"offer_checksum\":\"%s\",\"root_bone_id\":\"%s\",\"talent_ids\":%s,\"profile_schema_revision\":%s,\"profile_schema_checksum\":\"%s\",\"profile_values\":%s}",
    uname, pass, oid, oseq, crid, cchk, ochk, rbid, tids, srev, schk, pvals
  }' >"$req"

  sc="$(http_call POST /auth/register "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "register member failed (HTTP $sc): $(cat "$rsp")"
  log "seeding: member registered"

  # Now the member session cookies are in orig_cookies (which we swapped to).
  # Update member_csrf.
  member_csrf="$(get_csrf)"
  [ -n "$member_csrf" ] || fail "no CSRF after member registration"

  # Set member profile to public.
  log "seeding: setting member profile to public"

  # Get current privacy state.
  sc="$(http_call GET /users/profile/privacy "$hdr" "" "$rsp")"
  [ "$sc" = "200" ] || fail "get profile privacy failed (HTTP $sc): $(cat "$rsp")"

  local privacy_version
  privacy_version="$(json_field "$rsp" "version")"
  [ -n "$privacy_version" ] || privacy_version="1"

  local privacy_cmd_id
  privacy_cmd_id="$(gen_uuid)"
  write_hdrs "$member_csrf" "$privacy_cmd_id"
  awk -v ev="$privacy_version" 'BEGIN{
    printf "{\"visibility\":\"public\",\"expected_version\":%d}", ev
  }' >"$req"
  sc="$(http_call PUT /users/profile/privacy "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || {
    log "set profile public returned HTTP $sc; response: $(cat "$rsp")"
    log "proceeding despite privacy update failure"
  }
  log "seeding: member profile visibility set to public"

  # Restore admin cookies.
  cp "$orig_cookies" "$member_cookies"
  cp "$state_root/cookies.jar.admin.bak" "$orig_cookies"

  # ==========================================================================
  # STEP 11: Obtain the policy role key from the roles API.
  # ==========================================================================
  log "seeding: fetching roles to get policy role key"
  csrf="$(get_csrf)"
  sc="$(http_call GET /roles "$hdr" "" "$rsp")"
  [ "$sc" = "200" ] || fail "list roles failed (HTTP $sc): $(cat "$rsp")"

  # The student role has key "student".
  local policy_role_key="student"
  log "seeding: using role key '$policy_role_key' for policy E2E"

  write_env "STACK_SEED_POLICY_ROLE_KEY" "$policy_role_key"

  # Cleanup temp req/rsp files.
  rm -f "$req" "$rsp" "$hdr" "$state_root/cookies.jar.admin.bak" "$member_cookies"

  log "seeding complete"
}

# ===========================================================================
# SUBCOMMAND: down
# ===========================================================================
cmd_down() {
  log "stopping stack"

  local server_bin="$state_root/judgify-server"

  # Helper: stop a process by pid file, with a graceful TERM → KILL sequence.
  # Usage: stop_pid_file LABEL PID_FILE
  stop_pid_file() {
    local label="$1" pf="$2"
    local spid=""
    [ -f "$pf" ] && spid="$(cat "$pf" 2>/dev/null)" || spid=""
    if [ -n "$spid" ] && kill -0 "$spid" 2>/dev/null; then
      kill -TERM "$spid" 2>/dev/null || true
      local si=1
      while [ "$si" -le 10 ] && kill -0 "$spid" 2>/dev/null; do
        sleep 1
        si=$((si + 1))
      done
      kill -KILL "$spid" 2>/dev/null || true
      log "$label process $spid stopped"
    else
      [ -n "$spid" ] && log "$label pid $spid was not running (stale pid file)"
    fi
    rm -f "$pf"
  }

  # --- Kill proxy ---
  stop_pid_file "proxy" "$proxy_pid_file"

  # --- Kill Next.js ---
  stop_pid_file "Next.js" "$next_pid_file"


  # --- Kill API process ---
  # Primary: use the pid file if it points at a live process.
  local api_pid=""
  if [ -f "$pid_file" ]; then
    api_pid="$(cat "$pid_file" 2>/dev/null)" || api_pid=""
  fi

  # If the pid is live, kill it.
  if [ -n "$api_pid" ] && kill -0 "$api_pid" 2>/dev/null; then
    kill -TERM "$api_pid" 2>/dev/null || true
    local i=1
    while [ "$i" -le 10 ] && kill -0 "$api_pid" 2>/dev/null; do
      sleep 1
      i=$((i + 1))
    done
    kill -KILL "$api_pid" 2>/dev/null || true
    log "API process $api_pid stopped"
  else
    # Fallback: find any process running our private server binary by path.
    # pgrep -f matches against the full command string; we use the absolute
    # path so we can never accidentally signal an unrelated process.
    if [ -f "$server_bin" ]; then
      local stale_pids
      stale_pids="$(pgrep -f "$server_bin" 2>/dev/null)" || stale_pids=""
      if [ -n "$stale_pids" ]; then
        printf '%s\n' "$stale_pids" | while IFS= read -r spid; do
          [ -n "$spid" ] || continue
          kill -TERM "$spid" 2>/dev/null || true
          log "killed stale server process $spid"
        done
      fi
    fi
    [ -n "$api_pid" ] && log "pid $api_pid was not running (stale pid file)"
  fi
  rm -f "$pid_file"

  # --- Remove the container ---
  if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx "$container_name"; then
    docker rm -f "$container_name" >/dev/null 2>&1 || true
    log "container $container_name removed"
  else
    log "container $container_name not found (already gone)"
  fi

  log "stack down"
}

# ===========================================================================
# SUBCOMMAND: env
# ===========================================================================
cmd_env() {
  printf '%s\n' "$env_file"
}

# ===========================================================================
# SUBCOMMAND: status
# ===========================================================================
cmd_status() {
  local api_pid="" api_running="no" container_running="no"
  local next_pid="" next_running="no"
  local proxy_pid_val="" proxy_running="no"

  if [ -f "$pid_file" ]; then
    api_pid="$(cat "$pid_file")"
    if [ -n "$api_pid" ] && kill -0 "$api_pid" 2>/dev/null; then
      api_running="yes (pid $api_pid)"
    fi
  fi

  if [ -f "$next_pid_file" ]; then
    next_pid="$(cat "$next_pid_file")"
    if [ -n "$next_pid" ] && kill -0 "$next_pid" 2>/dev/null; then
      next_running="yes (pid $next_pid)"
    fi
  fi

  if [ -f "$proxy_pid_file" ]; then
    proxy_pid_val="$(cat "$proxy_pid_file")"
    if [ -n "$proxy_pid_val" ] && kill -0 "$proxy_pid_val" 2>/dev/null; then
      proxy_running="yes (pid $proxy_pid_val)"
    fi
  fi

  if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$container_name"; then
    container_running="yes"
  fi

  printf 'container %s: %s\n' "$container_name" "$container_running"
  printf 'api server:    %s\n' "$api_running"
  printf 'next.js:       %s\n' "$next_running"
  printf 'proxy:         %s\n' "$proxy_running"
  printf 'env file:      %s\n' "$env_file"
}

# ===========================================================================
# Signal trap for cleanup on Ctrl-C
# ===========================================================================
on_signal() {
  log "interrupted — cleaning up"
  cmd_down 2>/dev/null || true
  exit 130
}
trap on_signal INT TERM

# ===========================================================================
# SUBCOMMAND: judge
#
# Drives the complete judge-worker boot chain against a running stack.
# Requires the stack to already be up (run `up` first).
#
# Steps attempted in order:
#   1. Build the judgify operator CLI (cmd/judgify).
#   2. Generate a release-manifest draft via `judgify release-manifest template`.
#   3. Add all runtime-checker capability pairs to the draft.
#   4. Attempt `judgify release-manifest sandbox-env` to produce the sandbox
#      Make fragment needed to build the sandbox image with correct runtime labels.
#   5. Build the sandbox image with those build-args.
#   6. Resolve the immutable image digest.
#   7. Approve the manifest via `judgify release-manifest approve`.
#   8. Verify the manifest.
#   9. Build the judge-worker binary.
#  10. Boot the worker and wait for its /healthz probe.
#  11. Submit a correct solution; poll for verdict.
#  12. Submit a wrong solution; poll for verdict.
#  13. Tear down the worker.
#
# The script stops at the first genuine blocker and reports it precisely.
# It NEVER modifies any production control to force a pass.
# ===========================================================================
cmd_judge() {
  require_cmd docker
  require_cmd curl
  require_cmd awk
  require_cmd python3

  # ------------------------------------------------------------------
  # Guard: stack must already be up.
  # ------------------------------------------------------------------
  if [ ! -f "$env_file" ]; then
    fail "judge: stack is not up — run 'up' first (env file missing: $env_file)"
  fi
  local api_pid=""
  if [ -f "$pid_file" ]; then
    api_pid="$(cat "$pid_file" 2>/dev/null)" || api_pid=""
  fi
  if [ -z "$api_pid" ] || ! kill -0 "$api_pid" 2>/dev/null; then
    fail "judge: API server is not running — run 'up' first"
  fi

  # ------------------------------------------------------------------
  # Working directories for judge artifacts.
  # ------------------------------------------------------------------
  local judge_dir="$state_root/judge"
  mkdir -p "$judge_dir"
  chmod 700 "$judge_dir"

  local release_id="judgify.e2e.2026.08.1"
  local draft_path="$judge_dir/draft.json"
  local sandbox_mk="$judge_dir/sandbox.mk"
  local manifest_path="$judge_dir/release.json"
  local judgify_bin="$judge_dir/judgify"
  local worker_bin="$judge_dir/judge-worker"
  local inspector_bin="$judge_dir/judgify-capability-inspector"
  local worker_log="$state_root/worker.log"
  local worker_pid_file="$state_root/worker.pid"

  local judge_forge_dir="$judge_dir/forge"
  local judge_staging_dir="$judge_dir/staging"
  local judge_worker_probe_port

  judge_worker_probe_port="$(find_free_port)"

  # ------------------------------------------------------------------
  # STEP 1: Build the judgify operator CLI.
  # ------------------------------------------------------------------
  log "judge: building cmd/judgify"
  (
    cd "$api_dir"
    env GOPROXY=off go build -trimpath -o "$judgify_bin" ./cmd/judgify
  )
  log "judge: judgify binary at $judgify_bin"

  # ------------------------------------------------------------------
  # STEP 2: Generate the release-manifest draft.
  # ------------------------------------------------------------------
  rm -f "$draft_path"
  log "judge: generating release-manifest draft (release-id=$release_id)"
  "$judgify_bin" release-manifest template \
    --release-id "$release_id" \
    --output "$draft_path" >&2
  log "judge: draft created at $draft_path"

  # ------------------------------------------------------------------
  # STEP 3: Add all capability pairs to the draft.
  # The template leaves capabilities empty; we add all 12 (runtime x checker) pairs.
  # ------------------------------------------------------------------
  log "judge: adding capability pairs to draft"
  python3 - "$draft_path" <<'PYEOF'
import json, sys
with open(sys.argv[1]) as f:
    d = json.load(f)
d["capabilities"] = [
    {"runtime_key": "cpp17",   "checker_key": "exact"},
    {"runtime_key": "cpp17",   "checker_key": "token"},
    {"runtime_key": "cpp17",   "checker_key": "float"},
    {"runtime_key": "python3", "checker_key": "exact"},
    {"runtime_key": "python3", "checker_key": "token"},
    {"runtime_key": "python3", "checker_key": "float"},
    {"runtime_key": "java17",  "checker_key": "exact"},
    {"runtime_key": "java17",  "checker_key": "token"},
    {"runtime_key": "java17",  "checker_key": "float"},
    {"runtime_key": "go",      "checker_key": "exact"},
    {"runtime_key": "go",      "checker_key": "token"},
    {"runtime_key": "go",      "checker_key": "float"},
]
import os
# Write atomically to preserve mode 0600
tmp = sys.argv[1] + ".tmp"
with open(tmp, "w") as f:
    json.dump(d, f, indent=2)
os.chmod(tmp, 0o600)
os.rename(tmp, sys.argv[1])
PYEOF
  log "judge: draft now has 12 capability pairs"

  # ------------------------------------------------------------------
  # STEP 4: Produce the sandbox Makefile fragment from the draft.
  #
  # This step used to fail for every possible draft: WriteSandboxMakefile
  # forbade ':' in field values while every ManifestChecksum is "sha256:<hex>".
  # Fixed in api/internal/operator/releasemanifest/files.go and pinned by
  # TestWriteSandboxMakefileEmitsEveryRuntimeFieldFromACodeOwnedDraft.
  # ------------------------------------------------------------------
  log "judge: generating sandbox build identities"
  rm -f "$sandbox_mk"
  "$judgify_bin" release-manifest sandbox-env \
    --draft "$draft_path" \
    --output "$sandbox_mk" ||
    fail "judge: release-manifest sandbox-env failed"

  # ------------------------------------------------------------------
  # STEP 5: Build the sandbox image with the correct runtime labels.
  # ------------------------------------------------------------------
  local sandbox_tag="judgify/sandbox:e2e-judge-$(id -u)"
  log "judge: building sandbox image tag=$sandbox_tag"
  (
    cd "$root_dir"
    make docker-build-sandbox \
      SANDBOX_RELEASE_ENV="$sandbox_mk" \
      VERSION="e2e-judge-$(id -u)" \
      >/dev/null 2>&1
  )
  log "judge: sandbox image built"

  # ------------------------------------------------------------------
  # STEP 6: Resolve the immutable content digest.
  # ------------------------------------------------------------------
  log "judge: resolving sandbox image digest"
  local sandbox_digest
  sandbox_digest="$(docker image inspect --format '{{.Id}}' "$sandbox_tag" 2>/dev/null)"
  if [ -z "$sandbox_digest" ] || ! printf '%s' "$sandbox_digest" | grep -qE '^sha256:[0-9a-f]{64}$'; then
    fail "judge: could not resolve sandbox image digest (got: $sandbox_digest)"
  fi
  log "judge: sandbox digest = $sandbox_digest"

  # ------------------------------------------------------------------
  # STEP 7: Approve the manifest.
  # ------------------------------------------------------------------
  local approved_by audit_id approved_at
  approved_by="$(gen_uuid)"
  audit_id="$(gen_uuid)"
  approved_at="$(python3 -c "import datetime; print(datetime.datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%S.000000000Z'))")"

  log "judge: approving manifest"
  # approve publishes with an exclusive hard link and refuses an existing path,
  # so a rerun has to clear the previous artifact first.
  rm -f "$manifest_path"
  "$judgify_bin" release-manifest approve \
    --draft "$draft_path" \
    --output "$manifest_path" \
    --sandbox-image-digest "$sandbox_digest" \
    --approved-by "$approved_by" \
    --audit-id "$audit_id" \
    --approved-at "$approved_at" >&2

  # ------------------------------------------------------------------
  # STEP 8: Verify the manifest.
  # ------------------------------------------------------------------
  log "judge: verifying manifest"
  "$judgify_bin" release-manifest verify \
    --manifest "$manifest_path" \
    --release-id "$release_id" \
    --sandbox-image-digest "$sandbox_digest" >&2
  log "judge: manifest verified"

  # ------------------------------------------------------------------
  # STEP 9: Build the judge-worker binary.
  # ------------------------------------------------------------------
  log "judge: building cmd/judge-worker"
  (
    cd "$api_dir"
    env GOPROXY=off go build -trimpath -o "$worker_bin" ./cmd/judge-worker
  )

  # ------------------------------------------------------------------
  # STEP 10: Build the capability inspector binary for the host.
  # The probe expects /usr/local/bin/judgify-capability-inspector on the host.
  # We build it and set JUDGIFY_CAPABILITY_INSPECTOR_PATH (if supported) or
  # install it temporarily at the expected path.
  # ------------------------------------------------------------------
  log "judge: building cmd/judgify-capability-inspector"
  (
    cd "$api_dir"
    env GOPROXY=off go build -trimpath -o "$inspector_bin" ./cmd/judgify-capability-inspector
  )
  # The probe hardcodes /usr/local/bin/judgify-capability-inspector.
  # We create a temporary wrapper if that path does not point to our binary.
  local inspector_installed=false
  if [ ! -f "/usr/local/bin/judgify-capability-inspector" ]; then
    if cp "$inspector_bin" /usr/local/bin/judgify-capability-inspector 2>/dev/null; then
      inspector_installed=true
      log "judge: capability inspector installed at /usr/local/bin/judgify-capability-inspector"
    else
      log "judge: WARNING: cannot install inspector at /usr/local/bin/ (no write permission)."
      log "judge:   The capability probe will fail at ProbeSandboxSemantics even if earlier"
      log "judge:   checks pass. Run: sudo cp $inspector_bin /usr/local/bin/judgify-capability-inspector"
    fi
  fi

  # ------------------------------------------------------------------
  # STEP 11: Get the real OrbStack socket path.
  # The worker config validation rejects /var/run/docker.sock by exact string.
  # OrbStack's real socket is at /Users/<user>/.orbstack/run/docker.sock.
  # ------------------------------------------------------------------
  local docker_socket=""
  if [ -S "$HOME/.orbstack/run/docker.sock" ]; then
    docker_socket="$HOME/.orbstack/run/docker.sock"
  elif [ -S "/run/user/$(id -u)/docker.sock" ]; then
    docker_socket="/run/user/$(id -u)/docker.sock"
  elif [ -S "/run/docker.sock" ]; then
    docker_socket="/run/docker.sock"
  else
    fail "judge: could not find a Docker Unix socket (tried ~/.orbstack/run/docker.sock, /run/user/UID/docker.sock, /run/docker.sock)"
  fi
  log "judge: using docker socket: $docker_socket"

  # ------------------------------------------------------------------
  # STEP 12: Construct the worker YAML config in the judge working dir.
  # ------------------------------------------------------------------
  local worker_apiroot="$judge_dir/workerroot"
  mkdir -p "$worker_apiroot/config" "$worker_apiroot/storages/logs" "$judge_staging_dir" "$judge_forge_dir"
  chmod 700 "$worker_apiroot"

  local src_config="$api_dir/config/local.yaml"
  local worker_config="$worker_apiroot/config/local.yaml"
  cp "$src_config" "$worker_config"
  chmod 644 "$worker_config"

  # Patch judge block: release_id, release_manifest_path, docker_image, docker_socket_path,
  # staging_root, api_authoring_catalog_mode (worker doesn't use this, but must be valid).
  # Use awk to rewrite known keys (bash 3.2 safe, no perl/python for secret handling).
  local patched_worker_config
  patched_worker_config="$(awk \
    -v rid="$release_id" \
    -v rmp="$manifest_path" \
    -v img="${sandbox_tag}@${sandbox_digest}" \
    -v sock="$docker_socket" \
    -v staging="$judge_staging_dir" \
    -v probe="127.0.0.1:${judge_worker_probe_port}" '
    /^[[:space:]]*release_id:/ && in_judge { printf "  release_id: \"%s\"\n", rid; next }
    /^[[:space:]]*release_manifest_path:/ && in_judge { printf "  release_manifest_path: \"%s\"\n", rmp; next }
    /^[[:space:]]*docker_image:/ && in_judge { printf "  docker_image: \"%s\"\n", img; next }
    /^[[:space:]]*docker_socket_path:/ && in_judge { printf "  docker_socket_path: \"%s\"\n", sock; next }
    /^[[:space:]]*staging_root:/ && in_judge { printf "  staging_root: \"%s\"\n", staging; next }
    /^[[:space:]]*worker_probe_address:/ { printf "  worker_probe_address: \"%s\"\n", probe; next }
    /^judge:/ { in_judge=1; print; next }
    /^[a-z]/ { in_judge=0 }
    { print }
  ' "$worker_config")"
  printf '%s\n' "$patched_worker_config" >"$worker_config"
  chmod 644 "$worker_config"

  local db_port
  db_port="$(read_env_value "$env_file" "STACK_DB_PORT")" || fail "judge: could not read STACK_DB_PORT"
  local worker_password
  worker_password="$(read_env_value "$env_file" "STACK_API_PASSWORD")" || fail "judge: could not read STACK_API_PASSWORD"

  # ------------------------------------------------------------------
  # STEP 13: Boot the judge-worker.
  # ------------------------------------------------------------------
  mkdir -p "$judge_forge_dir"
  log "judge: starting judge-worker (probe: http://127.0.0.1:${judge_worker_probe_port}/healthz)"
  (
    cd "$worker_apiroot"
    env \
      GO_ENV=local \
      DATABASE_DRIVER=postgres \
      DATABASE_HOST=127.0.0.1 \
      DATABASE_PORT="$db_port" \
      DATABASE_USERNAME=judgify_api \
      DATABASE_PASSWORD="$worker_password" \
      DATABASE_DATABASE=judgify \
      FORGE_DATA_DIR="$judge_forge_dir" \
      JUDGIFY_VERSION=e2e-judge \
      "$worker_bin" >>"$worker_log" 2>&1 &
    echo $!
  ) >"$worker_pid_file"
  local worker_pid
  worker_pid="$(cat "$worker_pid_file")"
  log "judge: worker started (pid $worker_pid)"

  # ------------------------------------------------------------------
  # STEP 14: Wait for the worker's /healthz probe.
  # ------------------------------------------------------------------
  log "judge: waiting for worker probe (up to 120 s)"
  local worker_ready=false
  local w_attempt=1
  while [ "$w_attempt" -le 60 ]; do
    local hc_code=""
    hc_code="$(curl --silent --max-time 3 -o /dev/null -w '%{http_code}' \
      "http://127.0.0.1:${judge_worker_probe_port}/healthz" 2>/dev/null)" || true
    if [ "$hc_code" = "200" ]; then
      worker_ready=true
      break
    fi
    if ! kill -0 "$worker_pid" 2>/dev/null; then
      # A docker_security refusal is the host failing the probe, not the product
      # failing. Name it explicitly so nobody is tempted to weaken the probe to
      # get a green run on a machine that cannot satisfy it.
      if grep -q 'activation denied: docker_security' "$worker_log" 2>/dev/null; then
        log "judge: capability probe refused this host at check 1 of 8 (docker_security)."
        log "judge:   ProbeDockerSecurity requires 'name=rootless' in docker info SecurityOptions."
        log "judge:   This daemon reports: $(docker info --format '{{json .SecurityOptions}}' 2>/dev/null)"
        log "judge:   OrbStack and Docker Desktop on macOS contain workloads in a Linux VM rather"
        log "judge:   than through rootless user namespaces, so the entry is absent by design."
        log "judge:   Checks 2-4 read /proc/self/cgroup and /sys/fs/cgroup, which macOS also lacks."
        log "judge:   Proving steps 11-13 needs a Linux host running rootless dockerd with cgroup v2."
        log "judge:   The probe is correct to refuse; it must not be relaxed to pass here."
        fail "judge: host cannot satisfy the sandbox capability probe"
      fi
      log "judge: worker process died; last log lines:"
      tail -50 "$worker_log" >&2 2>/dev/null || true
      fail "judge: worker exited before becoming ready"
    fi
    sleep 2
    w_attempt=$((w_attempt + 1))
  done
  if [ "$worker_ready" != "true" ]; then
    log "judge: worker did not become ready. Last 50 log lines:"
    tail -50 "$worker_log" >&2 2>/dev/null || true
    kill -TERM "$worker_pid" 2>/dev/null || true
    fail "judge: worker never became ready"
  fi
  log "judge: worker is ready"

  # ------------------------------------------------------------------
  # STEP 15: Submit a correct solution via the member session.
  # Re-login as the seeded member and submit to the seeded published problem.
  # The seeded problem uses checker_key=token and runtime_key=<first available>.
  # We submit a correct "print N" solution in cpp17.
  # ------------------------------------------------------------------
  local problem_id
  problem_id="$(read_env_value "$env_file" "STACK_SEED_PUBLIC_PROBLEM_ID")" || fail "judge: could not read STACK_SEED_PUBLIC_PROBLEM_ID"

  local member_user member_pass
  member_user="$(read_env_value "$env_file" "JUDGIFY_E2E_USER_USERNAME")" || fail "judge: missing JUDGIFY_E2E_USER_USERNAME"
  member_pass="$(read_env_value "$env_file" "JUDGIFY_E2E_USER_PASSWORD")" || fail "judge: missing JUDGIFY_E2E_USER_PASSWORD"

  # Use a separate cookie jar for member judge session.
  local judge_cookies="$judge_dir/judge.cookies"
  : >"$judge_cookies"
  chmod 600 "$judge_cookies"

  local req="$judge_dir/req.json"
  local rsp="$judge_dir/rsp.json"
  local hdr="$judge_dir/hdr.txt"

  # Helper: write headers file for judge flow (different cookie jar via sed).
  judge_write_hdrs() {
    local csrf="$1" ikey="$2"
    : >"$hdr"
    [ -n "$csrf" ] && printf 'X-CSRF-Token: %s\n' "$csrf" >>"$hdr"
    [ -n "$ikey" ] && printf 'Idempotency-Key: %s\n' "$ikey" >>"$hdr"
  }

  # Helper: HTTP call using the judge cookie jar.
  judge_http() {
    local method="$1" path="$2"
    local hdr_file="${3:-}"
    local body_file="${4:-}"
    local out_file="${5:-$judge_dir/response.json}"

    local base_url
    base_url="$(read_env_value "$env_file" "STACK_API_BASE_URL")" || base_url="http://127.0.0.1:8000"

    local args=()
    args+=(--silent --show-error --max-time 30)
    args+=(-X "$method")
    args+=(-H "Content-Type: application/json")
    args+=(-H "Origin: http://localhost:3000")
    args+=(--cookie "$judge_cookies")
    args+=(--cookie-jar "$judge_cookies")

    if [ -n "$hdr_file" ] && [ -f "$hdr_file" ]; then
      while IFS= read -r hdr_line; do
        [ -n "$hdr_line" ] && args+=(-H "$hdr_line")
      done <"$hdr_file"
    fi
    [ -n "$body_file" ] && [ -f "$body_file" ] && args+=(--data-binary "@$body_file")
    args+=(-o "$out_file")
    args+=(-w '%{http_code}')

    local status=""
    local attempt=1
    local delay=2
    while :; do
      status="$(curl "${args[@]}" "$base_url$path" 2>&1)" || {
        log "judge: curl error for $method $path"
        return 1
      }
      if [ "$status" != "429" ] || [ "$attempt" -ge 6 ]; then
        break
      fi
      sleep "$delay"
      delay=$((delay * 2))
      attempt=$((attempt + 1))
    done
    printf '%s' "$status"
  }

  judge_get_csrf() {
    awk '/judgify_csrf/{print $NF}' "$judge_cookies" | tail -1
  }

  # --- Member login ---
  log "judge: logging in as member"
  local ikey csrf sc
  ikey="$(gen_uuid)"
  judge_write_hdrs "" "$ikey"
  printf '{}' >"$req"
  sc="$(judge_http POST /auth/pre-session "$hdr" "$req" "$rsp")"
  [ "$sc" = "204" ] || fail "judge: member pre-session failed (HTTP $sc): $(cat "$rsp")"

  csrf="$(judge_get_csrf)"
  [ -n "$csrf" ] || fail "judge: no CSRF after member pre-session"

  ikey="$(gen_uuid)"
  judge_write_hdrs "$csrf" "$ikey"
  python3 -c "
import json, sys
print(json.dumps({'username': sys.argv[1], 'password': sys.argv[2]}))
" "$member_user" "$member_pass" >"$req"
  sc="$(judge_http POST /auth/login "$hdr" "$req" "$rsp")"
  [ "$sc" = "200" ] || fail "judge: member login failed (HTTP $sc): $(cat "$rsp")"

  csrf="$(judge_get_csrf)"
  [ -n "$csrf" ] || fail "judge: no CSRF after member login"
  log "judge: member session established"

  # ------------------------------------------------------------------
  # STEP 16: Submit a CORRECT solution: print N for input N.
  # The seeded problem uses token checker and cpp17 runtime.
  # ------------------------------------------------------------------
  log "judge: submitting correct solution"
  ikey="$(gen_uuid)"
  judge_write_hdrs "$csrf" "$ikey"
  awk -v pid="$problem_id" -v ikey="$ikey" 'BEGIN{
    src = "#include<cstdio>\nint main(){int n;scanf(\"%d\",&n);printf(\"%d\\n\",n);}"
    printf "{\"problem_id\":\"%s\",\"runtime_key\":\"cpp17\",\"source_code\":\"%s\"}", pid, src
  }' >"$req"
  sc="$(judge_http POST /submissions "$hdr" "$req" "$rsp")"
  if [ "$sc" != "202" ] && [ "$sc" != "200" ]; then
    log "judge: submit correct solution failed (HTTP $sc): $(cat "$rsp")"
    if [ "$sc" = "503" ]; then
      log "judge: 503 = no active judge capability. Worker may not have activated capabilities yet."
      log "judge: Worker probe was healthy, but capability activation in DB may be pending."
    fi
    kill -TERM "$worker_pid" 2>/dev/null || true
    fail "judge: correct submission intake failed"
  fi
  local correct_sub_id
  correct_sub_id="$(json_field "$rsp" "id")"
  [ -n "$correct_sub_id" ] || fail "judge: could not extract submission id from intake response"
  log "judge: correct submission accepted (id=$correct_sub_id)"

  # ------------------------------------------------------------------
  # STEP 17: Poll for the verdict (correct submission).
  # ------------------------------------------------------------------
  log "judge: polling verdict for correct submission $correct_sub_id"
  local correct_verdict="" poll_i=1
  while [ "$poll_i" -le 90 ]; do
    sc="$(judge_http GET "/submissions/$correct_sub_id" "" "" "$rsp")"
    if [ "$sc" = "200" ]; then
      correct_verdict="$(json_field "$rsp" "status")"
      case "$correct_verdict" in
        accepted|wrong_answer|compile_error|runtime_error|time_limit_exceeded|memory_limit_exceeded|process_limit_exceeded|output_limit_exceeded|system_error)
          break
          ;;
      esac
    fi
    sleep 2
    poll_i=$((poll_i + 1))
  done
  log "judge: CORRECT SOLUTION VERDICT: $correct_verdict"
  local correct_passed correct_total
  correct_passed="$(json_field "$rsp" "passed_count")"
  correct_total="$(json_field "$rsp" "total_count")"
  log "judge:   passed=$correct_passed / total=$correct_total"

  # ------------------------------------------------------------------
  # STEP 18: Submit a WRONG solution: always prints 0.
  # ------------------------------------------------------------------
  log "judge: submitting wrong solution"
  ikey="$(gen_uuid)"
  judge_write_hdrs "$csrf" "$ikey"
  awk -v pid="$problem_id" -v ikey="$ikey" 'BEGIN{
    src = "#include<cstdio>\nint main(){printf(\"0\\n\");}"
    printf "{\"problem_id\":\"%s\",\"runtime_key\":\"cpp17\",\"source_code\":\"%s\"}", pid, src
  }' >"$req"
  sc="$(judge_http POST /submissions "$hdr" "$req" "$rsp")"
  if [ "$sc" != "202" ] && [ "$sc" != "200" ]; then
    log "judge: submit wrong solution failed (HTTP $sc): $(cat "$rsp")"
    kill -TERM "$worker_pid" 2>/dev/null || true
    fail "judge: wrong submission intake failed"
  fi
  local wrong_sub_id
  wrong_sub_id="$(json_field "$rsp" "id")"
  [ -n "$wrong_sub_id" ] || fail "judge: could not extract wrong submission id"
  log "judge: wrong submission accepted (id=$wrong_sub_id)"

  # ------------------------------------------------------------------
  # STEP 19: Poll for the verdict (wrong submission).
  # ------------------------------------------------------------------
  log "judge: polling verdict for wrong submission $wrong_sub_id"
  local wrong_verdict="" wpoll_i=1
  while [ "$wpoll_i" -le 90 ]; do
    sc="$(judge_http GET "/submissions/$wrong_sub_id" "" "" "$rsp")"
    if [ "$sc" = "200" ]; then
      wrong_verdict="$(json_field "$rsp" "status")"
      case "$wrong_verdict" in
        accepted|wrong_answer|compile_error|runtime_error|time_limit_exceeded|memory_limit_exceeded|process_limit_exceeded|output_limit_exceeded|system_error)
          break
          ;;
      esac
    fi
    sleep 2
    wpoll_i=$((wpoll_i + 1))
  done
  log "judge: WRONG SOLUTION VERDICT: $wrong_verdict"
  local wrong_passed wrong_total
  wrong_passed="$(json_field "$rsp" "passed_count")"
  wrong_total="$(json_field "$rsp" "total_count")"
  log "judge:   passed=$wrong_passed / total=$wrong_total"

  # ------------------------------------------------------------------
  # STEP 20: Stop the worker.
  # ------------------------------------------------------------------
  log "judge: stopping worker"
  if kill -0 "$worker_pid" 2>/dev/null; then
    kill -TERM "$worker_pid" 2>/dev/null || true
    local si=1
    while [ "$si" -le 10 ] && kill -0 "$worker_pid" 2>/dev/null; do
      sleep 1
      si=$((si + 1))
    done
    kill -KILL "$worker_pid" 2>/dev/null || true
    log "judge: worker stopped"
  fi
  rm -f "$worker_pid_file"

  if [ "$inspector_installed" = "true" ]; then
    rm -f /usr/local/bin/judgify-capability-inspector 2>/dev/null || true
  fi

  # ------------------------------------------------------------------
  # STEP 21: Print summary.
  # ------------------------------------------------------------------
  printf '\njudge summary:\n'
  printf '  correct solution: verdict=%s  passed=%s/%s\n' \
    "$correct_verdict" "$correct_passed" "$correct_total"
  printf '  wrong solution:   verdict=%s  passed=%s/%s\n' \
    "$wrong_verdict" "$wrong_passed" "$wrong_total"
  printf '\n'

  if [ "$correct_verdict" = "accepted" ] && [ "$wrong_verdict" = "wrong_answer" ]; then
    log "judge: PASS — correct solution accepted, wrong solution rejected"
  else
    log "judge: UNEXPECTED VERDICTS (correct=$correct_verdict wrong=$wrong_verdict)"
    fail "judge: expected accepted + wrong_answer, got $correct_verdict + $wrong_verdict"
  fi
}

# ===========================================================================
# Dispatch
# ===========================================================================
subcommand="${1:-}"
case "$subcommand" in
  up)     cmd_up ;;
  down)   cmd_down ;;
  env)    cmd_env ;;
  status) cmd_status ;;
  judge)  cmd_judge ;;
  *)
    printf 'Usage: %s {up|down|env|status|judge}\n' "$0" >&2
    exit 1
    ;;
esac
