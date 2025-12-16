# =============================================================================
# Go Config Library - Makefile
# Version: 1.0.0
# =============================================================================

# Default environment file
ENV_FILE ?= .env

# Docker Compose command
DC = docker compose
DOCKER = docker

# Go related variables
GO = go
GO_TEST = $(GO) test
GO_MOD = $(GO) mod
GO_BUILD = $(GO) build
GO_LINT = golangci-lint
GO_BENCH = $(GO) test -bench=.
GO_RACE = $(GO) test -race
GO_COVER = $(GO) test -cover

# Project variables
PROJECT_NAME = cache
MODULE_PATH = github.com/os-golib/go-cache
BINARY_NAME = cache-example
BIN_DIR = bin
COVERAGE_FILE = coverage.out
COVERAGE_HTML = coverage.html
BENCH_FILE = bench.txt

# Test coverage threshold
COVERAGE_THRESHOLD = 80

# Benchmark parameters
BENCH_TIME = 1s
BENCH_COUNT = 5

# Linter configuration
LINT_CONFIG = .golangci.yml

# =============================================================================
# PHONY TARGETS
# =============================================================================
.PHONY: help \
        test test-race test-v test-integration \
        benchmark benchmark-cpu benchmark-mem \
        lint lint-fix fmt vet \
        coverage coverage-html coverage-ci \
        deps deps-update deps-check \
        build build-all cross-build \
        generate docs \
        dev-tools install-tools \
        dc-up dc-up-build dc-down dc-down-volumes dc-logs dc-logs-redis dc-logs-commander dc-ps dc-restart dc-health dc-redis-cli dc-exec-redis dc-exec-commander dc-stats dc-memory dc-keys dc-monitor dc-flush dc-backup dc-test dc-clean\
        release release-patch release-minor release-major \
        validate clean

# =============================================================================
# HELP
# =============================================================================
help:
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║                     Go Cache Library - Makefile                  ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "📦 Development"
	@echo "  make deps             - Download dependencies"
	@echo "  make deps-update      - Update dependencies"
	@echo "  make deps-check       - Check for outdated dependencies"
	@echo "  make fmt              - Format code"
	@echo "  make lint             - Run linter"
	@echo "  make lint-fix         - Run linter and fix issues"
	@echo "  make vet              - Run go vet"
	@echo ""
	@echo "🧪 Testing"
	@echo "  make test             - Run unit tests"
	@echo "  make test-race        - Run tests with race detector"
	@echo "  make test-v           - Run tests verbosely"
	@echo "  make test-integration - Run integration tests"
	@echo "  make benchmark        - Run benchmarks"
	@echo "  make benchmark-cpu    - Run CPU profiling benchmarks"
	@echo "  make benchmark-mem    - Run memory profiling benchmarks"
	@echo "  make coverage         - Generate coverage report"
	@echo "  make coverage-html    - Generate HTML coverage report"
	@echo ""
	@echo "🏗️ Building"
	@echo "  make build            - Build main binary"
	@echo "  make build-all        - Build all packages"
	@echo "  make cross-build      - Cross-compile for multiple platforms"
	@echo "  make generate         - Generate code (mocks, etc.)"
	@echo ""
	@echo "🐳 Docker Compose"
	@echo "  make dc-up            - Start services"
	@echo "  make dc-up-build      - Build and start services"
	@echo "  make dc-down          - Stop services"
	@echo "  make dc-down-volumes  - Stop services and remove volumes"
	@echo "  make dc-logs          - Show all logs"
	@echo "  make dc-logs-redis    - Show Redis logs"
	@echo "  make dc-logs-commander- Show Redis Commander logs"
	@echo "  make dc-ps            - List services"
	@echo "  make dc-restart       - Restart all services"
	@echo "  make dc-health        - Check service health"
	@echo "  make dc-redis-cli     - Open Redis CLI"
	@echo "  make dc-exec-redis    - Exec into Redis container"
	@echo "  make dc-exec-commander- Exec into Redis Commander container"
	@echo "  make dc-stats         - Show Redis statistics"
	@echo "  make dc-memory        - Show Redis memory usage"
	@echo "  make dc-keys          - List Redis keys"
	@echo "  make dc-monitor       - Monitor Redis commands"
	@echo "  make dc-flush         - Flush Redis database"
	@echo "  make dc-backup        - Create Redis backup"
	@echo "  make dc-test          - Test Redis connection"
	@echo "  make dc-clean         - Clean all resources"
	@echo ""
	@echo "📚 Documentation"
	@echo "  make docs             - Generate documentation"
	@echo "  make docs-serve       - Serve documentation locally"
	@echo ""
	@echo "🔧 Tools"
	@echo "  make dev-tools        - Install development tools"
	@echo "  make install-tools    - Install all required tools"
	@echo "  make validate         - Run all validation checks"
	@echo ""
	@echo "🚀 Release"
	@echo "  make release          - Create a release (requires semver)"
	@echo "  make release-patch    - Create patch release"
	@echo "  make release-minor    - Create minor release"
	@echo "  make release-major    - Create major release"
	@echo ""

