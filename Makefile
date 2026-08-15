SHELL := /bin/bash

COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || (docker-compose version >/dev/null 2>&1 && echo "docker-compose"))
COMPOSE_FILE := deploy/compose/docker-compose.yml
APP_ENV_FILE := $(if $(wildcard .env),.env,.env.example)
COMPOSE_ENV_FILE := $(if $(wildcard deploy/compose/.env),deploy/compose/.env,deploy/compose/.env.example)

.DEFAULT_GOAL := help

.PHONY: help deps migrate migrate-down dev-services-up dev-services-down dev-services-logs run-api run-worker run-scheduler run-frontend test backend-test backend-integration-test frontend-test lint backend-lint frontend-lint deploy-lint format format-check build quality

help: ## Show available commands.
	@awk 'BEGIN {FS = ":.*## "; printf "OpsKeeper development commands:\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download backend and frontend dependencies.
	cd backend && go mod download
	cd frontend && npm install

migrate: ## Apply pending PostgreSQL migrations.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/migrate

migrate-down: ## Roll back the latest PostgreSQL migration.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/migrate down

dev-services-up: ## Start PostgreSQL and Redis.
	@test -n "$(COMPOSE)" || (echo "Docker Compose is not installed" && exit 1)
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) up -d

dev-services-down: ## Stop PostgreSQL and Redis.
	@test -n "$(COMPOSE)" || (echo "Docker Compose is not installed" && exit 1)
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) down

dev-services-logs: ## Follow PostgreSQL and Redis logs.
	@test -n "$(COMPOSE)" || (echo "Docker Compose is not installed" && exit 1)
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) logs -f

run-api: ## Run the API server.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/api

run-worker: ## Run the worker process.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/worker

run-scheduler: ## Run the scheduler process.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/scheduler

run-frontend: ## Run the Svelte development server.
	cd frontend && npm run dev

backend-test:
	cd backend && go test ./...

backend-integration-test: ## Run PostgreSQL organization integration tests.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go test -tags=integration ./organization

frontend-test:
	cd frontend && npm run test

test: backend-test frontend-test ## Run all unit tests.

backend-lint:
	cd backend && go vet ./...

frontend-lint:
	cd frontend && npm run check

deploy-lint:
	sh -n deploy/compose/postgres/check-ready.sh deploy/compose/postgres/init/001-create-opskeeper.sh

lint: backend-lint frontend-lint deploy-lint ## Run backend, frontend, and deployment static checks.

format: ## Format source files.
	cd backend && gofmt -w .
	cd frontend && npm run format

format-check: ## Check source formatting without modifying files.
	@files=$$(cd backend && gofmt -l .); test -z "$$files" || (echo "Unformatted Go files:"; echo "$$files"; exit 1)
	cd frontend && npm run format:check

build: ## Build backend binaries and the frontend bundle.
	cd backend && go build -buildvcs=false ./...
	cd frontend && npm run build

quality: format-check lint test build ## Run the complete local quality gate.
