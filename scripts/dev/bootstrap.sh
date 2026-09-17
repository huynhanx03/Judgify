#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
root_env="$root/.env"
api_env="$root/api/.env"
cert_dir="$root/api/certs"

umask 077

command -v openssl >/dev/null 2>&1 || {
  echo "development bootstrap: openssl is required" >&2
  exit 2
}

read_env_value() {
  local file="$1"
  local key="$2"
  [[ -f "$file" ]] || return 1
  awk -F= -v key="$key" '
    $1 == key {
      sub(/^[^=]*=/, "")
      sub(/\r$/, "")
      print
      exit
    }
  ' "$file"
}

safe_local_secret() {
  [[ "$1" =~ ^[A-Za-z0-9._~-]{16,256}$ ]]
}

root_password="$(read_env_value "$root_env" POSTGRES_PASSWORD || true)"
api_password="$(read_env_value "$api_env" DATABASE_PASSWORD || true)"

if [[ -n "$root_password" ]] && ! safe_local_secret "$root_password"; then
  echo "development bootstrap: existing POSTGRES_PASSWORD is not a simple local secret" >&2
  exit 2
fi
if [[ -n "$api_password" ]] && ! safe_local_secret "$api_password"; then
  echo "development bootstrap: existing DATABASE_PASSWORD is not a simple local secret" >&2
  exit 2
fi
if [[ -n "$root_password" && -n "$api_password" && "$root_password" != "$api_password" ]]; then
  echo "development bootstrap: Compose and API database passwords do not match" >&2
  exit 2
fi

database_password="${root_password:-${api_password:-}}"
if [[ -z "$database_password" ]]; then
  database_password="$(openssl rand -hex 24)"
fi

if [[ ! -e "$root_env" ]]; then
  temporary="$(mktemp "$root/.env.XXXXXX")"
  {
    printf '%s\n' 'POSTGRES_USER=judgify_dev'
    printf 'POSTGRES_PASSWORD=%s\n' "$database_password"
    printf '%s\n' 'POSTGRES_DB=judgify' 'POSTGRES_PORT=5432'
  } >"$temporary"
  chmod 600 "$temporary"
  mv "$temporary" "$root_env"
fi

if [[ ! -e "$api_env" ]]; then
  jwt_secret="$(openssl rand -hex 32)"
  temporary="$(mktemp "$root/api/.env.XXXXXX")"
  {
    printf '%s\n' 'GO_ENV=local' 'JUDGIFY_VERSION=development'
    printf '%s\n' 'DATABASE_HOST=localhost' 'DATABASE_PORT=5432'
    printf '%s\n' 'DATABASE_USERNAME=judgify_dev'
    printf 'DATABASE_PASSWORD=%s\n' "$database_password"
    printf '%s\n' 'DATABASE_DATABASE=judgify'
    printf 'JWT_SECRET=%s\n' "$jwt_secret"
    printf '%s\n' \
      'GOOGLE_CLIENT_ID=local-google-client-id' \
      'GOOGLE_CLIENT_SECRET=local-google-client-secret' \
      'GITHUB_CLIENT_ID=local-github-client-id' \
      'GITHUB_CLIENT_SECRET=local-github-client-secret' \
      'RESEND_API_KEY=local-resend-placeholder' \
      'RESEND_FROM_EMAIL=security@localhost.invalid' \
      'RESEND_FROM_NAME=Judgify' \
      'RESEND_RESET_URL=http://localhost:3000/reset-password' \
      'RESEND_VERIFICATION_URL=http://localhost:3000/verify-recovery-contact' \
      'RESEND_INVITATION_URL=http://localhost:3000/accept-invitation'
  } >"$temporary"
  chmod 600 "$temporary"
  mv "$temporary" "$api_env"
fi

mkdir -p "$cert_dir" "$root/api/storages/logs"
chmod 700 "$cert_dir" "$root/api/storages" "$root/api/storages/logs"

private_key="$cert_dir/private_key.pem"
public_key="$cert_dir/public_key.pem"
if [[ ! -e "$private_key" && -e "$public_key" ]]; then
  echo "development bootstrap: public JWT key exists without its private key" >&2
  exit 2
fi
if [[ ! -e "$private_key" ]]; then
  private_temporary="$(mktemp "$cert_dir/.private-key.XXXXXX")"
  public_temporary="$(mktemp "$cert_dir/.public-key.XXXXXX")"
  trap 'rm -f "${private_temporary:-}" "${public_temporary:-}"' EXIT
  openssl genpkey -algorithm RSA \
    -pkeyopt rsa_keygen_bits:3072 \
    -out "$private_temporary" >/dev/null 2>&1
  openssl pkey -in "$private_temporary" \
    -pubout -out "$public_temporary" >/dev/null 2>&1
  chmod 600 "$private_temporary"
  chmod 644 "$public_temporary"
  mv "$private_temporary" "$private_key"
  mv "$public_temporary" "$public_key"
  trap - EXIT
elif [[ ! -e "$public_key" ]]; then
  public_temporary="$(mktemp "$cert_dir/.public-key.XXXXXX")"
  trap 'rm -f "${public_temporary:-}"' EXIT
  openssl pkey -in "$private_key" \
    -pubout -out "$public_temporary" >/dev/null 2>&1
  chmod 644 "$public_temporary"
  mv "$public_temporary" "$public_key"
  trap - EXIT
fi

printf '%s\n' \
  "development bootstrap: local environment files are ready" \
  "development bootstrap: local JWT key pair is ready" \
  "development bootstrap: API uses the non-executable code-owned authoring catalog" \
  "development bootstrap: replace OAuth/Resend placeholders only for those journeys"
