FRONTEND_DIR = ./web/default
BACKEND_DIR = .
DEV_FRONTEND_DEFAULT_PORT ?= 5173
DEV_COMPOSE_FILE = docker-compose.dev.yml
DEV_POSTGRES_SERVICE = postgres
DEV_BACKEND_SERVICE = data-proxy
DEV_POSTGRES_DB = data_proxy
DEV_POSTGRES_USER = root
DEV_SQLITE_PATH ?= one-api.db
GO ?= go
GO_TEST_ENV ?= GOTOOLCHAIN=auto
NODE ?= $(shell command -v node 2>/dev/null || { test -x "$(HOME)/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node" && printf '%s' "$(HOME)/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node"; } || printf 'node')
TSC ?= ./node_modules/typescript/bin/tsc
MCP_BRIDGE_GO_TEST_PATTERN ?= TestParseBridgeEndpoint|TestBridgeClient|TestMCPProxy.*Bridge|TestBridge|TestRemoteBridge|TestMCP.*Bridge
MCP_BRIDGE_SMOKE_CONCURRENCY ?= 12
MCP_BRIDGE_SMOKE_ITERATIONS ?= 4
MCP_BRIDGE_SMOKE_TIMEOUT ?= 240000
MCP_BRIDGE_SMOKE_ARGS ?=
MCP_BRIDGE_STRESS_CONCURRENCY ?= 32
MCP_BRIDGE_STRESS_ITERATIONS ?= 8
MCP_BRIDGE_STRESS_TIMEOUT ?= 360000
MCP_BRIDGE_STRESS_ARGS ?=
MCP_OPENAPI_GO_TEST_PATTERN ?= TestPreviewMCPOpenAPIForAdmin|Test.*OpenAPI|TestDownloadMCPOpenAPIBinaryObject
MCP_PROXY_GO_TEST_PATTERN ?= TestMCPProxy|TestBillingEventSourceMatrix
MCP_MIGRATION_MYSQL_DSN ?=
MCP_MIGRATION_POSTGRES_DSN ?=
MCP_MIGRATION_COMPOSE_FILE ?= docker-compose.migration.yml
MCP_MIGRATION_POSTGRES_PORT ?= 15432
MCP_MIGRATION_MYSQL_PORT ?= 13306
MCP_MIGRATION_KEEP_DOCKER ?= 0
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD ?= 0
DEPLOYMENT_PREFLIGHT_DOCKER_TARGET ?= builder2
DEPLOYMENT_PREFLIGHT_IMAGE ?= data-proxy:preflight-builder
ENTERPRISE_QUOTA_COUNTER_STRESS_MODE ?= all
ENTERPRISE_QUOTA_COUNTER_STRESS_REDIS_BACKEND ?= fake
ENTERPRISE_QUOTA_COUNTER_STRESS_WORKERS ?= 32
ENTERPRISE_QUOTA_COUNTER_STRESS_OPERATIONS ?= 200
ENTERPRISE_QUOTA_COUNTER_STRESS_TIMEOUT ?= 180s

.PHONY: all build-frontend build-all-frontends start-backend dev dev-api dev-api-rebuild dev-web reset-setup deployment-preflight snapless-connected-app-preflight snapless-connected-app-preprod-smoke snapless-connected-app-release-evidence enterprise-quota-counter-stress mcp-openapi-check mcp-proxy-check mcp-dashboard-check mcp-migration-sqlite mcp-migration-mysql mcp-migration-postgres mcp-migration-postgres-docker mcp-migration-mysql-docker mcp-migration-docker mcp-migration-docker-clean mcp-bridge-check mcp-bridge-smoke mcp-bridge-stress mcp-regression

all: build-all-frontends start-backend

build-frontend:
	@echo "Building default frontend..."
	@cd ./web && bun install --frozen-lockfile
	@cd $(FRONTEND_DIR) && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat ../../VERSION) bun run build