# =============================================================================
# DOCKER COMPOSE
# =============================================================================
dc-up:
	@echo "🚀 Starting Docker Compose services..."
	$(DC) -f docker-compose.yml up -d
	@echo "✅ Services started"

dc-up-build:
	@echo "🔨 Building and starting Docker Compose services..."
	$(DC) -f docker-compose.yml up -d --build
	@echo "✅ Services built and started"

dc-down:
	@echo "🛑 Stopping Docker Compose services..."
	$(DC) -f docker-compose.yml down
	@echo "✅ Services stopped"

dc-down-volumes:
	@echo "🗑️ Stopping Docker Compose services and removing volumes..."
	$(DC) -f docker-compose.yml down -v
	@echo "✅ Services stopped and volumes removed"

dc-logs:
	@echo "📋 Showing Docker Compose logs..."
	$(DC) -f docker-compose.yml logs -f

dc-logs-redis:
	@echo "🔴 Showing Redis logs..."
	$(DC) -f docker-compose.yml logs -f redis

dc-logs-commander:
	@echo "📊 Showing Redis Commander logs..."
	$(DC) -f docker-compose.yml logs -f redis-commander

dc-ps:
	@echo "📊 Listing Docker Compose services..."
	$(DC) -f docker-compose.yml ps

dc-restart:
	@echo "🔄 Restarting Docker Compose services..."
	$(DC) -f docker-compose.yml restart
	@echo "✅ Services restarted"

dc-restart-redis:
	@echo "🔄 Restarting Redis service..."
	$(DC) -f docker-compose.yml restart redis
	@echo "✅ Redis restarted"

dc-restart-commander:
	@echo "🔄 Restarting Redis Commander service..."
	$(DC) -f docker-compose.yml restart redis-commander
	@echo "✅ Redis Commander restarted"

dc-exec-redis:
	@echo "🐚 Executing into Redis container..."
	$(DC) -f docker-compose.yml exec redis sh

dc-exec-commander:
	@echo "🐚 Executing into Redis Commander container..."
	$(DC) -f docker-compose.yml exec redis-commander sh

dc-redis-cli:
	@echo "🛠️ Opening Redis CLI..."
	$(DC) -f docker-compose.yml exec redis redis-cli

dc-health:
	@echo "❤️ Checking service health..."
	@echo "Redis health check:"
	$(DC) -f docker-compose.yml exec redis redis-cli ping
	@echo ""
	@echo "Services status:"
	$(DC) -f docker-compose.yml ps

dc-stats:
	@echo "📈 Showing Redis statistics..."
	$(DC) -f docker-compose.yml exec redis redis-cli info

dc-memory:
	@echo "🧠 Showing Redis memory usage..."
	$(DC) -f docker-compose.yml exec redis redis-cli info memory

dc-keys:
	@echo "🔑 Listing Redis keys..."
	$(DC) -f docker-compose.yml exec redis redis-cli --scan

dc-flush:
	@echo "🧹 Flushing Redis database..."
	@read -p "Are you sure you want to flush Redis? [y/N]: " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		$(DC) -f docker-compose.yml exec redis redis-cli flushall; \
		echo "✅ Redis flushed"; \
	else \
		echo "❌ Flush cancelled"; \
	fi

dc-config:
	@echo "⚙️ Showing Redis configuration..."
	$(DC) -f docker-compose.yml exec redis redis-cli config get "*"

dc-backup:
	@echo "💾 Creating Redis backup..."
	@timestamp=$$(date +%Y%m%d_%H%M%S); \
	backup_dir="backups"; \
	mkdir -p $$backup_dir; \
	$(DC) -f docker-compose.yml exec -T redis redis-cli --rdb - > $$backup_dir/redis_backup_$$timestamp.rdb; \
	echo "✅ Backup saved to $$backup_dir/redis_backup_$$timestamp.rdb"

dc-monitor:
	@echo "👀 Monitoring Redis commands in real-time..."
	$(DC) -f docker-compose.yml exec redis redis-cli monitor

