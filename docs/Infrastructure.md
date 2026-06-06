# Infrastructure

## Docker Compose

### Development — [`docker-compose.yml`](../docker-compose.yml)

Start all backing services:

```bash
docker-compose up -d
```

| Service | Image | Port | Purpose |
|---------|-------|------|---------|
| `postgres` | `postgres:15` | 5432 | Primary database |
| `redis` | `redis:7` | 6379 | Sessions, caches |
| `kafka` + `zookeeper` | `bitnami/kafka` | 9092 | Message transport |
| `cassandra` | `cassandra:4` | 9042 | Not yet wired |
| `mongo` | `mongo:6` | 27017 | Not yet wired |
| `grafana` | `grafana/grafana` | 3000 | Dashboards |
| `loki` | `grafana/loki` | 3100 | Log aggregation |
| `promtail` | `grafana/promtail` | — | Log shipper (reads container logs) |
| `tempo` | `grafana/tempo` | 3200 | Distributed tracing |
| `mimir` | `grafana/mimir` | 9009 | Long-term metrics |

### Production — [`docker-compose.prod.yml`](../docker-compose.prod.yml)

Extends the development file with production-grade settings (resource limits, restart policies, external networks).

---

## Observability stack

### Grafana — dashboards

Provisioned automatically from [`grafana-provisioning/`](../grafana-provisioning/).  
Access at `http://localhost:3000` (default credentials: `admin` / `admin`).

### Loki — log aggregation

Config: [`loki-config.yaml`](../loki-config.yaml)  
Promtail ships structured JSON logs (written by [`pkg/logging/logging.go`](../pkg/logging/logging.go) via logrus) into Loki.

### Promtail — log shipper

Config: [`promtail-config.yaml`](../promtail-config.yaml)  
Tails Docker container log files and forwards to Loki.

### Tempo — distributed tracing

Config: [`tempo.yaml`](../tempo.yaml)  
Accepts OTLP spans. Trace IDs are propagated via `BaseMessage.Metadata.TraceID` ([`internal/message/domain/message.go`](../internal/message/domain/message.go)).

### Mimir — long-term metrics

Config: [`mimir.yaml`](../mimir.yaml)  
Compatible with the Prometheus remote-write protocol; used as Grafana's long-term metrics backend.

---

## Ansible deployment

| File | Purpose |
|------|---------|
| [`playbook.yml`](../playbook.yml) | Main deployment playbook |
| [`inventory`](../inventory) | Host inventory |
| [`ansible.cfg`](../ansible.cfg) | Ansible configuration |

The playbook provisions the target host, copies application artefacts, and starts services via Docker Compose.

---

## Dockerfile

[`Dockerfile`](../Dockerfile) — multi-stage build:

1. **Builder** — `golang:1.22-alpine`: runs `go build -v ./cmd/app`
2. **Final** — `alpine:latest`: copies the compiled binary, exposes `:8080`

Build and run:
```bash
docker build -t krampus .
docker run -p 8080:8080 --env-file .env krampus
```

---

## Build and run locally

```bash
# Build
make build           # go build -v ./cmd/app

# Run (reads .env automatically if using a wrapper like godotenv)
go run ./cmd/app

# Regenerate sqlc query bindings after schema/query changes
sqlc generate
```
