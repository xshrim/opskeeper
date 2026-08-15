SHELL := /bin/bash

COMPOSE_FILE := deploy/compose/docker-compose.yml
APP_ENV_FILE := $(if $(wildcard .env),.env,.env.example)
COMPOSE_ENV_FILE := $(if $(wildcard deploy/compose/.env),deploy/compose/.env,deploy/compose/.env.example)
WEBUI_DIST := backend/webui/dist
IMAGE_REPOSITORY ?= opskeeper
IMAGE_TAG ?= local
GOPROXY ?= https://goproxy.cn,direct
ALPINE_MIRROR ?= https://mirrors.aliyun.com/alpine
NPM_REGISTRY ?= https://registry.npmmirror.com
CGO_ENABLED ?= 0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PACKAGE := opskeeper/backend/version
GO_LDFLAGS := -s -w -X $(VERSION_PACKAGE).Value=$(VERSION) -X $(VERSION_PACKAGE).Commit=$(COMMIT) -X $(VERSION_PACKAGE).BuildTime=$(BUILD_TIME)

define run-compose
@if docker compose version >/dev/null 2>&1; then \
	docker compose -f $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) $(1); \
elif command -v docker-compose >/dev/null 2>&1; then \
	docker-compose -f $(COMPOSE_FILE) --env-file $(COMPOSE_ENV_FILE) $(1); \
else \
	echo "Docker Compose is not installed"; \
	exit 1; \
fi
endef

.DEFAULT_GOAL := help

.PHONY: help deps migrate migrate-down infra-up infra-down infra-logs run-api run-worker run-scheduler run-frontend run-front-api test backend-test backend-embedded-test backend-integration-test frontend-test lint backend-lint frontend-lint deploy-lint format format-check frontend-build webui-assets backend-build build image quality

help: ## Show available commands.
	@awk 'BEGIN {FS = ":.*## "; printf "OpsKeeper development commands:\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download backend and frontend dependencies.
	cd backend && go mod download
	cd frontend && npm install

migrate: ## Apply pending PostgreSQL migrations.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/migrate

migrate-down: ## Roll back the latest PostgreSQL migration.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/migrate down

infra-up: ## Start PostgreSQL and Redis.
	$(call run-compose,up -d)

infra-down: ## Stop PostgreSQL and Redis.
	$(call run-compose,down)

infra-logs: ## Follow PostgreSQL and Redis logs.
	$(call run-compose,logs -f)

run-api: ## Run the API server.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/api

run-worker: ## Run the worker process.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/worker

run-scheduler: ## Run the scheduler process.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run ./cmd/scheduler

run-frontend: ## Run the Svelte development server.
	set -a; source $(APP_ENV_FILE); set +a; cd frontend && npm run dev

run-front-api: webui-assets ## Build and embed the frontend, then run the API.
	set -a; source $(APP_ENV_FILE); set +a; cd backend && go run -tags=embed_webui ./cmd/api

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

frontend-build:
	cd frontend && npm run build

webui-assets: frontend-build
	mkdir -p $(WEBUI_DIST)
	find $(WEBUI_DIST) -mindepth 1 -maxdepth 1 -exec rm -rf {} +
	cp -R frontend/dist/. $(WEBUI_DIST)/

backend-build:
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -tags=embed_webui -o bin/opskeeper-api ./cmd/api
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-worker ./cmd/worker
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-scheduler ./cmd/scheduler
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-migrate ./cmd/migrate

build: webui-assets ## Build production binaries with the embedded frontend.
	$(MAKE) backend-build

image: ## Build the final application image.
	docker build \
		--build-arg GOPROXY=$(GOPROXY) \
		--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR) \
		--build-arg NPM_REGISTRY=$(NPM_REGISTRY) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f deploy/Dockerfile \
		-t $(IMAGE_REPOSITORY):$(IMAGE_TAG) \
		.

quality: format-check lint test backend-embedded-test build ## Run the complete local quality gate.
