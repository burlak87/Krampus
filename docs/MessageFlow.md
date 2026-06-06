# Message Flow

This document traces the path of a message from the moment a client sends it until all subscribers receive it.

## 1. Client sends over WebSocket

The client opens a WebSocket connection to `/ws` with `?room_id=<id>&token=<jwt>`.

`WebSocketServer.HandleWebSocket` ([`internal/message/adapters/WebSocketServer.go`](../internal/message/adapters/WebSocketServer.go)):
1. Authenticates the request via `WSAuthService.AuthenticateWebSocket` (JWT + Redis session check).
2. Creates a `WebSocketClient` and registers it with `ConnectionManager`.
3. Starts a read loop that deserialises each incoming frame into a `BaseMessage`.

## 2. MessageService saves and publishes

`MessageService.HandleMessage` ([`internal/message/service/messageService.go`](../internal/message/service/messageService.go)):

```
incoming BaseMessage
  │
  ├─ Validate (required fields, timestamp range)
  ├─ Idempotency check (message_idempotency table) → dedup duplicate sends
  ├─ IsRealtimeOnly? → skip DB write, go straight to broadcast
  │
  ├─ Save to PostgreSQL via sqlc (messages table)
  ├─ Write outbox entry (message_outbox) in same transaction
  ├─ Write to file-backed secondary store (async, non-blocking)
  │
  └─ Publish to Kafka "incoming" topic via MessageDistributor
```

The **outbox worker** ([`internal/message/service/outboxWorker.go`](../internal/message/service/outboxWorker.go)) drains `message_outbox` and re-publishes to Kafka for at-least-once delivery guarantee.

## 3. Kafka fan-out

The `kafka.Consumer` ([`pkg/messaging/kafka/consumer.go`](../pkg/messaging/kafka/consumer.go)) receives from the `incoming` topic and delivers to all registered subscribers:

| Subscriber | File | Action |
|------------|------|--------|
| WebSocket broadcaster | `ConnectionManager.RouteMessage` | Delivers to every room member connected via WS or SSE |
| Search consumer | [`internal/search/consumer.go`](../internal/search/consumer.go) | Indexes message content into `message_search_projection` |
| Audit consumer | [`internal/audit/consumer.go`](../internal/audit/consumer.go) | Writes a row to `audit_logs` |

## 4. ConnectionManager broadcast

`ConnectionManager.RouteMessage` ([`internal/message/adapters/WebSocketManager.go`](../internal/message/adapters/WebSocketManager.go)):

1. Checks the **shadow-ban suppressor** — if the sending user is shadow-banned, the broadcast is silently suppressed (message is saved but recipients don't see it).
2. Looks up all clients in the room.
3. Calls `client.Send(msg)` for each, which serialises to JSON and writes to the WebSocket or SSE channel.

## 5. SSE clients

`SSEServer` ([`internal/message/adapters/SSEServer.go`](../internal/message/adapters/SSEServer.go)) registers SSE clients with the same `ConnectionManager` using `TransportKind = "sse"`. Messages are written as standard `data:` SSE lines.

On reconnect, the client provides `Last-Event-ID`. The server calls `ReplayRepository.GetMessagesAfterSequence` to catch up any missed messages.

## 6. Delivery tracking

After broadcast, the server records each delivery attempt in `message_delivery_status`. Failures feed the **retry worker** ([`internal/message/service/retryWorker.go`](../internal/message/service/retryWorker.go)), which retries with exponential back-off. After exhausting retries, the message moves to the DLQ ([`internal/message/storage/DLQStorage.go`](../internal/message/storage/DLQStorage.go)).

## Realtime-only messages

Message types `typing`, `video_call`, `cursor`, `presence_realtime`, `game`, `webrtc_signal` are broadcast immediately without hitting Postgres. The `IsRealtimeOnly()` check in `BaseMessage` ([`internal/message/domain/message.go`](../internal/message/domain/message.go)) controls this.

## Message types

| Type | Persisted | Description |
|------|-----------|-------------|
| `text` | Yes | Plain or formatted text message |
| `file` | Yes | File attachment reference |
| `system` | Yes | Server-generated room events |
| `typing` | No | Typing indicator |
| `read_receipt` | Yes | Read confirmation |
| `ack` / `ack_*` | No | Delivery acknowledgement signals |
| `video_call` | No | WebRTC signalling |
| `cursor` | No | Shared cursor position |
| `game` | No | In-chat mini-game state |
| `webrtc_signal` | No | Generic WebRTC data |

## Rate limiting

`RateLimiterService` ([`internal/message/service/rateLimiterService.go`](../internal/message/service/rateLimiterService.go)) enforces per-user send quotas using a Redis token-bucket implementation.

## Message scheduling

`internal/message/scheduler/` provides scheduled message delivery: messages stored with a future `send_at` timestamp are polled and delivered at the appropriate time.
