SHELL := /bin/bash

COMPOSE_FILE := deploy/compose/docker-compose.yml
WEBUI_DIST := backend/webui/dist
IMAGE_REPOSITORY ?= opskeeper
IMAGE_TAG ?= local
# Use pipe fallback semantics so an unavailable proxy does not block the
# development workflow. A comma only falls back for 404/410 responses.
# A command-line assignment (for example, `make GOPROXY=...`) can still
# override this project default. Environment-only values are intentionally
# ignored so a stale global GOPROXY cannot disable the fallback chain.
GOPROXY = https://goproxy.cn|https://proxy.golang.org|direct
export GOPROXY
ALPINE_MIRROR ?= https://mirrors.aliyun.com/alpine
NPM_REGISTRY ?= https://registry.npmmirror.com
CGO_ENABLED ?= 0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PACKAGE := opskeeper/backend/version
GO_LDFLAGS := -s -w -X $(VERSION_PACKAGE).Value=$(VERSION) -X $(VERSION_PACKAGE).Commit=$(COMMIT) -X $(VERSION_PACKAGE).BuildTime=$(BUILD_TIME)

define docker-image
	docker build \
		--build-arg "GOPROXY=$(GOPROXY)" \
		--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR) \
		--build-arg NPM_REGISTRY=$(NPM_REGISTRY) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--target $(1) -t $(IMAGE_REPOSITORY)-$(1):$(IMAGE_TAG) -f deploy/Dockerfile .
endef

define compose
@if docker compose version >/dev/null 2>&1; then \
	docker compose -f $(COMPOSE_FILE) --env-file .env $(1); \
elif command -v docker-compose >/dev/null 2>&1; then \
	docker-compose -f $(COMPOSE_FILE) --env-file .env $(1); \
else \
	echo "Docker Compose is not installed"; \
	exit 1; \
fi
endef

.DEFAULT_GOAL := help

.PHONY: help deps env run up down clean db-up db-down db-clean db-logs obs-up obs-down obs-clean obs-logs infra-up infra-down infra-clean infra-logs api-run worker-run scheduler-run frontend-run app-run api-image worker-image scheduler-image frontend-image app-image image api-up worker-up scheduler-up frontend-up app-up sample-seed sample-clean docker-mcp-test docker-mcp-build docker-mcp-run host-mcp-test host-mcp-build host-mcp-run kubernetes-mcp-test kubernetes-mcp-build kubernetes-mcp-run test backend-test backend-embedded-test backend-integration-test llm-provider-test frontend-test lint backend-lint frontend-lint deploy-lint helm-lint format format-check frontend-build webui-assets backend-build build quality

help: ## Show available commands.
	@awk 'BEGIN {FS = ":.*## "; printf "OpsKeeper development commands:\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-22s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download backend and frontend dependencies.
	cd backend && go mod download
	cd frontend && npm install --registry="$(NPM_REGISTRY)"

env: ## Create and synchronize the local environment files.
	@mkdir -p deploy/compose
	@test -f .env || cp .env.example .env
	cp .env.example deploy/compose/.env.example
	cp .env deploy/compose/.env

run: env db-up app-run ## Start PostgreSQL and run the embedded app locally.

up: env app-image ## Start PostgreSQL and the embedded app container.
	$(call compose,up -d --wait --wait-timeout 180 postgres app)

down: env ## Stop PostgreSQL and the embedded app container.
	$(call compose,stop postgres app)

clean: env ## Remove all local Compose containers, networks, and data volumes.
	$(call compose,down --volumes --remove-orphans)

sample-seed: env ## Insert local sample teams, projects, users, GLM channel, Docker and Host resources, and role bindings.
	@set -a; source .env; set +a; cd backend && go run ./cmd/sampledata seed

sample-clean: env ## Remove only marked local sample teams, projects, users, GLM channel, Docker and Host resources, and bindings.
	@set -a; source .env; set +a; cd backend && go run ./cmd/sampledata clean

db-up: env ## Start the PostgreSQL development dependency.
	$(call compose,up -d --wait --wait-timeout 60 postgres)

db-down: env ## Stop the PostgreSQL development dependency.
	$(call compose,stop postgres)

db-clean: env ## Remove the PostgreSQL container and data volume.
	$(call compose,down --volumes postgres)

db-logs: env ## Follow PostgreSQL logs.
	$(call compose,logs -f postgres)

obs-up: env ## Start Jaeger, OpenTelemetry, Prometheus, Loki, and Grafana.
	$(call compose,--profile obs up -d --wait --wait-timeout 60 jaeger otel-collector prometheus loki grafana)

obs-down: env ## Stop the local observability stack.
	$(call compose,--profile obs stop jaeger otel-collector prometheus loki grafana)

obs-clean: env ## Remove observability containers and data volumes.
	$(call compose,--profile obs down --volumes jaeger otel-collector prometheus loki grafana)

obs-logs: env ## Follow local observability stack logs.
	$(call compose,--profile obs logs -f jaeger otel-collector prometheus loki grafana)

infra-up: env db-up obs-up ## Start the database and observability containers.

infra-down: env db-down obs-down ## Stop the database and observability containers.

