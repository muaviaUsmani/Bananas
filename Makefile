.PHONY: help dev prod up down build logs clean restart ps scale-workers test test-verbose test-coverage test-local test-local-verbose test-local-coverage

# Default target - show help
help:
	@echo "Bananas - Distributed Task Queue"
	@echo ""
	@echo "Development Commands:"
	@echo "  make dev              - Start development environment with hot reload"
	@echo "  make dev-build        - Build and start development environment"
	@echo "  make dev-down         - Stop development environment"
	@echo "  make dev-logs         - Follow development logs"
	@echo ""
	@echo "Production Commands:"
	@echo "  make prod             - Start production environment"
	@echo "  make prod-build       - Build and start production environment"
	@echo "  make prod-down        - Stop production environment"
	@echo "  make prod-logs        - Follow production logs"
	@echo ""
	@echo "Service-Specific Commands:"
	@echo "  make logs-api         - View API logs"
	@echo "  make logs-worker      - View worker logs"
	@echo "  make logs-scheduler   - View scheduler logs"
	@echo "  make logs-redis       - View Redis logs"
	@echo ""
	@echo "Testing Commands (in Docker - recommended):"
	@echo "  make test             - Run all tests in Docker (Go 1.23)"
	@echo "  make test-verbose     - Run tests with verbose output in Docker"
	@echo "  make test-coverage    - Run tests with coverage in Docker"
	@echo ""
	@echo "Testing Commands (local - requires Go 1.21+):"
	@echo "  make test-local       - Run tests locally"
	@echo "  make test-local-verbose - Run tests locally with verbose output"
	@echo "  make test-local-coverage - Run tests locally with coverage"
	@echo ""
	@echo "Utility Commands:"
	@echo "  make ps               - List running containers"
	@echo "  make restart          - Restart all services (dev mode)"
	@echo "  make clean            - Stop and remove all containers, networks, volumes"
	@echo "  make scale-workers N=5 - Scale workers to N instances (default: 5)"
	@echo ""

# Development mode (hot reload)
dev:
	docker compose -f docker-compose.dev.yml up

dev-build:
	docker compose -f docker-compose.dev.yml up --build

dev-down:
	docker compose -f docker-compose.dev.yml down

dev-logs:
	docker compose -f docker-compose.dev.yml logs -f

# Production mode
prod:
	docker compose up

prod-build:
	docker compose up --build

prod-down:
	docker compose down

prod-logs:
	docker compose logs -f

# Service-specific logs
logs-api:
	docker compose -f docker-compose.dev.yml logs -f api

logs-worker:
	docker compose -f docker-compose.dev.yml logs -f worker

logs-scheduler:
	docker compose -f docker-compose.dev.yml logs -f scheduler

logs-redis:
	docker compose -f docker-compose.dev.yml logs -f redis

# Utility commands
ps:
	docker compose -f docker-compose.dev.yml ps

restart:
	docker compose -f docker-compose.dev.yml restart

clean:
	docker compose -f docker-compose.dev.yml down -v
	docker compose down -v
	@echo "Cleaned up all containers, networks, and volumes"

scale-workers:
	docker compose -f docker-compose.dev.yml up --scale worker=$(or $(N),5)

# Testing
test:
	@echo "Running tests in Docker (Go 1.23)..."
	@docker compose -f docker-compose.dev.yml run --rm --no-deps api go test ./...

test-verbose:
	@echo "Running tests with verbose output in Docker..."
	@docker compose -f docker-compose.dev.yml run --rm --no-deps api go test -v ./...

test-coverage:
	@echo "Running tests with coverage in Docker..."
	@docker compose -f docker-compose.dev.yml run --rm --no-deps api sh -c "go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep total"

test-local:
	@echo "Running tests locally (requires Go 1.21+)..."
	@go test ./...

test-local-verbose:
	@echo "Running tests with verbose output locally..."
	@go test -v ./...

test-local-coverage:
	@echo "Running tests with coverage locally..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out | grep total

# Quick aliases
up: dev
down: dev-down
build: dev-build
logs: dev-logs

