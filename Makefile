# Makefile for managing Docker Compose stack and development tasks
version: 1.3.0

# Default environment file
ENV_FILE ?= .env

# Docker Compose command
DC = docker compose

# Go related variables
GO = go
GO_TEST = $(GO) test
GO_MOD = $(GO) mod
GO_BUILD = $(GO) build
GO_LINT = golangci-lint

# Test coverage threshold
COVERAGE_THRESHOLD = 80

.PHONY: help up down restart logs clean test lint fmt coverage deps

# Default target
help:
	@echo "Available targets:"
	@echo "  up           - Start all services"
	@echo "  down         - Stop all services"
	@echo "  restart      - Restart all services"
	@echo "  logs         - View logs for all services"
	@echo "  clean        - Stop services and clean volumes"
	@echo "  test         - Run tests with coverage"
	@echo "  test-race    - Run tests with race detector"
	@echo "  test-v       - Run tests verbosely"
	@echo "  lint         - Run linter"
	@echo "  lint-fix     - Run linter and fix issues"
	@echo "  fmt          - Format code"
	@echo "  coverage     - Generate test coverage report"
	@echo "  deps         - Download dependencies"
	@echo "  deps-update  - Update dependencies"
	@echo "  build        - Build the project"
	@echo "  health       - Check service health"
	@echo "  prune        - Clean Docker system"

# Start all services
up:
	@echo "Starting services..."
	$(DC) up -d --build
	@make health

# Stop all services
down:
	@echo "Stopping services..."
	$(DC) down

# Restart containers
restart:
	@echo "Restarting services..."
	$(DC) restart

# View logs for all services
logs:
	$(DC) logs -f --tail=200

# View logs for a specific service
logs-%:
	$(DC) logs -f $*

# Rebuild specific service without cache
rebuild-%:
	$(DC) build --no-cache $*
	$(DC) up -d $*

# Clean volumes (DANGEROUS: deletes DB/Redis data)
clean-volumes:
	$(DC) down -v

# Check service health
health:
	@echo "Service status:"
	@docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# Stop + remove all containers, networks, volumes
prune:
	docker system prune -af --volumes

# Show running containers
ps:
	docker ps

# Show container resource usage
stats:
	docker stats

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w -local github.com/os-golib/go-cache .

# Run linter
lint:
	@echo "Running linter..."
	$(GO_LINT) run

# Run linter and fix issues
lint-fix:
	@echo "Running linter with fixes..."
	$(GO_LINT) run --fix

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy
	$(GO_MOD) verify

# Run tests
test:
	@echo "Running tests..."
	$(GO_TEST) -v -race -coverprofile=coverage.out ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	$(GO_TEST) -race -v ./...

# Run tests verbosely
test-v:
	@echo "Running tests verbosely..."
	$(GO_TEST) -v ./...

# Generate test coverage report
coverage: test
	@echo "Generating coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@coverage=$$($(GO) tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $$coverage%"; \
	if [ $$coverage -lt $(COVERAGE_THRESHOLD) ]; then \
		echo "Error: Coverage $$coverage% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi

# Build the project
build:
	@echo "Building project..."
	$(GO_BUILD) -o bin/ ./...

# Clean build artifacts
clean: clean-volumes
	@echo "Cleaning build artifacts..."
	rm -rf bin/ coverage.out coverage.html

# Install development tools
dev-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/vektra/mockery/v2@latest

# Generate mocks
generate:
	@echo "Generating mocks..."
	go generate ./...

.DEFAULT_GOAL := help