dc-test:
	@echo "🧪 Testing Redis connection..."
	@if $(DC) -f docker-compose.yml exec redis redis-cli ping | grep -q "PONG"; then \
		echo "✅ Redis is responding correctly"; \
	else \
		echo "❌ Redis is not responding"; \
		exit 1; \
	fi

dc-clean:
	@echo "🧹 Cleaning Docker Compose resources..."
	$(DC) -f docker-compose.yml down -v --rmi local
	@echo "✅ Docker Compose resources cleaned"

dc-pull:
	@echo "📥 Pulling latest Docker images..."
	$(DC) -f docker-compose.yml pull
	@echo "✅ Images pulled"

dc-prune:
	@echo "🧹 Cleaning Docker system..."
	$(DC) -f docker-compose.yml down -v --rmi local --volumes
	$(DOCKER) system prune -af --volumes
	@echo "✅ Docker system cleaned"

# =============================================================================
# DEVELOPMENT
# =============================================================================
fmt:
	@echo "🎨 Formatting code..."
	$(GO) fmt ./...
	@if command -v goimports > /dev/null; then \
		goimports -w -local $(MODULE_PATH) .; \
		echo "✅ Code formatted with goimports"; \
	else \
		echo "⚠️  goimports not installed, using go fmt only"; \
	fi

lint:
	@echo "🔍 Running linter..."
	@if [ -f "$(LINT_CONFIG)" ]; then \
		$(GO_LINT) run -c $(LINT_CONFIG); \
	else \
		$(GO_LINT) run; \
	fi

lint-fix:
	@echo "🔧 Running linter with fixes..."
	@if [ -f "$(LINT_CONFIG)" ]; then \
		$(GO_LINT) run -c $(LINT_CONFIG) --fix; \
	else \
		$(GO_LINT) run --fix; \
	fi

vet:
	@echo "🔬 Running go vet..."
	$(GO) vet ./...
	@echo "✅ Vet completed"

# =============================================================================
# DEPENDENCIES
# =============================================================================
deps:
	@echo "📦 Downloading dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy -v
	@echo "✅ Dependencies downloaded"

deps-update:
	@echo "🔄 Updating dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy -v
	$(GO_MOD) verify
	@echo "✅ Dependencies updated"

deps-check:
	@echo "🔍 Checking for outdated dependencies..."
	@if command -v goose > /dev/null; then \
		go list -u -m -f '{{if .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}' all; \
	else \
		echo "⚠️  goose not installed. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest"; \
	fi

# =============================================================================
# TESTING
# =============================================================================
test:
	@echo "🧪 Running unit tests..."
	$(GO_TEST) -v -race -coverprofile=$(COVERAGE_FILE) ./...

test-race:
	@echo "🏃 Running tests with race detector..."
	$(GO_RACE) -v ./...

test-v:
	@echo "🔊 Running tests verbosely..."
	$(GO_TEST) -v ./...

test-integration:
	@echo "🔗 Running integration tests..."
	@if [ -f "$(ENV_FILE)" ]; then \
		export $$(cat $(ENV_FILE) | xargs); \
	fi
	$(GO_TEST) -v -tags=integration ./...

benchmark:
	@echo "⚡ Running benchmarks..."
	$(GO_BENCH) -benchtime=$(BENCH_TIME) -count=$(BENCH_COUNT) ./... | tee $(BENCH_FILE)
	@echo "✅ Benchmarks saved to $(BENCH_FILE)"

benchmark-cpu:
	@echo "💻 Running CPU profiling benchmarks..."
	$(GO) test -bench=. -benchtime=$(BENCH_TIME) -cpuprofile=cpu.prof ./...

benchmark-mem:
	@echo "🧠 Running memory profiling benchmarks..."
	$(GO) test -bench=. -benchtime=$(BENCH_TIME) -memprofile=mem.prof ./...

