# Configuration

All configuration is loaded from environment variables by [`pkg/config/config.go`](../pkg/config/config.go).

Copy `.env.example` to `.env` and set values before running locally.

---

## Application

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | `development` | Gin mode (`development`, `release`, `test`) |
| `NODE_ID` | `node-1` | Unique identifier for this node; used by the event coordinator for partition ownership |
| `HTTP_PORT` | `:8080` | HTTP/WS/SSE listen address |
| `GRPC_PORT` | `:9090` | Reserved (not yet active) |
| `SSE_PORT` | `:8081` | Reserved (SSE currently shares HTTP_PORT) |

---

## PostgreSQL

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_DSN` | `postgres://user:pass@localhost/chatdb?sslmode=disable` | Full connection string |

The application opens two connections to Postgres: a `pgx/v5` pool (via [`pkg/client-database/postgresql/postgres.go`](../pkg/client-database/postgresql/postgres.go)) used by sqlc-generated queries, and a `database/sql` handle used by modules that predate the sqlc migration.

---

## Redis

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | `localhost:6379` | Host:port |
| `REDIS_PASSWORD` | _(empty)_ | Auth password |
| `REDIS_DB` | `0` | Database index |

Client: [`pkg/client-database/redis/redis.go`](../pkg/client-database/redis/redis.go)

Used for: JWT session storage, room cache, user-client presence cache.

---

## Kafka

| Variable | Default | Description |
|----------|---------|-------------|
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated broker list |
| `KAFKA_TOPICS_INCOMING` | `incoming` | Topic for new messages written by `MessageService` |
| `KAFKA_TOPICS_VALIDATED` | `validated` | Reserved |
| `KAFKA_TOPICS_SAVED` | `saved` | Reserved |
| `KAFKA_TOPICS_BROADCAST` | `broadcast` | Reserved |

Producer: [`internal/message/storage/MessageDistributor.go`](../internal/message/storage/MessageDistributor.go)  
Consumer: [`pkg/messaging/kafka/consumer.go`](../pkg/messaging/kafka/consumer.go)

---

## JWT

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | `my-super-secret-key-change-in-prod` | HMAC-SHA256 signing secret — **must be overridden in production** |

JWT implementation: [`pkg/auth/jwt.go`](../pkg/auth/jwt.go)

---

## File storage (local segment store)

| Variable | Default | Description |
|----------|---------|-------------|
| `FILE_BASE_PATH` | `./storage` | Root directory for local segment files |
| `FILE_SEGMENT_SIZE` | `1h` | Duration per segment file (e.g. `1h`, `30m`) |
| `FILE_BUFFER_SIZE` | `64MB` (parsed) | Write buffer size per segment |
| `FILE_FLUSH_TIMEOUT` | `100ms` | Max time before a buffer is force-flushed |

Storage adapter: [`internal/message/storage/fileStorage.go`](../internal/message/storage/fileStorage.go)

---

## S3 / object storage

| Variable | Default | Description |
|----------|---------|-------------|
| `AWS_REGION` | `us-east-1` | AWS region |
| `AWS_BUCKET` | `krampus-media` | Bucket name |
| `AWS_ACCESS_KEY_ID` | _(empty)_ | Static credential key ID |
| `AWS_SECRET_ACCESS_KEY` | _(empty)_ | Static credential secret |
| `AWS_ENDPOINT` | _(empty)_ | Custom endpoint URL (for MinIO or LocalStack) |

When `AWS_ACCESS_KEY_ID` is empty the SDK falls back to the default credential chain (IAM role, `~/.aws/credentials`, etc.).  
When `AWS_ENDPOINT` is set, path-style addressing is enabled automatically (needed for MinIO).

Client factory: [`internal/files/storage/client.go`](../internal/files/storage/client.go)  
S3 adapter: [`internal/files/storage/s3.go`](../internal/files/storage/s3.go)
