# Tiên Đạo Online — common tasks. Run from repo root.
GAME := services/game
WEB  := apps/web
UID  := $(shell id -u)
GID  := $(shell id -g)
DB_URL ?= postgres://tiendao:tiendao@localhost:2432/tiendao?sslmode=disable
MIGRATE_IMG := migrate/migrate:v4.18.1
SQLC_IMG := sqlc/sqlc:1.29.0
COMPOSE := docker compose --env-file .env -f deployments/docker-compose.yml

.PHONY: help up down logs db-up migrate-up migrate-down migrate-new sqlc api worker web test test-go test-web test-integration lint check content-check fmt balance
.PHONY: monitoring-up backup restore backup-test loadtest loadtest-seed loadtest-clean opsctl

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n",$$1,$$2}'

up: ## Start full stack (docker compose)
	$(COMPOSE) up -d --build
down: ## Stop stack
	$(COMPOSE) down
logs: ## Tail stack logs
	$(COMPOSE) logs -f --tail=100
db-up: ## Start only postgres + redis + migrate
	$(COMPOSE) up -d postgres redis migrate

migrate-up: ## Apply migrations to DB_URL
	docker run --rm --network host -v $(CURDIR)/$(GAME)/migrations:/m:ro $(MIGRATE_IMG) -path=/m -database "$(DB_URL)" up
migrate-down: ## Roll back ONE migration
	docker run --rm --network host -v $(CURDIR)/$(GAME)/migrations:/m:ro $(MIGRATE_IMG) -path=/m -database "$(DB_URL)" down 1
migrate-new: ## Create migration pair: make migrate-new name=add_sects
	docker run --rm -u $(UID):$(GID) -v $(CURDIR)/$(GAME)/migrations:/m $(MIGRATE_IMG) create -ext sql -dir /m -seq -digits 6 $(name)

sqlc: ## Regenerate internal/db from db/queries + migrations
	docker run --rm -u $(UID):$(GID) -v $(CURDIR)/$(GAME):/src -w /src $(SQLC_IMG) generate

api: ## Run API on host (reads .env)
	cd $(GAME) && set -a && . ../../.env && set +a && go run ./cmd/api
worker: ## Run worker on host (reads .env)
	cd $(GAME) && set -a && . ../../.env && set +a && go run ./cmd/worker
web: ## Run SvelteKit dev server
	cd $(WEB) && npm run dev

test: test-go test-web ## All tests
test-go:
	cd $(GAME) && go test -race ./...
test-web:
	cd $(WEB) && npm test
test-integration: ## DB/Redis integration tests (needs `make db-up`)
	cd $(GAME) && go test -tags integration -race -count=1 ./...

content-check: ## Validate content/catalog/*.yaml (each loader + cross-references)
	cd $(GAME) && go test ./internal/realm/ ./internal/vip/ ./internal/item/ ./internal/spiritroot/ ./internal/contribution/ ./internal/world/ ./internal/moderation/ ./internal/chat/ ./internal/faction/ ./internal/dailytask/ ./internal/shop/ ./internal/fortune/ ./internal/trial/ ./internal/cosmetic/ ./internal/factionwork/ ./internal/secretrealm/ ./internal/protector/ ./internal/payment/ ./internal/trade/ ./internal/appearance/ ./internal/tutorial/ ./internal/announce/ ./internal/content/ -run Shipped -count=1

balance: ## Progression matrix (sessions × VIP) from the real catalogue
	cd $(GAME) && go run ./cmd/balancesim

fmt: ## Format Go code
	cd $(GAME) && gofmt -w .
lint: ## Static checks
	cd $(GAME) && go vet ./... && test -z "$$(gofmt -l .)"
	cd $(WEB) && npm run check
check: lint test ## Everything CI runs

# ───── operations (Phase 4) — see deployments/README.md ─────
K6_IMG := grafana/k6:1.1.0
PROFILE ?= smoke
LT_USERS ?= 4000
PSQL := $(COMPOSE) exec -T postgres psql -X -q -U tiendao -d tiendao

monitoring-up: ## Start the stack + Prometheus (127.0.0.1:2090) + Grafana (127.0.0.1:2001)
	$(COMPOSE) --profile monitoring up -d --build
backup: ## pg_dump -Fc to backups/ (BACKUP_DIR, BACKUP_RETENTION=14)
	deployments/scripts/backup.sh
restore: ## Restore a dump: make restore FILE=backups/x.dump [TARGET_DB=...] (asks for confirmation)
	@test -n "$(FILE)" || (echo "usage: make restore FILE=backups/<dump>"; exit 1)
	TARGET_DB=$(TARGET_DB) deployments/scripts/restore.sh $(FILE)
backup-test: ## Backup → restore into a scratch DB → compare schema + row counts
	deployments/scripts/backup-test.sh
loadtest-seed: ## Create LT_USERS load-test users/characters in the dev DB
	$(PSQL) -v n=$(LT_USERS) < loadtest/seed.sql
loadtest-clean: ## Delete load-test users (cascade) and rebuild Redis rankings
	$(PSQL) < loadtest/cleanup.sql
	$(COMPOSE) restart worker
loadtest: ## k6 via docker against the compose api: make loadtest [PROFILE=smoke|full]
	docker run --rm -i --network tiendao_default -v $(CURDIR)/loadtest:/scripts:ro -e PROFILE=$(PROFILE) -e LT_USERS=$(LT_USERS) \
		-e BASE_URL=http://api:8080 $(K6_IMG) run /scripts/main.js
opsctl: ## Operator CLI in the api image: make opsctl ARGS="status"
	$(COMPOSE) run --rm --no-deps api /app/opsctl $(ARGS)
