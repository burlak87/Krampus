.PHONY: build run up down db-reset sqlc lint frontend test smoke

# Build the Go binary
build:
	go build -v ./cmd/app

# Run locally (reads .env)
run:
	go run ./cmd/app

# Run all Go tests
test:
	go test ./...

# API contract smoke test against a running backend (see TESTING.md)
smoke:
	./scripts/smoke_test.sh

# Start local infrastructure
up:
	docker compose up -d

# Stop local infrastructure
down:
	docker compose down

# Drop all volumes and recreate schema (use after schema changes)
db-reset:
	docker compose down -v
	docker compose up -d

# Regenerate sqlc query bindings
sqlc:
	sqlc generate

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

# Run the Nuxt dev frontend (http://localhost:3000)
frontend:
	cd frontend && npm install && npm run dev

.DEFAULT_GOAL := build
