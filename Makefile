APP_NAME := dispatch-backend
# DB_URL ?= postgres://postgres:pwaiswa@localhost:5432/dispatch_db?sslmode=disable
DB_URL ?= postgres://dispatch:dispatch@localhost:5431/dispatch_db?sslmode=disable
GOOSE_DRIVER ?= postgres
GOOSE_DIR ?= migrations
GOOSE ?= goose
REDIS_ADDR ?= localhost:6379

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make migrate-up          - run all up migrations"
	@echo "  make migrate-down        - roll back one migration"
	@echo "  make migrate-down-all    - roll back all migrations"
	@echo "  make migrate-reset       - down-all then up"
	@echo "  make migrate-status      - show goose status"
	@echo "  make migrate-version     - show current version"
	@echo "  make migrate-create name=add_something - create migration"
	@echo "  make run-migrator        - run Go migration runner"
	@echo "  make docker-up           - start docker services"
	@echo "  make docker-down         - stop docker services"
	@echo "  make docker-logs         - tail docker logs"
	@echo "  make deps                - install Go deps"
	@echo "  make tidy                - go mod tidy"

.PHONY: deps
deps:
	go mod download

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: migrate-up
migrate-up:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(DB_URL)" up

.PHONY: migrate-down
migrate-down:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(DB_URL)" down

.PHONY: migrate-down-all
migrate-down-all:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(DB_URL)" down-to 0

.PHONY: migrate-reset
migrate-reset: migrate-down-all migrate-up

.PHONY: migrate-status
migrate-status:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(DB_URL)" status

.PHONY: migrate-version
migrate-version:
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$(DB_URL)" version

.PHONY: migrate-create
migrate-create:
	@test -n "$(name)" || (echo "Usage: make migrate-create name=your_migration_name" && exit 1)
	$(GOOSE) -dir $(GOOSE_DIR) create $(name) sql

.PHONY: run-migrator
run-migrator:
	go run ./cmd/migrate -command up

.PHONY: docker-up
docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

.PHONY: docker-down
docker-down:
	docker compose -f deployments/docker/docker-compose.yml down -v --remove-orphans

.PHONY: docker-logs
docker-logs:
	docker compose -f deployments/docker/docker-compose.yml logs -f

.PHONY: test-redis
test-redis:
	docker exec -it dispatch_redis redis-cli -u redis://$(REDIS_ADDR) ping

.PHONY: run-app
run-app:
	go run ./cmd/server/main.go

.PHONY: run-seed
run-seed:
	go run ./cmd/seed/main.go

.PHONY: run-events
run-events:
	go run ./cmd/worker/main.go

.PHONY: swagger
swagger:
	swag init -g ./cmd/server/main.go -o ./docs --parseDependency --parseInternal