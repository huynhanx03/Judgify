API_DIR      := ./api
REL_MIGRATE_DIR := migration
DB_URL       ?=
DEV_URL      ?= docker://postgres/16/dev?search_path=public
WEBSITE_DIR := ./website
DOCKER       ?= docker
COMPOSE      := $(DOCKER) compose
GO_COMMON_CONTEXT ?= ../go-common
VERSION      ?= dev
VCS_REF      ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
SOURCE_DATE_EPOCH ?= $(shell git log -1 --format=%ct 2>/dev/null || printf 0)
BUILD_DATE   ?= $(shell git log -1 --format=%cI 2>/dev/null || printf 1970-01-01T00:00:00Z)
GO_COMMON_VCS_REF ?= $(shell git -C $(GO_COMMON_CONTEXT) rev-parse HEAD 2>/dev/null || printf unknown)
API_IMAGE      ?= judgify/api:$(VERSION)
WORKER_IMAGE   ?= judgify/worker:$(VERSION)
MIGRATE_IMAGE  ?= judgify/migrate:$(VERSION)
WEBSITE_IMAGE  ?= judgify/website:$(VERSION)
SANDBOX_IMAGE  ?= judgify/sandbox:$(VERSION)

include docker/sandbox/build.mk

OCI_BUILD_ARGS := \
	--build-arg VERSION=$(VERSION) \
	--build-arg VCS_REF=$(VCS_REF) \
	--build-arg SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) \
	--build-arg BUILD_DATE=$(BUILD_DATE)

API_BUILD_ARGS := \
	--file api/Dockerfile \
	--build-context go_common=$(GO_COMMON_CONTEXT) \
	--build-arg GO_COMMON_VCS_REF=$(GO_COMMON_VCS_REF)

.PHONY: generate generate-ent
generate: generate-ent mocks

generate-ent:
	cd $(API_DIR) && go generate ./...

.PHONY: dev-bootstrap local-setup run-local
dev-bootstrap:
	bash api/dev-bootstrap.sh

local-setup: dev-bootstrap
	$(COMPOSE) up --detach --wait postgres
	$(COMPOSE) --profile setup run --build --rm --no-deps migrate
	$(COMPOSE) --profile setup run --build --rm --no-deps initialize

run-local:
	$(COMPOSE) up --build

.PHONY: docker-build-api docker-build-worker docker-build-migrate docker-build-website docker-build-sandbox docker-build

docker-build-api:
	$(DOCKER) buildx build --load \
		$(API_BUILD_ARGS) \
		--target api \
		$(OCI_BUILD_ARGS) \
		--tag $(API_IMAGE) .

docker-build-worker:
	$(DOCKER) buildx build --load \
		$(API_BUILD_ARGS) \
		--target worker \
		$(OCI_BUILD_ARGS) \
		--tag $(WORKER_IMAGE) .

docker-build-migrate:
	$(DOCKER) buildx build --load \
		$(API_BUILD_ARGS) \
		--target migrate \
		$(OCI_BUILD_ARGS) \
		--tag $(MIGRATE_IMAGE) .

docker-build-website:
	$(DOCKER) buildx build --load \
		--file website/Dockerfile \
		--target runtime \
		--build-arg NEXT_PUBLIC_API_URL=/api \
		$(OCI_BUILD_ARGS) \
		--tag $(WEBSITE_IMAGE) .

docker-build-sandbox:
	$(DOCKER) buildx build --load \
		--file docker/sandbox/Dockerfile \
		$(SANDBOX_RUNTIME_BUILD_ARGS) \
		$(OCI_BUILD_ARGS) \
		--tag $(SANDBOX_IMAGE) .

docker-build: docker-build-api docker-build-worker docker-build-migrate docker-build-website docker-build-sandbox

.PHONY: docker-config verify-topology verify-packaging

docker-config:
	$(COMPOSE) \
		--env-file deploy/env.production.example \
		--file deploy/compose.production.yml \
		config