build-all-frontends: build-frontend

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

dev-api:
	@echo "Starting backend services (docker)..."
	@docker compose -f $(DEV_COMPOSE_FILE) up -d

dev-api-rebuild:
	@echo "Rebuilding and starting backend service (docker)..."
	@docker compose -f $(DEV_COMPOSE_FILE) up -d --build $(DEV_BACKEND_SERVICE)

dev-web:
	@echo "Starting default frontend dev server..."
	@echo "Default frontend: http://localhost:$(DEV_FRONTEND_DEFAULT_PORT)"
	@cd ./web && bun install
	@cd $(FRONTEND_DIR) && bun run dev -- --host 0.0.0.0 --port $(DEV_FRONTEND_DEFAULT_PORT)

dev: dev-api dev-web

reset-setup:
	@echo "Resetting local setup wizard state..."
	@if docker compose -f $(DEV_COMPOSE_FILE) ps --services --status running | grep -qx "$(DEV_POSTGRES_SERVICE)"; then \
		echo "Detected running docker dev PostgreSQL. Removing setup record and root users..."; \
		docker compose -f $(DEV_COMPOSE_FILE) exec -T $(DEV_POSTGRES_SERVICE) \
			psql -U $(DEV_POSTGRES_USER) -d $(DEV_POSTGRES_DB) \
			-c 'DELETE FROM setups;' \
			-c 'DELETE FROM users WHERE role = 100;' \
			-c "DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "Restarting docker dev backend so setup status is recalculated..."; \
		docker compose -f $(DEV_COMPOSE_FILE) restart $(DEV_BACKEND_SERVICE); \
	elif db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; db_path="$${db_path%%\?*}"; [ -f "$$db_path" ]; then \
		db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; \
		db_path="$${db_path%%\?*}"; \
		echo "Detected local SQLite database: $$db_path"; \
		sqlite3 "$$db_path" \
			"DELETE FROM setups; DELETE FROM users WHERE role = 100; DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "SQLite setup state reset. Restart the local backend process before testing the setup wizard."; \
	else \
		echo "No running docker dev PostgreSQL or local SQLite database found."; \
		echo "Start the dev stack with 'make dev-api', or set SQLITE_PATH/DEV_SQLITE_PATH to your local SQLite database."; \
		exit 1; \
	fi

deployment-preflight:
	@echo "Running deployment preflight..."
	@$(GO_TEST_ENV) $(GO) test ./...
	@$(MAKE) build-all-frontends
	@docker compose config >/dev/null
	@docker compose -f $(DEV_COMPOSE_FILE) config >/dev/null
	@docker version >/dev/null
	@docker buildx version >/dev/null
	@if [ "$(DEPLOYMENT_PREFLIGHT_DOCKER_BUILD)" = "1" ]; then \
		echo "Running optional Docker image build target $(DEPLOYMENT_PREFLIGHT_DOCKER_TARGET)..."; \
		docker build --target "$(DEPLOYMENT_PREFLIGHT_DOCKER_TARGET)" -t "$(DEPLOYMENT_PREFLIGHT_IMAGE)" .; \
	else \
		echo "Skipping full Docker image build. Set DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 to run it."; \
	fi
	@git diff --check
	@echo "Deployment preflight passed."

snapless-connected-app-preflight:
	@scripts/snapless-connected-app-preflight.sh

snapless-connected-app-preprod-smoke:
	@scripts/snapless-connected-app-preprod-smoke.sh

snapless-connected-app-release-evidence:
	@scripts/snapless-connected-app-release-evidence.sh

