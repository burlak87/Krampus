# Shared Packages (`pkg/`)

Reusable libraries shared across internal modules.

---

## `pkg/apperror` — Error handling

| File | Contents |
|------|----------|
| [`error.go`](../pkg/apperror/error.go) | `AppError` type with `Code` and `Message`; sentinel error codes (`ErrUnauthorized`, `ErrNotFound`, …) |
| [`middleware.go`](../pkg/apperror/middleware.go) | Gin middlewares: `ErrorMiddleware` (converts `AppError` to JSON), `AuthMiddleware` (JWT validation + session lookup), `CORSMiddleware` |
| [`retry.go`](../pkg/apperror/retry.go) | Retry helper used internally |

All HTTP handlers call `c.Error(apperror.New(...))` and `ErrorMiddleware` serialises it.

---

## `pkg/auth/jwt` — JWT

[`jwt.go`](../pkg/auth/jwt.go) — signs and verifies HMAC-SHA256 JWTs. Used by:
- `UserService` (issue tokens on login)
- `AuthMiddleware` (verify on every request)
- `WSAuthService` (verify on WebSocket upgrade)

---

## `pkg/client-database/postgresql` — PostgreSQL pool

[`postgresql/postgres.go`](../pkg/client-database/postgresql/postgres.go) — wraps `pgx/v5` pool creation with a configurable retry loop. Returns a `*pgxpool.Pool` shared by sqlc-generated code.

---

## `pkg/client-database/redis` — Redis client

[`redis/redis.go`](../pkg/client-database/redis/redis.go) — thin wrapper over `go-redis/v9`. Returns a struct with `RDB() *redis.Client` accessor.

---

## `pkg/compression` — Payload compression

Three algorithms available for message payload compression:

| File | Algorithm | Use case |
|------|-----------|----------|
| [`brotli.go`](../pkg/compression/brotli.go) | Brotli | Best ratio; used for HTTP responses |
| [`lz4.go`](../pkg/compression/lz4.go) | LZ4 | Low-latency real-time messages |
| [`zstd.go`](../pkg/compression/zstd.go) | Zstandard | Balanced; used for stored messages |

Compression algorithm is recorded in `BaseMessage.Metadata.Compression`.

---

## `pkg/config` — Configuration

[`config.go`](../pkg/config/config.go) — loads all settings from environment variables. See [Configuration.md](Configuration.md) for the full reference.

---

## `pkg/crypto/aesgcm` — Encryption

[`aesgcm.go`](../pkg/crypto/aesgcm.go) — AES-256-GCM encrypt / decrypt helpers. Used for end-to-end encrypted file uploads. The encryption nonce is stored alongside the file metadata.

---

## `pkg/ctxmeta` — Request context metadata

| File | Contents |
|------|----------|
| [`context.go`](../pkg/ctxmeta/context.go) | Typed helpers to get/set `user_id`, `trace_id`, `request_id` from `context.Context` |
| [`middleware.go`](../pkg/ctxmeta/middleware.go) | Gin middleware that extracts JWT claims and populates context values |

---

## `pkg/hash` — Hashing utilities

[`hash.go`](../pkg/hash/hash.go) — bcrypt password hashing and SHA-256 content hashing helpers.

---

## `pkg/logging` — Structured logging

[`logging.go`](../pkg/logging/logging.go) — initialises a logrus logger with JSON formatter. `GetLogger()` returns the global singleton. Log output is consumed by Promtail and forwarded to Loki.

---

## `pkg/messaging/kafka` — Kafka client

| File | Contents |
|------|----------|
| [`producer.go`](../pkg/messaging/kafka/producer.go) | Synchronous Kafka producer |
| [`consumer.go`](../pkg/messaging/kafka/consumer.go) | Consumer with per-topic handler registration; used by `WebSocketServer` for broadcast |

---

## `pkg/retry` — Retry with back-off

[`backoff.go`](../pkg/retry/backoff.go) — generic retry helper with configurable max attempts, initial delay, multiplier, and jitter. Used by database connection setup and delivery workers.

---

## `pkg/server` — HTTP server helpers

[`gin.go`](../pkg/server/gin.go) — helper to build a standard `*gin.Engine` with default middleware stack applied.

---

## `pkg/types/ids` — Typed identifiers

[`ids.go`](../pkg/types/ids.go) — newtype aliases for `string`:

```go
type UserID    string
type RoomID    string
type MessageID string
```

Using distinct types prevents accidental argument transposition at compile time.

---

## `pkg/utils` — Miscellaneous utilities

| File | Contents |
|------|----------|
| [`random.go`](../pkg/utils/random.go) | Cryptographically secure random token generator |
| [`repeatable.go`](../pkg/utils/repeatable.go) | `Repeat(n, fn)` helper for retry loops |

---

## `pkg/validation` — Input validation

[`validation.go`](../pkg/validation/validation.go) — common field validators (email format, password strength, UUID format) used in service-layer validation before hitting the database.
