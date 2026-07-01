.PHONY: help sqlc tidy build test test-integration cover run-ingester run-api up down logs fmt

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# sqlc must be installed on PATH: `brew install sqlc` or
# `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`.
sqlc: ## Generate type-safe DB code from db/queries (requires sqlc installed)
	sqlc generate

tidy: ## Tidy go modules
	go mod tidy

build: ## Build both binaries into ./bin
	go build -o ./bin/ingester ./cmd/ingester
	go build -o ./bin/api ./cmd/api

test: ## Run unit tests (no external services)
	go test ./...

test-integration: ## Run integration tests (needs Docker for testcontainers)
	go test -tags=integration ./...

cover: ## Run unit tests with a coverage summary
	go test -cover ./...

fmt: ## Format code
	go fmt ./...

run-ingester: ## Run the ingester locally (needs DB + .env)
	go run ./cmd/ingester

run-api: ## Run the API locally (needs DB + .env)
	go run ./cmd/api

up: ## Start the full stack (db + ingester + api)
	docker compose up --build -d

down: ## Stop the stack
	docker compose down

logs: ## Tail stack logs
	docker compose logs -f