enterprise-quota-counter-stress:
	@ENTERPRISE_QUOTA_COUNTER_STRESS_MODE="$(ENTERPRISE_QUOTA_COUNTER_STRESS_MODE)" \
		ENTERPRISE_QUOTA_COUNTER_STRESS_REDIS_BACKEND="$(ENTERPRISE_QUOTA_COUNTER_STRESS_REDIS_BACKEND)" \
		ENTERPRISE_QUOTA_COUNTER_STRESS_WORKERS="$(ENTERPRISE_QUOTA_COUNTER_STRESS_WORKERS)" \
		ENTERPRISE_QUOTA_COUNTER_STRESS_OPERATIONS="$(ENTERPRISE_QUOTA_COUNTER_STRESS_OPERATIONS)" \
		ENTERPRISE_QUOTA_COUNTER_STRESS_TIMEOUT="$(ENTERPRISE_QUOTA_COUNTER_STRESS_TIMEOUT)" \
		scripts/enterprise-quota-counter-stress.sh

mcp-openapi-check:
	@echo "Running MCP OpenAPI regression tests..."
	@$(GO_TEST_ENV) $(GO) test ./pkg/mcp/openapi ./pkg/mcp/executor ./service -run '$(MCP_OPENAPI_GO_TEST_PATTERN)' -count=1 -timeout=120s

mcp-proxy-check:
	@echo "Running MCP Proxy regression tests..."
	@$(GO_TEST_ENV) $(GO) test ./pkg/mcp/proxy -count=1 -timeout=120s
	@$(GO_TEST_ENV) $(GO) test ./model ./service -run '$(MCP_PROXY_GO_TEST_PATTERN)' -count=1 -timeout=180s

mcp-dashboard-check:
	@echo "Running MCP Dashboard regression checks..."
	@cd $(FRONTEND_DIR) && $(NODE) scripts/check-mcp-routes.mjs
	@cd $(FRONTEND_DIR) && $(NODE) --experimental-strip-types scripts/check-mcp-trends.mjs
	@cd $(FRONTEND_DIR) && $(NODE) --experimental-strip-types scripts/check-mcp-openapi-import-summary.mjs
	@cd $(FRONTEND_DIR) && $(NODE) $(TSC) -b

mcp-migration-sqlite:
	@echo "Running MCP migration smoke against temporary SQLite..."
	@MCP_MIGRATION_TEST=1 $(GO_TEST_ENV) $(GO) test ./model -run TestMCPMigrationSmoke -count=1 -timeout=120s

mcp-migration-mysql:
	@test -n "$(MCP_MIGRATION_MYSQL_DSN)" || { echo "Set MCP_MIGRATION_MYSQL_DSN to run MySQL migration smoke."; exit 1; }
	@echo "Running MCP migration smoke against MySQL..."
	@MCP_MIGRATION_TEST=1 SQL_DSN="$(MCP_MIGRATION_MYSQL_DSN)" $(GO_TEST_ENV) $(GO) test ./model -run TestMCPMigrationSmoke -count=1 -timeout=120s

mcp-migration-postgres:
	@test -n "$(MCP_MIGRATION_POSTGRES_DSN)" || { echo "Set MCP_MIGRATION_POSTGRES_DSN to run PostgreSQL migration smoke."; exit 1; }
	@echo "Running MCP migration smoke against PostgreSQL..."
	@MCP_MIGRATION_TEST=1 SQL_DSN="$(MCP_MIGRATION_POSTGRES_DSN)" $(GO_TEST_ENV) $(GO) test ./model -run TestMCPMigrationSmoke -count=1 -timeout=120s

mcp-migration-postgres-docker:
	@set -eu; \
	compose_file="$(MCP_MIGRATION_COMPOSE_FILE)"; \
	service="migration-postgres"; \
	port="$(MCP_MIGRATION_POSTGRES_PORT)"; \
	cleanup() { \
		if [ "$(MCP_MIGRATION_KEEP_DOCKER)" != "1" ]; then \
			docker compose -f "$$compose_file" rm -sfv "$$service" >/dev/null 2>&1 || true; \
		fi; \
	}; \
	docker compose -f "$$compose_file" rm -sfv "$$service" >/dev/null 2>&1 || true; \
	trap cleanup EXIT; \
	echo "Starting disposable PostgreSQL migration database on 127.0.0.1:$$port..."; \
	MCP_MIGRATION_POSTGRES_PORT="$$port" docker compose -f "$$compose_file" up -d --wait --wait-timeout 120 "$$service"; \
	$(MAKE) mcp-migration-postgres MCP_MIGRATION_POSTGRES_DSN="postgres://root:123456@127.0.0.1:$$port/data_proxy_migration?sslmode=disable"

