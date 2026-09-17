API_DIR      := ./api
SERVER_MAIN  := $(API_DIR)/cmd/server/main.go
REL_MIGRATE_DIR := internal/ent/migrate/migrations
DB_URL       ?=
DEV_URL      ?= docker://postgres/16/dev?search_path=public
POSTGRES_USER ?= judgify_dev
POSTGRES_DB   ?= judgify
WEBSITE_DIR := ./website
MOCKGEN_COMMAND ?= go run go.uber.org/mock/mockgen
MOCKGEN      := cd $(API_DIR) && $(MOCKGEN_COMMAND)
PORTS_DIR    := internal
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

# Production release tooling writes this immutable Make fragment from the
# code-owned runtime catalog. Local builds may omit it, but such an image has
# empty runtime labels and is intentionally incapable of passing a worker
# release-manifest probe.
SANDBOX_RELEASE_ENV ?=
ifneq ($(strip $(SANDBOX_RELEASE_ENV)),)
include $(SANDBOX_RELEASE_ENV)
endif

# Release automation supplies these approved identities. Empty local defaults
# deliberately build an image that cannot pass the production manifest probe.
CPP17_PROFILE_VERSION ?=
CPP17_COMPILER_VERSION ?=
CPP17_RUNTIME_BOOTSTRAP_VERSION ?=
CPP17_COMPILE_COMMAND_ID ?=
CPP17_RUN_COMMAND_ID ?=
CPP17_MANIFEST_VERSION ?=
CPP17_MANIFEST_CHECKSUM ?=
PYTHON3_PROFILE_VERSION ?=
PYTHON3_COMPILER_VERSION ?=
PYTHON3_RUNTIME_BOOTSTRAP_VERSION ?=
PYTHON3_COMPILE_COMMAND_ID ?=
PYTHON3_RUN_COMMAND_ID ?=
PYTHON3_MANIFEST_VERSION ?=
PYTHON3_MANIFEST_CHECKSUM ?=
JAVA17_PROFILE_VERSION ?=
JAVA17_COMPILER_VERSION ?=
JAVA17_RUNTIME_BOOTSTRAP_VERSION ?=
JAVA17_COMPILE_COMMAND_ID ?=
JAVA17_RUN_COMMAND_ID ?=
JAVA17_MANIFEST_VERSION ?=
JAVA17_MANIFEST_CHECKSUM ?=
GO_PROFILE_VERSION ?=
GO_COMPILER_VERSION ?=
GO_RUNTIME_BOOTSTRAP_VERSION ?=
GO_COMPILE_COMMAND_ID ?=
GO_RUN_COMMAND_ID ?=
GO_MANIFEST_VERSION ?=
GO_MANIFEST_CHECKSUM ?=

SANDBOX_RUNTIME_BUILD_ARGS := \
	--build-arg CPP17_PROFILE_VERSION=$(CPP17_PROFILE_VERSION) \
	--build-arg CPP17_COMPILER_VERSION=$(CPP17_COMPILER_VERSION) \
	--build-arg CPP17_RUNTIME_BOOTSTRAP_VERSION=$(CPP17_RUNTIME_BOOTSTRAP_VERSION) \
	--build-arg CPP17_COMPILE_COMMAND_ID=$(CPP17_COMPILE_COMMAND_ID) \
	--build-arg CPP17_RUN_COMMAND_ID=$(CPP17_RUN_COMMAND_ID) \
	--build-arg CPP17_MANIFEST_VERSION=$(CPP17_MANIFEST_VERSION) \
	--build-arg CPP17_MANIFEST_CHECKSUM=$(CPP17_MANIFEST_CHECKSUM) \
	--build-arg PYTHON3_PROFILE_VERSION=$(PYTHON3_PROFILE_VERSION) \
	--build-arg PYTHON3_COMPILER_VERSION=$(PYTHON3_COMPILER_VERSION) \
	--build-arg PYTHON3_RUNTIME_BOOTSTRAP_VERSION=$(PYTHON3_RUNTIME_BOOTSTRAP_VERSION) \
	--build-arg PYTHON3_COMPILE_COMMAND_ID=$(PYTHON3_COMPILE_COMMAND_ID) \
	--build-arg PYTHON3_RUN_COMMAND_ID=$(PYTHON3_RUN_COMMAND_ID) \
	--build-arg PYTHON3_MANIFEST_VERSION=$(PYTHON3_MANIFEST_VERSION) \
	--build-arg PYTHON3_MANIFEST_CHECKSUM=$(PYTHON3_MANIFEST_CHECKSUM) \
	--build-arg JAVA17_PROFILE_VERSION=$(JAVA17_PROFILE_VERSION) \
	--build-arg JAVA17_COMPILER_VERSION=$(JAVA17_COMPILER_VERSION) \
	--build-arg JAVA17_RUNTIME_BOOTSTRAP_VERSION=$(JAVA17_RUNTIME_BOOTSTRAP_VERSION) \
	--build-arg JAVA17_COMPILE_COMMAND_ID=$(JAVA17_COMPILE_COMMAND_ID) \
	--build-arg JAVA17_RUN_COMMAND_ID=$(JAVA17_RUN_COMMAND_ID) \
	--build-arg JAVA17_MANIFEST_VERSION=$(JAVA17_MANIFEST_VERSION) \
	--build-arg JAVA17_MANIFEST_CHECKSUM=$(JAVA17_MANIFEST_CHECKSUM) \
	--build-arg GO_PROFILE_VERSION=$(GO_PROFILE_VERSION) \
	--build-arg GO_COMPILER_VERSION=$(GO_COMPILER_VERSION) \
	--build-arg GO_RUNTIME_BOOTSTRAP_VERSION=$(GO_RUNTIME_BOOTSTRAP_VERSION) \
	--build-arg GO_COMPILE_COMMAND_ID=$(GO_COMPILE_COMMAND_ID) \
	--build-arg GO_RUN_COMMAND_ID=$(GO_RUN_COMMAND_ID) \
	--build-arg GO_MANIFEST_VERSION=$(GO_MANIFEST_VERSION) \
	--build-arg GO_MANIFEST_CHECKSUM=$(GO_MANIFEST_CHECKSUM)

