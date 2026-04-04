API_DIR      := ./api
SERVER_MAIN  := $(API_DIR)/cmd/server/main.go
REL_MIGRATE_DIR := internal/ent/migrate/migrations
REL_ENT_SCHEMA  := internal/ent/schema
DB_URL       := "postgresql://admin:admin@localhost:5432/judgify?sslmode=disable"
DEV_URL      := "docker://postgres/16/dev"
WEBSITE_DIR := ./website

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