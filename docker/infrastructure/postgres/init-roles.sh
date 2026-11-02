#!/bin/sh
set -eu

export JUDGIFY_API_PASSWORD="$(cat /run/secrets/postgres_api_password)"
export JUDGIFY_WORKER_PASSWORD="$(cat /run/secrets/postgres_worker_password)"
export JUDGIFY_MIGRATOR_PASSWORD="$(cat /run/secrets/postgres_migrator_password)"

psql \
  --no-psqlrc \
  --set ON_ERROR_STOP=1 \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" \
  --file /opt/judgify/init-roles.sql