OCI_BUILD_ARGS := \
	--build-arg VERSION=$(VERSION) \
	--build-arg VCS_REF=$(VCS_REF) \
	--build-arg SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) \
	--build-arg BUILD_DATE=$(BUILD_DATE)

.PHONY: dev-bootstrap run-api
dev-bootstrap:
	bash scripts/dev/bootstrap.sh

run-api:
	cd $(API_DIR) && go run cmd/server/main.go

.PHONY: run-website
run-website:
	cd $(WEBSITE_DIR) && npm run dev

.PHONY: generate generate-ent
generate: generate-ent mocks

generate-ent:
	cd $(API_DIR) && go generate ./...

.PHONY: docker-up
docker-up:
	$(COMPOSE) up -d

.PHONY: docker-down
docker-down:
	$(COMPOSE) down

.PHONY: docker-build-api docker-build-worker docker-build-migrate docker-build-website docker-build-sandbox docker-build

docker-build-api:
	$(DOCKER) buildx build --load \
		--file docker/api/Dockerfile \
		--target api \
		--build-context go_common=$(GO_COMMON_CONTEXT) \
		--build-arg GO_COMMON_VCS_REF=$(GO_COMMON_VCS_REF) \
		$(OCI_BUILD_ARGS) \
		--tag $(API_IMAGE) .

docker-build-worker:
	$(DOCKER) buildx build --load \
		--file docker/api/Dockerfile \
		--target worker \
		--build-context go_common=$(GO_COMMON_CONTEXT) \
		--build-arg GO_COMMON_VCS_REF=$(GO_COMMON_VCS_REF) \
		$(OCI_BUILD_ARGS) \
		--tag $(WORKER_IMAGE) .

docker-build-migrate:
	$(DOCKER) buildx build --load \
		--file docker/api/Dockerfile \
		--target migrate \
		--build-context go_common=$(GO_COMMON_CONTEXT) \
		--build-arg GO_COMMON_VCS_REF=$(GO_COMMON_VCS_REF) \
		$(OCI_BUILD_ARGS) \
		--tag $(MIGRATE_IMAGE) .

docker-build-website:
	$(DOCKER) buildx build --load \
		--file docker/website/Dockerfile \
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

.PHONY: migrate-apply
migrate-apply:
	cd $(API_DIR) && go run ./cmd/migrate

.PHONY: migrate-status
migrate-status:
	@test -n "$(DB_URL)" || { echo "DB_URL is required for migrate-status" >&2; exit 2; }
	cd $(API_DIR) && atlas migrate status \
		--dir "file://$(REL_MIGRATE_DIR)" \
		--url "$(DB_URL)"

.PHONY: seed
seed:
	docker exec -i judgify-postgres psql --set ON_ERROR_STOP=1 \
		-U $(POSTGRES_USER) -d $(POSTGRES_DB) < ./scripts/seed.sql

