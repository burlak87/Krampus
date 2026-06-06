# Event Sourcing

All event-sourcing code lives in [`internal/events/`](../internal/events/).

## Overview

The event layer provides an append-only log (`domain_events` table), a partition coordinator for distributed ownership, and a consumer loop that fans events out to projections. It runs independently of the HTTP/WebSocket request path.

## Core types

### `Event` — [`event.go`](../internal/events/event.go)

```go
type Event struct {
    ID            int64
    AggregateID   string
    AggregateType string
    EventType     string
    Payload       json.RawMessage
    EventVersion  int
    Sequence      int64
    CreatedAt     time.Time
}
```

### `Bus` — [`bus.go`](../internal/events/bus.go)

Synchronous in-process pub/sub. Handlers are called in subscription order within the publishing goroutine.

```go
bus := events.NewBus()
bus.Subscribe("message_created", myHandler)
bus.Publish(ctx, event)
```

Wired consumers in [`cmd/app/main.go`](../cmd/app/main.go):

| Event type | Consumer |
|------------|---------|
| `message_created` | `audit.Consumer`, `search.Consumer` |
| `moderation_action_created` | `audit.Consumer`, `moderation.Projection` |
| `poll_vote_cast` | `audit.Consumer` |
| `reaction_added` | `audit.Consumer` |

### `Coordinator` — [`coordinator.go`](../internal/events/coordinator.go)

Runs a ticker every 5 seconds and attempts to acquire ownership of each partition via `Ownership.AcquirePartition`. Ownership records are written to `event_partition_ownership` with a lease TTL. This prevents two nodes from processing the same partition simultaneously.

```go
coordinator := events.NewCoordinator(ownership, cfg.NodeID, 4) // 4 partitions
go supervisor.RunWorker(ctx, "event-coordinator", coordinator.Start)
```

### `Consumer` — [`consumer.go`](../internal/events/consumer.go)

Polls `domain_events` for new rows beyond the last checkpoint, batches them (up to `batchSize`), and calls `Projector.Project` for each batch.

```go
consumer := events.NewConsumer(sqlDB, projector, checkpointRepo, "audit", 100)
go supervisor.RunWorker(ctx, "audit-consumer", consumer.Start)
```

### `Projector` — [`projector.go`](../internal/events/projector.go)

Fans a batch of events to multiple `Projection` implementations.

```go
projector := events.NewProjector([]events.Projection{auditConsumer})
```

### `Projection` interface — [`projection.go`](../internal/events/projection.go)

```go
type Projection interface {
    Name() string
    Handle(ctx context.Context, event Event) error
}
```

Implemented by:
- [`internal/audit/consumer.go`](../internal/audit/consumer.go) — writes to `audit_logs`
- [`internal/search/consumer.go`](../internal/search/consumer.go) — indexes into `message_search_projection`
- [`internal/moderation/projection.go`](../internal/moderation/projection.go) — updates moderation state

### `CheckpointRepository` — [`checkpoints.go`](../internal/events/checkpoints.go)

Persists the last processed `sequence` per consumer name in `event_checkpoints`. Guarantees at-least-once processing across restarts.

### `Ownership` — [`ownership.go`](../internal/events/ownership.go)

Distributed partition lease management backed by `event_partition_ownership`. Uses `FOR UPDATE SKIP LOCKED` to prevent conflicts.

## Snapshots

[`snapshot.go`](../internal/events/snapshot.go) and [`snapshot_repository.go`](../internal/events/snapshot_repository.go) implement periodic aggregate snapshots stored in `event_snapshots`. [`rebuild.go`](../internal/events/rebuild.go) rebuilds projections from a snapshot plus subsequent events.

## Heartbeat and failover

[`heartbeat.go`](../internal/events/heartbeat.go) refreshes partition leases for owned partitions.  
[`failover.go`](../internal/events/failover.go) detects expired leases and triggers rebalancing.  
[`rebalancer.go`](../internal/events/rebalancer.go) redistributes partitions when a node joins or leaves.

## Replay

[`replay.go`](../internal/events/replay.go) allows replaying events from an arbitrary sequence number — used for cold-starting a new projection without losing history.

## Partition consumer

[`partition_consumer.go`](../internal/events/partition_consumer.go) ties ownership to consumption: a node only processes events from partitions it currently owns.

## Idempotency

[`idempotency.go`](../internal/events/idempotency.go) deduplicates event processing using a per-consumer idempotency key derived from `event.ID`.
