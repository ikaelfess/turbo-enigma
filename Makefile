.DEFAULT_GOAL := help

COMPOSE ?= docker compose

.PHONY: help tools hooks env up up-d up-all down logs ps restart migrate test lint build

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*##/ { printf "  %-12s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

tools: ## Install Go, goose, golangci-lint, and lefthook with mise
	mise install

hooks: tools ## Install git hooks via lefthook
	lefthook install

env: ## Create .env from .env.example if missing
	@test -f .env || cp .env.example .env

up: env ## Run the API (Postgres + migrations as dependencies)
	$(COMPOSE) up --build api

up-d: env ## Run the API in the background
	$(COMPOSE) up --build -d api

up-all: env ## Run every Compose service, including publisher and worker stubs
	$(COMPOSE) up --build

down: ## Stop Compose services
	$(COMPOSE) down

logs: ## Follow Compose logs
	$(COMPOSE) logs -f --tail=100

ps: ## Show Compose service status
	$(COMPOSE) ps

restart: env ## Recreate the API container
	$(COMPOSE) up --build -d --force-recreate api

migrate: env ## Re-run goose migrations in Compose
	$(COMPOSE) run --rm postgres_migrations

test: ## Run tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run

build: ## Compile all packages
	go build ./...
