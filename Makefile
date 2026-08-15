SHELL := /bin/bash

COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || (docker-compose version >/dev/null 2>&1 && echo "docker-compose"))
COMPOSE_FILE := deploy/compose/docker-compose.yml
APP_ENV_FILE := $(if $(wildcard .env),.env,.env.example)
COMPOSE_ENV_FILE := $(if $(wildcard deploy/compose/.env),deploy/compose/.env,deploy/compose/.env.example)
BINARY_PREFIX ?= opskeeper
WEBUI_DIST := backend/webui/dist
IMAGE_REPOSITORY ?= opskeeper
IMAGE_TAG ?= local
GOPROXY ?= https://goproxy.cn,direct

.DEFAULT_GOAL := help

.PHONY: help deps migrate migrate-down dev-services-up dev-services-down dev-services-logs run-api run-worker run-scheduler run-frontend run-dev test backend-test backend-embedded-test backend-integration-test frontend-test lint backend-lint frontend-lint deploy-lint format format-check validate-binary-prefix frontend-build webui-assets backend-binaries build image quality

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
	set -a; source $(APP_ENV_FILE); set +a; cd frontend && npm run dev

run-dev: ## Run the API and Vite frontend together.
	@set -uo pipefail; \
	api_pid=""; \
	frontend_pid=""; \
	cleanup() { \
		trap - EXIT INT TERM; \
		for pid in "$$api_pid" "$$frontend_pid"; do \
			if [[ -n "$$pid" ]] && kill -0 "$$pid" 2>/dev/null; then \
				kill -TERM "$$pid" 2>/dev/null || true; \
			fi; \
		done; \
		for pid in "$$api_pid" "$$frontend_pid"; do \
			if [[ -n "$$pid" ]]; then wait "$$pid" 2>/dev/null || true; fi; \
		done; \
	}; \
	trap cleanup EXIT; \
	trap 'exit 130' INT; \
	trap 'exit 143' TERM; \
	$(MAKE) --no-print-directory run-api & api_pid=$$!; \
	$(MAKE) --no-print-directory run-frontend & frontend_pid=$$!; \
	wait -n "$$api_pid" "$$frontend_pid"

backend-test:
	cd backend && go test ./...

backend-embedded-test: webui-assets
	cd backend && go test -tags=embed_webui ./webui ./httpapi

backend-integration-test: ## Run PostgreSQL migration and organization integration tests.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go test -tags=integration ./migrations ./organization

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

validate-binary-prefix:
	@[[ "$(BINARY_PREFIX)" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$$ ]] || (echo "BINARY_PREFIX must contain lowercase letters, digits, or internal hyphens" && exit 1)

frontend-build:
	cd frontend && npm run build

webui-assets: frontend-build
	mkdir -p $(WEBUI_DIST)
	find $(WEBUI_DIST) -mindepth 1 -maxdepth 1 -exec rm -rf {} +
	cp -R frontend/dist/. $(WEBUI_DIST)/

backend-binaries: validate-binary-prefix webui-assets
	mkdir -p backend/bin
	cd backend && go build -buildvcs=false -tags=embed_webui -o bin/$(BINARY_PREFIX)-api ./cmd/api
	cd backend && go build -buildvcs=false -o bin/$(BINARY_PREFIX)-worker ./cmd/worker
	cd backend && go build -buildvcs=false -o bin/$(BINARY_PREFIX)-scheduler ./cmd/scheduler
	cd backend && go build -buildvcs=false -o bin/$(BINARY_PREFIX)-migrate ./cmd/migrate

build: backend-binaries ## Build production binaries with the embedded frontend.

image: ## Build the final application image.
	docker build \
		--build-arg GOPROXY=$(GOPROXY) \
		-f deploy/Dockerfile \
		-t $(IMAGE_REPOSITORY):$(IMAGE_TAG) \
		.

quality: format-check lint test backend-embedded-test build ## Run the complete local quality gate.
