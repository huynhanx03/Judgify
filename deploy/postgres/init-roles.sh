#!/bin/sh
set -eu

read_secret() {
  variable_name="$1"
  file_path="$2"
  if [ ! -r "${file_path}" ]; then
    echo "postgres role bootstrap: secret is not readable: ${file_path}" >&2
    exit 1
  fi
  value="$(cat "${file_path}")"
  if [ -z "${value}" ]; then
    echo "postgres role bootstrap: secret is empty: ${file_path}" >&2
    exit 1
  fi
  export "${variable_name}=${value}"
}

read_secret JUDGIFY_API_PASSWORD /run/secrets/postgres_api_password
read_secret JUDGIFY_WORKER_PASSWORD /run/secrets/postgres_worker_password
read_secret JUDGIFY_MIGRATOR_PASSWORD /run/secrets/postgres_migrator_password

psql \
  --set ON_ERROR_STOP=1 \
  --username "${POSTGRES_USER}" \
  --dbname "${POSTGRES_DB}" \
  --file /opt/judgify/init-roles.sql

unset JUDGIFY_API_PASSWORD JUDGIFY_WORKER_PASSWORD JUDGIFY_MIGRATOR_PASSWORD
