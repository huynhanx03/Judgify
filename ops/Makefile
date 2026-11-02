COMPOSE := docker compose --project-name judgify-ops --file stack/docker-compose.yml

.PHONY: up down logs config status validate report perf-smoke perf-normal perf-spike perf-soak

up:
	$(COMPOSE) up --detach

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs --follow

config:
	$(COMPOSE) config

status:
	$(COMPOSE) ps

validate:
	./scripts/validate.sh

report:
	./scripts/create-report.sh $(RUN_ID)

perf-smoke:
	@test -n "$(TARGET_URL)" || (echo "TARGET_URL is required" >&2; exit 2)
	TARGET_URL=$(TARGET_URL) k6 run performance/k6/scenarios/smoke.js

perf-normal:
	@test -n "$(TARGET_URL)" || (echo "TARGET_URL is required" >&2; exit 2)
	TARGET_URL=$(TARGET_URL) k6 run performance/k6/scenarios/normal-1k-ccu.js

perf-spike:
	@test -n "$(TARGET_URL)" || (echo "TARGET_URL is required" >&2; exit 2)
	TARGET_URL=$(TARGET_URL) k6 run performance/k6/scenarios/contest-final-spike.js

perf-soak:
	@test -n "$(TARGET_URL)" || (echo "TARGET_URL is required" >&2; exit 2)
	TARGET_URL=$(TARGET_URL) k6 run performance/k6/scenarios/soak.js
