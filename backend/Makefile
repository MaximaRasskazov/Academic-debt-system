# Загружаем единый корневой .env (рядом с docker-compose.yml).
# Локальный backend/.env больше не поддерживается — один источник правды.
ifneq (,$(wildcard ../.env))
    include ../.env
    export
endif

.PHONY: help run build test lint tidy migrate-up migrate-down migrate-status migrate-create sqlc docker-up docker-down clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

run: ## Run server locally
	go run ./cmd/server

build: ## Build server binary
	go build -o bin/server ./cmd/server

test: ## Run all tests with race detector and coverage
	go test -race -cover ./...

test-integration: ## Run integration tests against running postgres (docker-compose up postgres first)
	TEST_DATABASE_URL="postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable" \
	    go test -race -cover ./...

lint: ## Run linters (gofmt + go vet + golangci-lint)
	@test -z "$$(gofmt -l -s .)" || (gofmt -l -s . && exit 1)
	go vet ./...
	golangci-lint run ./...

tidy: ## Tidy go.mod
	go mod tidy

# Goose CLI: install via
#   go install github.com/pressly/goose/v3/cmd/goose@latest
DB_DRIVER ?= postgres
DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
MIGRATIONS_DIR ?= sql/migrations

migrate-up: ## Apply all pending migrations
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_URL)" up

migrate-down: ## Rollback last migration
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_URL)" down

migrate-status: ## Show migration status
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) "$(DB_URL)" status

migrate-create: ## Create new SQL migration. Usage: make migrate-create name=create_users
	goose -dir $(MIGRATIONS_DIR) -s create $(name) sql

sqlc: ## Regenerate sqlc code
	sqlc generate

docker-up: ## Start postgres + mailpit + backend
	docker compose -f ../docker-compose.yml up -d

docker-down: ## Stop all services
	docker compose -f ../docker-compose.yml down

clean: ## Remove build artifacts
	rm -rf bin/
