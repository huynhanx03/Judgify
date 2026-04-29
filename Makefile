API_DIR      := ./api
SERVER_MAIN  := $(API_DIR)/cmd/server/main.go
REL_MIGRATE_DIR := internal/ent/migrate/migrations
REL_ENT_SCHEMA  := internal/ent/schema
DB_URL       := "postgresql://admin:admin@localhost:5432/judgify?sslmode=disable"
DEV_URL      := "docker://postgres/16/dev"
WEBSITE_DIR := ./website
MOCKGEN      := mockgen
PORTS_DIR    := $(API_DIR)/internal

.PHONY: run-api
run-api:
	cd $(API_DIR) && go run cmd/server/main.go

.PHONY: run-website
run-website:
	cd $(WEBSITE_DIR) && npm run dev

.PHONY: generate
generate:
	cd $(API_DIR) && go generate ./...

.PHONY: docker-up
docker-up:
	docker-compose up -d

.PHONY: docker-down
docker-down:
	docker-compose down

# Migrations
.PHONY: migrate-diff
migrate-diff:
	@name=$(name); \
	if [ -z "$$name" ]; then name="change"; fi; \
	cd $(API_DIR) && atlas migrate diff $$name \
		--dir "file://$(REL_MIGRATE_DIR)" \
		--to "ent://$(REL_ENT_SCHEMA)" \
		--dev-url $(DEV_URL)

.PHONY: migrate-apply
migrate-apply:
	cd $(API_DIR) && atlas migrate apply \
		--dir "file://$(REL_MIGRATE_DIR)" \
		--url $(DB_URL)

.PHONY: migrate-status
migrate-status:
	cd $(API_DIR) && atlas migrate status \
		--dir "file://$(REL_MIGRATE_DIR)" \
		--url $(DB_URL)

.PHONY: seed
seed:
	docker exec -i judgify-postgres psql -U admin -d judgify < ./scripts/seed.sql

# ============================================================
# Mock generation (mockgen source mode)
# ============================================================

.PHONY: mocks
mocks: mocks-tx mocks-identity mocks-cultivation mocks-problem mocks-contest mocks-material mocks-submission

.PHONY: mocks-tx
mocks-tx:
	@mkdir -p $(API_DIR)/pkg/common/tx/mocks
	$(MOCKGEN) -source=$(API_DIR)/pkg/common/tx/tx.go -destination=$(API_DIR)/pkg/common/tx/mocks/mock_tx.go -package=mocks

# --- Identity ---
.PHONY: mocks-identity
mocks-identity:
	@mkdir -p $(PORTS_DIR)/identity/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/user.go -destination=$(PORTS_DIR)/identity/mocks/mock_user.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/credential.go -destination=$(PORTS_DIR)/identity/mocks/mock_credential.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/role.go -destination=$(PORTS_DIR)/identity/mocks/mock_role.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/permission.go -destination=$(PORTS_DIR)/identity/mocks/mock_permission.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/resource.go -destination=$(PORTS_DIR)/identity/mocks/mock_resource.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/attribute_definition.go -destination=$(PORTS_DIR)/identity/mocks/mock_attribute_definition.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/user_attribute_value.go -destination=$(PORTS_DIR)/identity/mocks/mock_user_attribute_value.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/federated_identity.go -destination=$(PORTS_DIR)/identity/mocks/mock_federated_identity.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/identity/ports/cache.go -destination=$(PORTS_DIR)/identity/mocks/mock_cache.go -package=mocks

# --- Cultivation ---
.PHONY: mocks-cultivation
mocks-cultivation:
	@mkdir -p $(PORTS_DIR)/cultivation/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/element.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_element.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/level.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_level.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/rank.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_rank.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/rarity.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_rarity.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/trait.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_trait.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/user_element_exp.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_user_element_exp.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/user_stats.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_user_stats.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/user_trait.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_user_trait.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/gacha.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_gacha.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/cultivation/ports/ranking.go -destination=$(PORTS_DIR)/cultivation/mocks/mock_ranking.go -package=mocks

# --- Problem ---
.PHONY: mocks-problem
mocks-problem:
	@mkdir -p $(PORTS_DIR)/problem/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/problem.go -destination=$(PORTS_DIR)/problem/mocks/mock_problem.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/test_case.go -destination=$(PORTS_DIR)/problem/mocks/mock_test_case.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/difficulty.go -destination=$(PORTS_DIR)/problem/mocks/mock_difficulty.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/problem/ports/tag.go -destination=$(PORTS_DIR)/problem/mocks/mock_tag.go -package=mocks

# --- Contest ---
.PHONY: mocks-contest
mocks-contest:
	@mkdir -p $(PORTS_DIR)/contest/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/contest.go -destination=$(PORTS_DIR)/contest/mocks/mock_contest.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/rating.go -destination=$(PORTS_DIR)/contest/mocks/mock_rating.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/registration.go -destination=$(PORTS_DIR)/contest/mocks/mock_registration.go -package=mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/contest/ports/standing.go -destination=$(PORTS_DIR)/contest/mocks/mock_standing.go -package=mocks

# --- Material ---
.PHONY: mocks-material
mocks-material:
	@mkdir -p $(PORTS_DIR)/material/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/material/ports/material.go -destination=$(PORTS_DIR)/material/mocks/mock_material.go -package=mocks

# --- Submission ---
.PHONY: mocks-submission
mocks-submission:
	@mkdir -p $(PORTS_DIR)/submission/mocks
	$(MOCKGEN) -source=$(PORTS_DIR)/submission/ports/submission.go -destination=$(PORTS_DIR)/submission/mocks/mock_submission.go -package=mocks

# ============================================================
# Testing
# ============================================================

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