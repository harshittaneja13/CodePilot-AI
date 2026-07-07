# ============================================================================
# CodePilot AI — Makefile
# ============================================================================
# Usage: make help
# ============================================================================

.PHONY: build build-mcp run test eval lint clean \
        docker-build docker-up docker-down docker-logs docker-clean \
        migrate-up migrate-down \
        frontend-dev frontend-build \
        help

# ---------- Variables ----------
APP_NAME     := codepilot-server
GO           := go
GOFLAGS      := -trimpath -ldflags="-s -w"
COMPOSE      := docker compose
MIGRATE      := $(GO) run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ---------- Default Target ----------
.DEFAULT_GOAL := help

# ============================================================================
# Go Backend
# ============================================================================

## build: Compile the Go backend binary
build:
	@echo "🔨 Building $(APP_NAME)..."
	$(GO) build $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/server
	@echo "✅ Binary → bin/$(APP_NAME)"

## build-mcp: Compile the CodePilot MCP server binary (stdio)
build-mcp:
	@echo "🔨 Building codepilot-mcp-server..."
	$(GO) build $(GOFLAGS) -o bin/codepilot-mcp-server ./cmd/mcp-server
	@echo "✅ Binary → bin/codepilot-mcp-server"

## run: Run the backend locally (requires Postgres + Redis)
run:
	@echo "🚀 Starting $(APP_NAME)..."
	$(GO) run ./cmd/server

## test: Run all Go tests with coverage
test:
	@echo "🧪 Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@echo "✅ Tests complete. Coverage report → coverage.out"

## eval: Run the offline review-quality eval harness (needs LLM_API_KEY)
eval:
	@echo "📊 Running review eval harness..."
	$(GO) run ./cmd/eval

## lint: Run golangci-lint
lint:
	@echo "🔍 Linting..."
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}
	golangci-lint run ./...
	@echo "✅ Lint passed"

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf bin/ coverage.out coverage.html
	@echo "✅ Clean"

# ============================================================================
# Docker
# ============================================================================

## docker-build: Build all Docker images
docker-build:
	@echo "🐳 Building Docker images..."
	$(COMPOSE) build
	@echo "✅ Images built"

## docker-up: Start all services in detached mode
docker-up:
	@echo "🚀 Starting services..."
	$(COMPOSE) up -d
	@echo "✅ Services running"
	@echo "   Backend  → http://localhost:8080"
	@echo "   Frontend → http://localhost:3000"
	@echo "   Postgres → localhost:5432"
	@echo "   Redis    → localhost:6379"

## docker-down: Stop and remove all containers
docker-down:
	@echo "🛑 Stopping services..."
	$(COMPOSE) down
	@echo "✅ Services stopped"

## docker-logs: Follow logs from all services
docker-logs:
	$(COMPOSE) logs -f

## docker-clean: Stop services and remove volumes (⚠ destroys data)
docker-clean:
	@echo "⚠️  This will destroy all data volumes!"
	$(COMPOSE) down -v --remove-orphans
	@echo "✅ Cleaned"

# ============================================================================
# Database Migrations
# ============================================================================

## migrate-up: Run all pending database migrations
migrate-up:
	@echo "📦 Running migrations..."
	$(MIGRATE) -path migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)" up
	@echo "✅ Migrations applied"

## migrate-down: Roll back the last migration
migrate-down:
	@echo "⏪ Rolling back last migration..."
	$(MIGRATE) -path migrations -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)" down 1
	@echo "✅ Rolled back"

# ============================================================================
# Frontend
# ============================================================================

## frontend-dev: Start the Vite dev server
frontend-dev:
	@echo "⚛️  Starting frontend dev server..."
	cd frontend && npm run dev

## frontend-build: Build the frontend for production
frontend-build:
	@echo "📦 Building frontend..."
	cd frontend && npm ci && npm run build
	@echo "✅ Frontend built → frontend/dist/"

# ============================================================================
# Help
# ============================================================================

## help: Show this help message
help:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════╗"
	@echo "║           🤖 CodePilot AI — Make Targets                ║"
	@echo "╚══════════════════════════════════════════════════════════╝"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | \
		sed 's/^## //' | \
		awk -F: '{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
