.PHONY: help deps test lint build run dev docker-up docker-down clean

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