infra-clean: env db-clean obs-clean ## Remove the database and observability containers and data.

infra-logs: env ## Follow database and observability logs.
	$(call compose,--profile obs logs -f postgres jaeger otel-collector prometheus loki grafana)

api-run: env ## Run the API server directly in the development environment.
	set -a; source .env; set +a; cd backend && go run ./cmd/api

worker-run: env ## Run the Worker directly in the development environment.
	set -a; source .env; set +a; cd backend && go run ./cmd/worker

scheduler-run: env ## Run the Scheduler directly in the development environment.
	set -a; source .env; set +a; cd backend && go run ./cmd/scheduler

frontend-run: env ## Run the frontend development server directly.
	set -a; source .env; set +a; cd frontend && npm run dev

app-run: env webui-assets ## Build and embed the frontend, then run the app directly.
	set -a; source .env; set +a; cd backend && go run -tags=embed_webui ./cmd/api

api-image: ## Build the API image.
	$(call docker-image,api)

worker-image: ## Build the Worker image.
	$(call docker-image,worker)

scheduler-image: ## Build the Scheduler image.
	$(call docker-image,scheduler)

frontend-image: ## Build the standalone frontend image.
	$(call docker-image,frontend)

app-image: ## Build the embedded app image.
	$(call docker-image,app)

image: api-image worker-image scheduler-image frontend-image app-image ## Build all service images.

api-up: env api-image ## Start the API container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 api)

worker-up: env worker-image ## Start the Worker container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 worker)

scheduler-up: env scheduler-image ## Start the Scheduler container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 scheduler)

frontend-up: env frontend-image ## Start the standalone frontend container.
	$(call compose,--profile svc up -d --wait --wait-timeout 180 frontend)

app-up: env app-image ## Start the embedded app container.
	$(call compose,up -d --wait --wait-timeout 180 app)

docker-mcp-test: ## Run docker-mcp unit and Docker Engine simulator tests.
	cd backend && go test ./mcpserver/docker/... ./cmd/docker-mcp

docker-mcp-build: ## Build the standalone Docker MCP server binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-docker-mcp ./cmd/docker-mcp

docker-mcp-run: ## Run the standalone Docker MCP server.
	set -a; source .env; set +a; cd backend && go run ./cmd/docker-mcp

host-mcp-test: ## Run Host MCP unit tests.
	cd backend && go test ./tool/host ./mcpserver/host/... ./cmd/host-mcp-agent

host-mcp-build: ## Build the standalone Host MCP agent binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-host-mcp-agent ./cmd/host-mcp-agent

host-mcp-run: ## Run the standalone Host MCP agent.
	set -a; source .env; set +a; cd backend && go run ./cmd/host-mcp-agent

kubernetes-mcp-test: ## Run Kubernetes MCP unit tests.
	cd backend && go test ./mcpserver/kubernetes/... ./cmd/kubernetes-mcp

kubernetes-mcp-build: ## Build the standalone Kubernetes MCP server binary.
	mkdir -p backend/bin
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-kubernetes-mcp ./cmd/kubernetes-mcp

kubernetes-mcp-run: ## Run the standalone Kubernetes MCP server.
	set -a; source .env; set +a; cd backend && go run ./cmd/kubernetes-mcp

backend-test:
	cd backend && go test ./...

backend-embedded-test: webui-assets
	cd backend && go test -tags=embed_webui ./webui ./httpapi

backend-integration-test: ## Run PostgreSQL-backed integration tests.
	set -a; source .env; set +a; cd backend && go test -tags=integration ./migrations ./organization ./identity ./authorization ./resource ./discovery ./connector ./diagnosis ./inspection ./operation ./e2e

llm-provider-test: ## Test the configured external LLM through the ADK Runner.
	set -a; source .env; set +a; cd backend && go test -tags=integration ./llm -run TestSiliconFlowThroughADKRunner -v

frontend-test:
	cd frontend && npm run test

test: backend-test frontend-test ## Run all unit tests.

backend-lint:
	cd backend && go vet ./...

frontend-lint:
	cd frontend && npm run check

deploy-lint:
	sh -n deploy/compose/postgres/check-ready.sh deploy/compose/postgres/init/001-create-opskeeper.sh

helm-lint: ## Validate and render the production Helm chart.
	helm lint deploy/helm/opskeeper
	helm template opskeeper deploy/helm/opskeeper >/dev/null

lint: backend-lint frontend-lint deploy-lint helm-lint ## Run backend, frontend, and deployment static checks.

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
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-api ./cmd/api
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -tags=embed_webui -o bin/opskeeper-app ./cmd/api
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-worker ./cmd/worker
	cd backend && CGO_ENABLED=$(CGO_ENABLED) go build -buildvcs=false -ldflags "$(GO_LDFLAGS)" -o bin/opskeeper-scheduler ./cmd/scheduler

build: webui-assets ## Build production binaries with the embedded frontend and MCP servers.
	$(MAKE) backend-build
	$(MAKE) docker-mcp-build
	$(MAKE) kubernetes-mcp-build

quality: format-check lint test backend-embedded-test build ## Run the complete local quality gate.
