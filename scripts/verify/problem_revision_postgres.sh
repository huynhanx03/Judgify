#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
. "$root_dir/scripts/lib/operations.sh"
api_dir="$root_dir/api"
role_bootstrap="$root_dir/deploy/postgres/init-roles.sql"
container_name="judgify-problem-verify-$$-${RANDOM}"
work_dir="$(mktemp -d "$(ops_temp_root)/judgify-problem-verify.XXXXXX")"
migrator_binary="$work_dir/judgify-migrate"

database_admin_user="postgres"
database_admin_password="verifyadmin${RANDOM}${RANDOM}"
database_name="judgify"
migrator_password="verifymigrator${RANDOM}${RANDOM}"
api_password="verifyapi${RANDOM}${RANDOM}"
worker_password="verifyworker${RANDOM}${RANDOM}"

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT

fail() {
  echo "problem revision PostgreSQL verification failed: $*" >&2
  exit 1
}

docker run \
  --pull never \
  --rm \
  --detach \
  --name "$container_name" \
  --env "POSTGRES_USER=$database_admin_user" \
  --env "POSTGRES_PASSWORD=$database_admin_password" \
  --env "POSTGRES_DB=$database_name" \
  --publish "127.0.0.1::5432" \
  postgres:16 >/dev/null

ready=false
for _ in {1..60}; do
  if docker logs "$container_name" 2>&1 |
      grep -Fq 'PostgreSQL init process complete; ready for start up.' &&
    docker exec "$container_name" pg_isready \
      --username "$database_admin_user" \
      --dbname "$database_name" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != "true" ]]; then
  fail "temporary PostgreSQL did not become ready"
fi

host_port="$(
  docker port "$container_name" 5432/tcp |
    awk -F: 'NR == 1 { print $NF }'
)"
if [[ ! "$host_port" =~ ^[0-9]+$ ]]; then
  fail "could not resolve temporary PostgreSQL port"
fi

docker exec \
  --interactive \
  --env "POSTGRES_DB=$database_name" \
  --env "JUDGIFY_MIGRATOR_PASSWORD=$migrator_password" \
  --env "JUDGIFY_API_PASSWORD=$api_password" \
  --env "JUDGIFY_WORKER_PASSWORD=$worker_password" \
  "$container_name" \
  psql \
  --no-psqlrc \
  --set ON_ERROR_STOP=1 \
  --username "$database_admin_user" \
  --dbname "$database_name" \
  <"$role_bootstrap" >/dev/null

(
  cd "$api_dir"
  env GOCACHE="${GOCACHE:-/private/tmp/judgify-go-cache}" GOPROXY=off \
    go build -trimpath -o "$migrator_binary" ./cmd/migrate
)

(
  cd "$api_dir"
  env \
    GO_ENV=local \
    DATABASE_DRIVER=postgres \
    DATABASE_HOST=127.0.0.1 \
    DATABASE_PORT="$host_port" \
    DATABASE_USERNAME=judgify_migrator \
    DATABASE_PASSWORD="$migrator_password" \
    DATABASE_DATABASE="$database_name" \
    LOGGER_FILE_LOG_NAME="$work_dir/migrate.log" \
    "$migrator_binary"
)

(
  cd "$api_dir"
  env \
    GOCACHE="${GOCACHE:-/private/tmp/judgify-go-cache}" \
    GOPROXY=off \
    JUDGIFY_TEST_DATABASE_URL="postgres://judgify_api:$api_password@127.0.0.1:$host_port/$database_name?sslmode=disable&search_path=public" \
    go test ./internal/problem/adapters/driven/db \
      -run '^Test(RevisionRepositoryPostgresPersistsAndDeduplicatesCompleteDraftGraph|Taxonomy(RepositoryPostgresWritesImmutableHistoryAndRejectsMalformedFacts|CatalogPostgresKeepsHistoricalRewardSnapshotAfterUpdatesAndSoftDelete|CatalogPostgresResolutionIsWholeBeforeOrAfterConcurrentUpdate)|ProblemRepositoryPostgres(RejectsSequentialDuplicateSlug|SerializesConcurrentCanonicalEquivalentSlug|DoesNotReuseSlugAfterSoftDelete))$' \
      -count=1
)

echo "problem revision PostgreSQL verification: ok"
