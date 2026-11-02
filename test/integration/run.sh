#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
compose_file="$repository_root/test/compose.integration.yml"
port="${JUDGIFY_TEST_POSTGRES_PORT:-55432}"
export JUDGIFY_TEST_DATABASE_URL="postgres://judgify_test:judgify_test@127.0.0.1:${port}/judgify_test?sslmode=disable"

if [[ "${JUDGIFY_TEST_KEEP_DATABASE:-0}" != "1" ]]; then
	# A normal test run is isolated and starts from the checked-in migration
	# chain. Keep mode deliberately skips this reset for local investigation.
	docker compose -f "$compose_file" down --volumes --remove-orphans
fi
docker compose -f "$compose_file" up -d --wait
if [[ "${JUDGIFY_TEST_KEEP_DATABASE:-0}" != "1" ]]; then
	trap 'docker compose -f "$compose_file" down --volumes' EXIT
fi

cd "$repository_root/api"
GO_ENV=local \
DATABASE_HOST=127.0.0.1 \
DATABASE_PORT="$port" \
DATABASE_USERNAME=judgify_test \
DATABASE_PASSWORD=judgify_test \
DATABASE_DATABASE=judgify_test \
	go run ./cmd/migrate
go test -tags=integration ./...
