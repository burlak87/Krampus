# Krampus — Documentation Index

Krampus is a real-time messaging backend built in Go for an educational organisation (АНО ДПО «Академия ТОП Великий Новгород»). It replaces blocked or unavailable messengers for students and teachers.

## Documents

| File | Description |
|------|-------------|
| [Architecture.md](Architecture.md) | Module layout, layered design, dependency graph |
| [API.md](API.md) | Full HTTP REST, WebSocket, and SSE endpoint reference |
| [Configuration.md](Configuration.md) | All environment variables and their defaults |
| [Database.md](Database.md) | Schema tables, indexes, and sqlc workflow |
| [MessageFlow.md](MessageFlow.md) | End-to-end message lifecycle from client to broadcast |
| [EventSourcing.md](EventSourcing.md) | Event bus, coordinator, projections, checkpoints |
| [Infrastructure.md](Infrastructure.md) | Docker Compose services, observability stack, Ansible deployment |
| [Packages.md](Packages.md) | Shared `pkg/` library reference |

## Quick start

```bash
cp .env.example .env          # fill in credentials
docker-compose up -d          # start Postgres, Redis, Kafka, …
go run ./cmd/app              # start the server on :8080
```

See [Configuration.md](Configuration.md) for all available env vars.