# =============================================================================
# COVERAGE
# =============================================================================
coverage: test
	@echo "📊 Generating coverage report..."
	$(GO) tool cover -func=$(COVERAGE_FILE)
	@coverage=$$($(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "📈 Coverage: $$coverage%"; \
	if [ $$coverage -lt $(COVERAGE_THRESHOLD) ]; then \
		echo "❌ Error: Coverage $$coverage% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi

coverage-html: coverage
	@echo "🌐 Generating HTML coverage report..."
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✅ HTML report generated: $(COVERAGE_HTML)"
	@if command -v open > /dev/null; then \
		open $(COVERAGE_HTML); \
	elif command -v xdg-open > /dev/null; then \
		xdg-open $(COVERAGE_HTML); \
	fi

coverage-ci: test
	@echo "🔍 Running coverage for CI..."
	$(GO) tool cover -func=$(COVERAGE_FILE)

# =============================================================================
# BUILDING
# =============================================================================
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/example
	@echo "✅ Binary built: $(BIN_DIR)/$(BINARY_NAME)"

build-all:
	@echo "🔨 Building all packages..."
	$(GO_BUILD) ./...

cross-build:
	@echo "🌍 Cross-compiling for multiple platforms..."
	@mkdir -p $(BIN_DIR)/dist
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ]; then \
				ext=".exe"; \
			else \
				ext=""; \
			fi; \
			echo "Building for $$os/$$arch..."; \
			GOOS=$$os GOARCH=$$arch $(GO_BUILD) -o $(BIN_DIR)/dist/$(BINARY_NAME)-$$os-$$arch$$ext ./cmd/example; \
		done; \
	done
	@echo "✅ Cross-compilation complete"

# =============================================================================
# CODE GENERATION
# =============================================================================
generate:
	@echo "⚙️  Generating code..."
	$(GO) generate ./...
	@echo "✅ Code generation complete"


# # =============================================================================
# # DOCUMENTATION
# # =============================================================================
# docs:
# 	@echo "📚 Generating documentation..."
# 	@if command -v godoc > /dev/null; then \
# 		godoc -http=:6060 & \
# 		DOC_PID=$$!; \
# 		sleep 2; \
# 		echo "📖 Documentation available at http://localhost:6060/pkg/$(MODULE_PATH)/"; \
# 		echo "Press Ctrl+C to stop"; \
# 		wait $$DOC_PID; \
# 	else \
# 		echo "⚠️  godoc not installed. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
# 	fi

docs-serve:
	@echo "🌐 Serving documentation..."
	godoc -http=:6060

# =============================================================================
# TOOLS INSTALLATION
# =============================================================================
dev-tools:
	@echo "🔧 Installing development tools..."
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Installing goimports..."
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Installing mockery..."
	go install github.com/vektra/mockery/v2@latest
	@echo "Installing godoc..."
	go install golang.org/x/tools/cmd/godoc@latest
	@echo "Installing goose..."
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "✅ Development tools installed"

install-tools: dev-tools
	@echo "📦 Installing additional tools..."
	@echo "Installing richgo for better test output..."
	go install github.com/kyoh86/richgo@latest
	@echo "Installing gocov for coverage..."
	go install github.com/axw/gocov/gocov@latest
	go install github.com/AlekSi/gocov-xml@latest
	@echo "✅ All tools installed"

# =============================================================================
# VALIDATION
# =============================================================================
validate: deps fmt lint vet test coverage
	@echo "✅ All validation checks passed!"

# =============================================================================
# RELEASE
# =============================================================================
release:
	@echo "🚀 Creating release..."
	@if ! command -v goreleaser > /dev/null; then \
		echo "❌ goreleaser not installed. Install with: go install github.com/goreleaser/goreleaser@latest"; \
		exit 1; \
	fi
	@if [ -z "$$VERSION" ]; then \
		echo "❌ VERSION variable not set. Usage: VERSION=v1.0.0 make release"; \
		exit 1; \
	fi
	goreleaser release --clean

release-patch:
	@echo "📦 Creating patch release..."
	@if command -v semver > /dev/null; then \
		VERSION=$$(semver bump patch); \
		git tag -a "v$$VERSION" -m "Release v$$VERSION"; \
		git push origin "v$$VERSION"; \
		echo "✅ Patch release v$$VERSION created"; \
	else \
		echo "❌ semver not installed. Install with: go install github.com/bryanftsg/semver@latest"; \
	fi

release-minor:
	@echo "🔄 Creating minor release..."
	@if command -v semver > /dev/null; then \
		VERSION=$$(semver bump minor); \
		git tag -a "v$$VERSION" -m "Release v$$VERSION"; \
		git push origin "v$$VERSION"; \
		echo "✅ Minor release v$$VERSION created"; \
	else \
		echo "❌ semver not installed"; \
	fi

release-major:
	@echo "🚀 Creating major release..."
	@if command -v semver > /dev/null; then \
		VERSION=$$(semver bump major); \
		git tag -a "v$$VERSION" -m "Release v$$VERSION"; \
		git push origin "v$$VERSION"; \
		echo "✅ Major release v$$VERSION created"; \
	else \
		echo "❌ semver not installed"; \
	fi

# =============================================================================
# CLEANUP
# =============================================================================
clean:
	@echo "🧹 Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML) $(BENCH_FILE)
	rm -f *.prof *.test
	rm -rf dist/
	@echo "✅ Cleanup complete"

# =============================================================================
# DEFAULT TARGET
# =============================================================================
.DEFAULT_GOAL := help