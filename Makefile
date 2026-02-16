.PHONY: help deps test lint build run dev docker-up docker-down clean test-load test-load-live test-load-quick migrate-up migrate-down migrate-steps migrate-version web-install web-dev web-build web-lint

# Variables
BINARY_NAME=gatify
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

help: ## Show this help
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \\033[36m%-15s\\033[0m %s\\n", $$1, $$2}'

deps: ## Install dependencies
	$(GO) mod download
	$(GO) mod tidy
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

test: ## Run tests
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

test-verbose: ## Run tests with verbose output
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out -v ./...

test-integration: ## Run integration tests (requires Docker services)
	$(GO) test -tags=integration -v ./...

test-e2e: ## Run end-to-end tests (requires all services running)
	@echo "⚠️  Ensure all services are running:"
	@echo "   1. docker-compose up -d"
	@echo "   2. go run ./cmd/gatify (in separate terminal)"
	@echo ""
	$(GO) test -tags=e2e -v ./tests/e2e/ -count=1

test-load: ## Run k6 load tests (local gateway)
	@command -v k6 >/dev/null 2>&1 || { echo "Install k6: brew install k6"; exit 1; }
	k6 run tests/load/k6_gateway.js

test-load-live: ## Run k6 load tests against live gateway
	@command -v k6 >/dev/null 2>&1 || { echo "Install k6: brew install k6"; exit 1; }
	k6 run -e BASE_URL=https://api.siruyy.cloud tests/load/k6_gateway.js

test-load-quick: ## Run quick local k6 smoke+load test
	@command -v k6 >/dev/null 2>&1 || { echo "Install k6: brew install k6"; exit 1; }
	k6 run -e QUICK=true tests/load/k6_gateway.js

test-all: test test-integration test-e2e ## Run all tests (unit, integration, e2e)

lint: ## Run linter
	golangci-lint run --timeout=5m ./...

build: ## Build binary
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/gatify

run: build ## Build and run
	./bin/$(BINARY_NAME)

dev: ## Start development environment (Docker services)
	docker-compose up -d redis timescaledb test-backend
	@echo "✅ Development services started!"
	@echo "   Redis:       localhost:6379"
	@echo "   TimescaleDB: localhost:5432"
	@echo "   Test Backend: localhost:8080"

docker-up: ## Start all Docker services
	docker-compose up -d

docker-down: ## Stop all Docker services
	docker-compose down

docker-logs: ## View Docker logs
	docker-compose logs -f

web-install: ## Install frontend dependencies
	cd web && npm install

web-dev: ## Run frontend dev server
	cd web && npm run dev

web-build: ## Build frontend
	cd web && npm run build

web-lint: ## Lint frontend
	cd web && npm run lint

migrate-up: ## Apply all pending database migrations (requires DATABASE_URL)
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required" && exit 1)
	$(GO) run ./cmd/migrate -action up -database-url "$$DATABASE_URL"

migrate-down: ## Roll back all database migrations (requires DATABASE_URL)
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required" && exit 1)
	$(GO) run ./cmd/migrate -action down -database-url "$$DATABASE_URL"

migrate-steps: ## Apply migration steps with signed STEPS (requires DATABASE_URL and STEPS)
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required" && exit 1)
	@test -n "$$STEPS" || (echo "STEPS is required (example: STEPS=1 or STEPS=-1)" && exit 1)
	$(GO) run ./cmd/migrate -action steps -steps "$$STEPS" -database-url "$$DATABASE_URL"

migrate-version: ## Show current migration version (requires DATABASE_URL)
	@test -n "$$DATABASE_URL" || (echo "DATABASE_URL is required" && exit 1)
	$(GO) run ./cmd/migrate -action version -database-url "$$DATABASE_URL"

clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.out
	$(GO) clean

fmt: ## Format code
	$(GO) fmt ./...
	gofmt -s -w .

vet: ## Run go vet
	$(GO) vet ./...

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

.DEFAULT_GOAL := help
