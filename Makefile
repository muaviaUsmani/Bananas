.PHONY: help dev prod up down build logs clean restart ps scale-workers test test-verbose test-coverage test-local test-local-verbose test-local-coverage es-start es-stop es-init es-clean es-logs es-status

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
	@echo "Elasticsearch Commands (Logging):"
	@echo "  make es-start         - Start Elasticsearch and Kibana"
	@echo "  make es-stop          - Stop Elasticsearch and Kibana"
	@echo "  make es-init          - Initialize Elasticsearch (index templates, ILM)"
	@echo "  make es-clean         - Clean Elasticsearch data and stop"
	@echo "  make es-logs          - View Elasticsearch logs"
	@echo "  make es-status        - Check Elasticsearch health status"
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

# Elasticsearch commands
es-start:
	@echo "Starting Elasticsearch and Kibana..."
	docker compose -f docker-compose.elasticsearch.yml up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@echo ""
	@echo "✅ Elasticsearch: http://localhost:9200"
	@echo "✅ Kibana: http://localhost:5601"
	@echo ""
	@echo "Run 'make es-init' to initialize index templates and ILM policies"

es-stop:
	@echo "Stopping Elasticsearch and Kibana..."
	docker compose -f docker-compose.elasticsearch.yml down
	@echo "✅ Stopped"

es-init:
	@echo "Initializing Elasticsearch..."
	@bash scripts/init-elasticsearch.sh
	@echo ""
	@echo "✅ Elasticsearch is ready for logging!"

es-clean:
	@echo "Cleaning Elasticsearch data and stopping services..."
	docker compose -f docker-compose.elasticsearch.yml down -v
	@echo "✅ Cleaned"

es-logs:
	docker compose -f docker-compose.elasticsearch.yml logs -f elasticsearch

es-status:
	@echo "Elasticsearch Cluster Health:"
	@curl -s http://localhost:9200/_cluster/health?pretty || echo "❌ Elasticsearch is not running (run 'make es-start')"
	@echo ""
	@echo "Indices:"
	@curl -s http://localhost:9200/_cat/indices/bananas-logs-*?v || echo "No indices found yet"

