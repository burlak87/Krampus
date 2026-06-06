# Architecture

## Entry point

[`cmd/app/main.go`](../cmd/app/main.go) manually wires every dependency in order (no DI framework). Reading it top-to-bottom gives a complete picture of how the application assembles.

## Module layout

Every internal module follows the same four-layer layout:

```
internal/<module>/
  domain/     — entities, value objects, interfaces
  service/    — business logic
  storage/    — DB / cache adapters
  adapters/   — HTTP handlers, WebSocket / SSE servers
```

Modules:

| Module | Path | Role |
|--------|------|------|
| `auth` | [`internal/auth/`](../internal/auth/) | 2FA (TOTP) verify + enable |
| `user` | [`internal/user/`](../internal/user/) | Registration, login, logout, session, refresh token |
| `identity` | [`internal/identity/`](../internal/identity/) | JWT signing/verification; WebSocket auth service |
| `chat` | [`internal/chat/`](../internal/chat/) | Rooms (create / get / join), user-client tracking, Redis cache |
| `message` | [`internal/message/`](../internal/message/) | Send, persist, broadcast; outbox, idempotency, retry, DLQ, delivery status, replay |
| `events` | [`internal/events/`](../internal/events/) | In-process event bus, partition coordinator, projections, checkpoints, snapshots |
| `audit` | [`internal/audit/`](../internal/audit/) | Writes every event to `audit_logs` via the event bus |
| `search` | [`internal/search/`](../internal/search/) | Full-text indexing into `message_search_projection`; HTTP search handler |
| `moderation` | [`internal/moderation/`](../internal/moderation/) | Abuse scores, shadow bans, delivery suppressor, projection |
| `permissions` | [`internal/permissions/`](../internal/permissions/) | RBAC — roles, role_permissions, membership-level checks |
| `polls` | [`internal/polls/`](../internal/polls/) | Poll create, vote, projection, auto-close worker |
| `reactions` | [`internal/reactions/`](../internal/reactions/) | Per-message emoji reactions |
| `stickers` | [`internal/stickers/`](../internal/stickers/) | Sticker packs and individual sticker listing |
| `sync` | [`internal/sync/`](../internal/sync/) | Per-device sync cursor (`sync_state` table) |
| `invites` | [`internal/invites/`](../internal/invites/) | Deep-link invite token generation and extraction |
| `notifications` | [`internal/notifications/`](../internal/notifications/) | Push notification dispatch (FCM stub provider) |
| `retention` | [`internal/retention/`](../internal/retention/) | Policy-driven data retention + hard-delete worker |
| `media` | [`internal/media/`](../internal/media/) | Media file processing pipeline (upload → transcode → store) |
| `files` | [`internal/files/`](../internal/files/) | Chunked resumable upload sessions, S3 storage, quota |
| `profile` | [`internal/profile/`](../internal/profile/) | User avatar upload and profile retrieval |
| `platform` | [`internal/platform/`](../internal/platform/) | Supervisor — panic-recovering goroutine wrapper |

## Transport layer

Three transports share the same authenticated room model:

```
REST  GET|POST  /api/v1/…       — Gin router, AuthMiddleware on /chat group
WS             /ws              — WebSocketServer (gorilla/websocket)
SSE            /sse?room_id=…  — SSEServer (read-only; supports Last-Event-ID replay)
```

WebSocket and SSE share a single `ConnectionManager` instance (obtained via `wsServer.Manager()`).
The `TransportKind` field distinguishes the two client types inside the manager.

## Message pipeline (summary)

```
Client ──WS──▶ MessageService ──▶ PostgreSQL (sqlc)
                                 └──▶ Kafka "incoming"
                                          │
                          ┌──────────────┴──────────────┐
                    WS broadcaster              search / audit consumers
                    (ConnectionManager)          (events.Consumer polling loop)
```

Full details: [MessageFlow.md](MessageFlow.md).

## Event sourcing (summary)

`internal/events/` implements a partition-based event log with:
- `Bus` — synchronous in-process pub/sub
- `Coordinator` — acquires partition ownership via DB leases every 5 s
- `Consumer` — polls the `domain_events` table, checkpoints progress
- `Projector` — fan-out to multiple `Projection` implementations
- Snapshot + replay support for rebuilding projections from scratch

Full details: [EventSourcing.md](EventSourcing.md).

## Data stores

| Store | Purpose |
|-------|---------|
| PostgreSQL | Primary store — all persistent data; sqlc-generated queries |
| Redis | Sessions, room cache, user-client cache |
| Kafka | Async transport between write path and fan-out consumers |
| S3 (AWS / compatible) | Binary media and upload chunk storage |

Cassandra and MongoDB appear in `docker-compose.yml` but are not yet wired in application code.

## Shared packages

See [Packages.md](Packages.md) for `pkg/` documentation.