mcp-migration-mysql-docker:
	@set -eu; \
	compose_file="$(MCP_MIGRATION_COMPOSE_FILE)"; \
	service="migration-mysql"; \
	port="$(MCP_MIGRATION_MYSQL_PORT)"; \
	cleanup() { \
		if [ "$(MCP_MIGRATION_KEEP_DOCKER)" != "1" ]; then \
			docker compose -f "$$compose_file" rm -sfv "$$service" >/dev/null 2>&1 || true; \
		fi; \
	}; \
	docker compose -f "$$compose_file" rm -sfv "$$service" >/dev/null 2>&1 || true; \
	trap cleanup EXIT; \
	echo "Starting disposable MySQL migration database on 127.0.0.1:$$port..."; \
	MCP_MIGRATION_MYSQL_PORT="$$port" docker compose -f "$$compose_file" up -d --wait --wait-timeout 180 "$$service"; \
	$(MAKE) mcp-migration-mysql MCP_MIGRATION_MYSQL_DSN="root:123456@tcp(127.0.0.1:$$port)/data_proxy_migration?charset=utf8mb4&parseTime=true&loc=Local"

mcp-migration-docker: mcp-migration-postgres-docker mcp-migration-mysql-docker

mcp-migration-docker-clean:
	@docker compose -f $(MCP_MIGRATION_COMPOSE_FILE) down -v --remove-orphans

mcp-bridge-check:
	@echo "Checking MCP Bridge daemon scripts..."
	@$(NODE) --check tools/bridge_client_daemon.mjs
	@$(NODE) --check tools/bridge_daemon_concurrency_smoke.mjs
	@tmp_dir=$$(mktemp -d); $(NODE) tools/bridge_client_daemon.mjs --self-test --workspace="$$tmp_dir"; self_test_status=$$?; rm -rf "$$tmp_dir"; exit $$self_test_status
	@echo "Running MCP Bridge Go tests..."
	@$(GO_TEST_ENV) $(GO) test ./pkg/mcp/proxy ./pkg/mcp/executor ./service -run '$(MCP_BRIDGE_GO_TEST_PATTERN)' -count=1 -timeout=120s

mcp-bridge-smoke:
	@echo "Running MCP Bridge local daemon concurrency smoke..."
	@$(NODE) tools/bridge_daemon_concurrency_smoke.mjs \
		--concurrency=$(MCP_BRIDGE_SMOKE_CONCURRENCY) \
		--iterations=$(MCP_BRIDGE_SMOKE_ITERATIONS) \
		--timeout=$(MCP_BRIDGE_SMOKE_TIMEOUT) \
		$(MCP_BRIDGE_SMOKE_ARGS)

mcp-bridge-stress:
	@$(MAKE) mcp-bridge-smoke \
		MCP_BRIDGE_SMOKE_CONCURRENCY=$(MCP_BRIDGE_STRESS_CONCURRENCY) \
		MCP_BRIDGE_SMOKE_ITERATIONS=$(MCP_BRIDGE_STRESS_ITERATIONS) \
		MCP_BRIDGE_SMOKE_TIMEOUT=$(MCP_BRIDGE_STRESS_TIMEOUT) \
		MCP_BRIDGE_SMOKE_ARGS="$(MCP_BRIDGE_STRESS_ARGS)"

mcp-regression: mcp-openapi-check mcp-proxy-check mcp-bridge-check mcp-dashboard-check
	@echo "MCP regression passed."