# ============================================================
# Mock generation. Source mode keeps declarations scoped to each port file;
# package mode is used for interfaces with fixed-size digest parameters because
# mockgen source mode cannot resolve qualified array lengths on current Go.
# ============================================================

.PHONY: mocks
mocks:
	bash $(API_DIR)/scripts/generate-mocks.sh

# NOTE: tx.Manager (go-common) is not mocked. Tests use testutil.TxManager, an
# inline pass-through Manager — see internal/testutil/tx.go.

# --- Identity ---
.PHONY: mocks-identity
mocks-identity:
	@mkdir -p $(API_DIR)/$(PORTS_DIR)/identity/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/authentication.go -destination=$(PORTS_DIR)/identity/mocks/mock_authentication.go -package=mocks
	$(MOCKGEN) -destination=$(PORTS_DIR)/identity/mocks/mock_user.go -package=mocks github.com/huynhanx03/judgify/internal/identity/ports UserRepository,UserService
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/credential.go -destination=$(PORTS_DIR)/identity/mocks/mock_credential.go -package=mocks
	$(MOCKGEN) -destination=$(PORTS_DIR)/identity/mocks/mock_role.go -package=mocks github.com/huynhanx03/judgify/internal/identity/ports RoleRepository,RoleService
	$(MOCKGEN) -destination=$(PORTS_DIR)/identity/mocks/mock_authorization.go -package=mocks github.com/huynhanx03/judgify/internal/identity/ports AuthorizationService,AuthorizationCatalogReader,EffectiveCapabilityReader
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/attribute_definition.go -destination=$(PORTS_DIR)/identity/mocks/mock_attribute_definition.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/user_attribute_value.go -destination=$(PORTS_DIR)/identity/mocks/mock_user_attribute_value.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/federated_identity.go -destination=$(PORTS_DIR)/identity/mocks/mock_federated_identity.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/cache.go -destination=$(PORTS_DIR)/identity/mocks/mock_cache.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/session.go -destination=$(PORTS_DIR)/identity/mocks/mock_session.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/token.go -destination=$(PORTS_DIR)/identity/mocks/mock_token.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/token_lifecycle.go -destination=$(PORTS_DIR)/identity/mocks/mock_token_lifecycle.go -package=mocks

# --- Cultivation ---
.PHONY: mocks-cultivation
mocks-cultivation:
	@mkdir -p $(API_DIR)/$(PORTS_DIR)/cultivation/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/element.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_element.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/level.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_level.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/rank.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_rank.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/rarity.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_rarity.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/trait.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_trait.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/user_element_exp.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_user_element_exp.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/user_stats.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_user_stats.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/user_trait.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_user_trait.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/ranking.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_ranking.go -package=mocks

# --- Problem ---
.PHONY: mocks-problem
mocks-problem:
	@mkdir -p $(API_DIR)/$(PORTS_DIR)/problem/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/problem.go -destination=$(PORTS_DIR)/problem/mocks/mock_problem.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/test_case.go -destination=$(PORTS_DIR)/problem/mocks/mock_test_case.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/difficulty.go -destination=$(PORTS_DIR)/problem/mocks/mock_difficulty.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/tag.go -destination=$(PORTS_DIR)/problem/mocks/mock_tag.go -package=mocks

# --- Contest ---
.PHONY: mocks-contest
mocks-contest:
	@mkdir -p $(API_DIR)/$(PORTS_DIR)/contest/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/contest.go -destination=$(PORTS_DIR)/contest/mocks/mock_contest.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/rating.go -destination=$(PORTS_DIR)/contest/mocks/mock_rating.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/registration.go -destination=$(PORTS_DIR)/contest/mocks/mock_registration.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/standing.go -destination=$(PORTS_DIR)/contest/mocks/mock_standing.go -package=mocks

# --- Material ---
.PHONY: mocks-material
mocks-material:
	@mkdir -p $(API_DIR)/$(PORTS_DIR)/material/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/material/ports/material.go -destination=$(PORTS_DIR)/material/mocks/mock_material.go -package=mocks

# --- Submission ---
.PHONY: mocks-submission
mocks-submission:
	@mkdir -p $(API_DIR)/$(PORTS_DIR)/submission/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/submission/ports/submission.go -destination=$(PORTS_DIR)/submission/mocks/mock_submission.go -package=mocks

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
	docker compose -f test/compose.integration.yml down --volumes

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
