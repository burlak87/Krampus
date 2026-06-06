# Krampus

Real-time messaging backend for an educational organisation.  
Replaces blocked/unavailable messengers for students and teachers at АНО ДПО «Академия ТОП Великий Новгород».

---

## Technology stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.22 |
| HTTP / REST | Gin |
| Real-time | WebSocket (gorilla/websocket), SSE |
| Primary store | PostgreSQL 17 (sqlc-generated queries) |
| Cache / sessions | Redis 7 |
| Message transport | Apache Kafka |
| Object storage | MinIO (S3-compatible) |
| Query generation | sqlc |

---

## Architecture

```
internal/<module>/
  domain/     — entities, interfaces
  service/    — business logic
  storage/    — DB / cache adapters
  adapters/   — HTTP handlers, WebSocket, SSE
```

Full architecture reference: [docs/Architecture.md](docs/Architecture.md)

---

## Quick start (local)

### 1. Copy environment file

```bash
cp .env.example .env
# Edit .env if needed — defaults work out of the box with docker-compose
```

### 2. Start infrastructure

```bash
make up
# Starts: postgres, redis, zookeeper, kafka, minio + auto-creates the S3 bucket
```

### 3. Run the backend

```bash
make run
# App listens on http://localhost:8080
```

### 4. Open the test frontend

```bash
make frontend
# Serves frontend/ at http://localhost:8000
# Open http://localhost:8000 in a browser
```

---

## Makefile targets

| Target | Description |
|--------|-------------|
| `make build` | Compile binary to `./krampus` |
| `make run` | Run with `go run ./cmd/app` |
| `make up` | `docker compose up -d` |
| `make down` | `docker compose down` |
| `make db-reset` | Drop all volumes and recreate schema |
| `make sqlc` | Regenerate `internal/sqlc/` from SQL files |
| `make lint` | Run golangci-lint |
| `make frontend` | Serve `frontend/` on port 8000 |

---

## Resetting the database

If you change `sql/schema/init.sql`, volumes must be dropped so the init script re-runs:

```bash
make db-reset
```

---

## Environment variables

See [docs/Configuration.md](docs/Configuration.md) for the full reference.  
The `.env.example` file contains all required variables with safe local defaults.

---

## API documentation

See [docs/API.md](docs/API.md) for:
- All REST endpoints with request / response schemas
- WebSocket message type catalogue
- SSE event format and reconnect protocol
- Error code table
- Authentication flow diagram

---

## Project structure

```
cmd/app/          — entry point (main.go)
internal/         — application modules
  auth/           — login, 2FA, refresh tokens
  user/           — registration, sessions
  chat/           — rooms, message routing
  message/        — WebSocket + SSE servers, delivery
  search/         — full-text search indexer
  moderation/     — ban, mute, shadow-ban
  notifications/  — FCM push notifications
  events/         — event sourcing layer
  media/          — media processing pipeline
  files/          — chunked upload workers
  profile/        — user profiles and avatars
  … (15 more modules)
pkg/              — shared libraries
  apperror/       — error types + Gin middleware
  config/         — env-based configuration
  logging/        — logrus JSON logger
  messaging/kafka — Kafka producer + consumer
  compression/    — brotli / lz4 / zstd
sql/              — schema and sqlc queries
frontend/         — vanilla HTML/JS test client
docs/             — detailed documentation
```

---

## Diploma project

Speciality: **09.02.07 Информационные системы и программирование**  
Organisation: Новгородский государственный университет, Политехнический колледж
