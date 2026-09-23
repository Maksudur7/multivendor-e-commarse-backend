# ──────────────────────────────────────────────────────────────────────────────
# Enterprise E-Commerce Backend — Makefile
# ──────────────────────────────────────────────────────────────────────────────

APP_NAME    := ecom-backend
BINARY_DIR  := bin
BINARY      := $(BINARY_DIR)/$(APP_NAME)
MAIN_PKG    := ./cmd/server
MIGRATE_BIN := migrate
SQLC_BIN    := sqlc

DB_URL ?= $(shell grep DATABASE_URL .env | cut -d '=' -f2-)

.PHONY: all build run dev docker-up docker-down migrate-up migrate-down migrate-create sqlc tidy test lint clean help

## Default target
all: build

## ── BUILD ────────────────────────────────────────────────────────────────────
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BINARY_DIR)
	@go build -ldflags="-s -w" -o $(BINARY) $(MAIN_PKG)
	@echo "Binary: $(BINARY)"

## Run binary directly
run: build
	@./$(BINARY)

## Run with hot-reload (requires: go install github.com/air-verse/air@latest)
dev:
	@which air > /dev/null 2>&1 || go install github.com/air-verse/air@latest
	@air -c .air.toml

## ── DOCKER ───────────────────────────────────────────────────────────────────
docker-up:
	@echo "Starting local services (PostgreSQL, Redis, MeiliSearch)..."
	@docker compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@docker compose ps

docker-down:
	@docker compose down

docker-reset:
	@docker compose down -v
	@docker compose up -d

## ── DATABASE MIGRATIONS ──────────────────────────────────────────────────────
migrate-up:
	@echo "Running migrations UP..."
	@$(MIGRATE_BIN) -path=./db/migrations -database "$(DB_URL)" up

migrate-down:
	@echo "Rolling back 1 migration..."
	@$(MIGRATE_BIN) -path=./db/migrations -database "$(DB_URL)" down 1

migrate-down-all:
	@echo "Rolling back ALL migrations..."
	@$(MIGRATE_BIN) -path=./db/migrations -database "$(DB_URL)" down -all

migrate-create:
	@read -p "Migration name: " name; \
	$(MIGRATE_BIN) create -ext sql -dir ./db/migrations -seq $$name

migrate-status:
	@$(MIGRATE_BIN) -path=./db/migrations -database "$(DB_URL)" version

## ── SQLC ─────────────────────────────────────────────────────────────────────
sqlc:
	@echo "Generating type-safe Go code from SQL..."
	@$(SQLC_BIN) generate

sqlc-verify:
	@$(SQLC_BIN) verify

## ── CODE QUALITY ─────────────────────────────────────────────────────────────
tidy:
	@go mod tidy

test:
	@go test ./... -v -race -count=1

test-cover:
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out

lint:
	@which golangci-lint > /dev/null 2>&1 || \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@golangci-lint run ./...

## ── TOOLS ────────────────────────────────────────────────────────────────────
install-tools:
	@echo "Installing development tools..."
	@go install github.com/air-verse/air@latest
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "All tools installed."

swag:
	@which swag > /dev/null 2>&1 || go install github.com/swaggo/swag/cmd/swag@latest
	@swag init -g cmd/server/main.go -o docs

## ── CLEANUP ──────────────────────────────────────────────────────────────────
clean:
	@rm -rf $(BINARY_DIR) coverage.out

## ── HELP ─────────────────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "$(APP_NAME) — Available Commands:"
	@echo ""
	@echo "  make build         Build the binary"
	@echo "  make run           Build and run"
	@echo "  make dev           Run with hot-reload (air)"
	@echo "  make docker-up     Start local services"
	@echo "  make docker-down   Stop local services"
	@echo "  make migrate-up    Run all pending migrations"
	@echo "  make migrate-down  Rollback 1 migration"
	@echo "  make migrate-create  Create new migration file"
	@echo "  make sqlc          Generate Go code from SQL"
	@echo "  make tidy          go mod tidy"
	@echo "  make test          Run all tests"
	@echo "  make lint          Run linter"
	@echo "  make install-tools Install dev tools"
	@echo "  make swag          Generate Swagger docs"
	@echo ""
