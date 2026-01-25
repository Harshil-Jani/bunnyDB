# =============================================================================
# BunnyDB Makefile
# =============================================================================
# Common commands for running BunnyDB locally
#
# Quick start:
#   make setup    # First-time setup (copies .env.example)
#   make up       # Start all services
#   make logs     # View logs
#   make down     # Stop all services
# =============================================================================

.PHONY: help setup up down restart logs logs-api logs-worker logs-ui \
        ps build rebuild clean dev status health

# Default target
help:
	@echo "BunnyDB - PostgreSQL CDC Replication"
	@echo ""
	@echo "Setup:"
	@echo "  make setup      - First-time setup (create .env from template)"
	@echo "  make build      - Build all Docker images"
	@echo "  make rebuild    - Force rebuild all images (no cache)"
	@echo ""
	@echo "Running:"
	@echo "  make up         - Start all services"
	@echo "  make down       - Stop all services"
	@echo "  make restart    - Restart all services"
	@echo "  make dev        - Start with dev profile (includes test DBs)"
	@echo "  make docs       - Start with docs profile (includes local docs)"
	@echo "  make all        - Start with all profiles (dev + docs)"
	@echo ""
	@echo "Monitoring:"
	@echo "  make ps         - Show running containers"
	@echo "  make status     - Show service status and health"
	@echo "  make logs       - Tail all logs"
	@echo "  make logs-api   - Tail API server logs"
	@echo "  make logs-worker - Tail worker logs"
	@echo "  make logs-ui    - Tail UI logs"
	@echo "  make health     - Check API health endpoint"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean      - Stop and remove volumes (WARNING: deletes data)"
	@echo ""
	@echo "URLs:"
	@echo "  UI:          http://localhost:3000"
	@echo "  API:         http://localhost:8112"
	@echo "  Temporal UI: http://localhost:8085"
	@echo "  Docs:        http://localhost:3001 (with docs profile)"

# =============================================================================
# Setup
# =============================================================================

setup:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env from .env.example"; \
		echo ""; \
		echo "IMPORTANT: Edit .env and change these values for production:"; \
		echo "  - BUNNY_JWT_SECRET"; \
		echo "  - BUNNY_ADMIN_USER"; \
		echo "  - BUNNY_ADMIN_PASSWORD"; \
		echo "  - BUNNY_CATALOG_PASSWORD"; \
	else \
		echo ".env already exists, skipping"; \
	fi

build:
	docker compose build

rebuild:
	docker compose build --no-cache

# =============================================================================
# Running
# =============================================================================

up:
	docker compose up -d
	@echo ""
	@echo "BunnyDB is starting..."
	@echo "  UI:          http://localhost:3000"
	@echo "  API:         http://localhost:8112"
	@echo "  Temporal UI: http://localhost:8085"
	@echo ""
	@echo "Default login: admin / admin"
	@echo "Run 'make logs' to view logs"

down:
	docker compose down

restart:
	docker compose restart

dev:
	docker compose --profile dev up -d
	@echo ""
	@echo "BunnyDB started with dev profile"
	@echo "  Source DB: localhost:5434 (postgres/sourcepass/source_db)"
	@echo "  Dest DB:   localhost:5435 (postgres/destpass/dest_db)"

docs:
	docker compose --profile docs up -d
	@echo ""
	@echo "BunnyDB started with docs profile"
	@echo "  Docs: http://localhost:3001"

all:
	docker compose --profile dev --profile docs up -d
	@echo ""
	@echo "BunnyDB started with all profiles"

# =============================================================================
# Monitoring
# =============================================================================

ps:
	docker compose ps

status:
	@echo "=== Container Status ==="
	@docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"

logs:
	docker compose logs -f

logs-api:
	docker compose logs -f bunny-api

logs-worker:
	docker compose logs -f bunny-worker

logs-ui:
	docker compose logs -f bunny-ui

health:
	@echo "Checking API health..."
	@curl -s http://localhost:8112/api/health | python3 -m json.tool 2>/dev/null || echo "API not responding"

# =============================================================================
# Cleanup
# =============================================================================

clean:
	@echo "WARNING: This will delete all BunnyDB data!"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	docker compose down -v
	@echo "All containers and volumes removed"
