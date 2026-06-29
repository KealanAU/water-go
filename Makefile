# Common developer tasks. Run `make help` for a summary of targets.
SQLC_VERSION := v1.27.0

.PHONY: help sqlc tidy build test run-ingester run-api up down logs fmt

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

sqlc: ## Generate type-safe DB code from db/queries
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

tidy: ## Tidy go modules
	go mod tidy

build: ## Build both binaries into ./bin
	go build -o ./bin/ingester ./cmd/ingester
	go build -o ./bin/api ./cmd/api

test: ## Run tests
	go test ./...

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