verify-topology:
	scripts/verify/topology_test.sh

verify-packaging: verify-topology docker-config

# Migrations
.PHONY: migrate-diff
migrate-diff:
	@name=$(name); \
	if [ -z "$$name" ]; then name="change"; fi; \
	cd $(API_DIR) && atlas migrate diff $$name \
		--env local \
		--var "dev_url=$(DEV_URL)"

.PHONY: migrate-clean
migrate-clean:
	rm -f $(API_DIR)/$(REL_MIGRATE_DIR)/*.sql $(API_DIR)/$(REL_MIGRATE_DIR)/atlas.sum

.PHONY: migrate-apply
migrate-apply:
	cd $(API_DIR) && go run ./cmd/migrate

.PHONY: migrate-status
migrate-status:
	@test -n "$(DB_URL)" || { echo "DB_URL is required for migrate-status" >&2; exit 2; }
	cd $(API_DIR) && atlas migrate status \
		--dir "file://$(REL_MIGRATE_DIR)" \
		--url "$(DB_URL)"

.PHONY: mocks
mocks:
	bash $(API_DIR)/scripts/generate-mocks.sh

# NOTE: tx.Manager (go-common) is not mocked. Tests use testutil.TxManager, an
# inline pass-through Manager — see internal/testutil/tx.go.

# ============================================================
# Testing
# ============================================================

.PHONY: test-api test-common test-web test-integration test-integration-keep test-integration-down verify-generated verify-architecture verify-architecture-strict verify-database-baseline verify-migrations verify-integration verify-go verify-frontend verify-contracts verify-supply-chain verify verify-release

test-api:
	cd $(API_DIR) && go test ./...

test-integration:
	bash test/integration/run.sh

test-integration-keep:
	JUDGIFY_TEST_KEEP_DATABASE=1 bash test/integration/run.sh

test-integration-down:
	$(COMPOSE) -f test/compose.integration.yml down --volumes

test-common:
	cd ../go-common && go test ./...

test-web:
	cd $(WEBSITE_DIR) && npm run lint && npm run build

verify-generated:
	bash scripts/verify/generated.sh

verify-architecture:
	cd $(API_DIR) && go test ./internal/architecture -count=1

verify-architecture-strict:
	cd $(API_DIR) && JUDGIFY_ARCH_STRICT=1 go test ./internal/architecture -count=1

verify-database-baseline:
	bash scripts/verify/database_baseline_test.sh
	scripts/verify/database_baseline.sh

verify-migrations:
	scripts/verify/migrations.sh --check

verify-integration: verify-database-baseline
	@test -n "$(JUDGIFY_BACKUP_RESTORE_DRIVER)" || { echo "JUDGIFY_BACKUP_RESTORE_DRIVER is required for destructive backup/restore integration" >&2; exit 2; }
	test/operations/backup_restore_test.sh

verify-go:
	scripts/verify/go.sh

verify-frontend:
	scripts/verify/frontend.sh

verify-contracts:
	scripts/verify/contracts.sh

verify-supply-chain:
	scripts/verify/supply_chain.sh --check

verify: verify-go verify-frontend verify-contracts

verify-release:
	JUDGIFY_VERIFY_MODE=release scripts/verify/go.sh
	JUDGIFY_VERIFY_MODE=release scripts/verify/frontend.sh
	scripts/verify/contracts.sh
	scripts/verify/migrations.sh --integration
	scripts/verify/supply_chain.sh --check
	scripts/verify/supply_chain.sh --evidence

.PHONY: test
test:
	cd $(API_DIR) && go test ./internal/.../service/... -v -count=1

.PHONY: test-all
test-all:
	cd $(API_DIR) && go test ./... -v -count=1

.PHONY: test-coverage
test-coverage:
	cd $(API_DIR) && go test ./internal/.../service/... -cover -coverprofile=coverage.out
	cd $(API_DIR) && go tool cover -func=coverage.out
