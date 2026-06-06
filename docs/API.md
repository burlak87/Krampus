# API Reference

Base URL: `http://<host>:8080`

All endpoints under `/api/v1/chat/` require a valid JWT in the `Authorization: Bearer <token>` header, enforced by `AuthMiddleware` ([`pkg/apperror/middleware.go`](../pkg/apperror/middleware.go)).

---

## Health

### `GET /health`

Returns server status.

**Response `200`**
```json
{ "status": "ok", "timestamp": "2026-06-04T12:00:00Z" }
```

---

## Authentication — `/api/v1/auth`

Handlers: [`internal/user/adapters/userHandler.go`](../internal/user/adapters/userHandler.go), [`internal/user/adapters/refreshTokenHandler.go`](../internal/user/adapters/refreshTokenHandler.go), [`internal/auth/adapters/twofaHandler.go`](../internal/auth/adapters/twofaHandler.go)

### `POST /api/v1/auth/signup`

Register a new user.

**Request**
```json
{ "username": "alice", "email": "alice@example.com", "password": "s3cr3t" }
```

**Response `201`** — returns the created `User` object.

---

### `POST /api/v1/auth/signin`

Login. Returns tokens or, if 2FA is enabled, a temporary token.

**Request**
```json
{ "email": "alice@example.com", "password": "s3cr3t" }
```

**Response `200` — tokens**
```json
{ "access_token": "eyJ…", "refresh_token": "eyJ…" }
```

**Response `200` — 2FA required**
```json
{ "temp_token": "eyJ…", "requires_two_fa": true }
```

---

### `POST /api/v1/auth/logout`

Invalidates the session. Requires `user_id` in context (i.e. must be authenticated).

**Request**
```json
{ "password": "s3cr3t" }
```

**Response `200`**
```json
{ "success": true }
```

---

### `POST /api/v1/auth/refresh`

Exchange a refresh token for a new access token.

**Request**
```json
{ "refresh_token": "eyJ…" }
```

**Response `200`** — new `TokenResponse`.

---

### `POST /api/v1/auth/verify`

Verify a 2FA TOTP code using the temporary token from `/signin`.

**Request**
```json
{ "temp_token": "eyJ…", "code": "123456" }
```

**Response `200`** — full `TokenResponse`.

---

### `POST /api/v1/auth/enable`

Enable 2FA for the authenticated user.

**Response `200`**
```json
{ "success": true }
```

---

## Chat — `/api/v1/chat` _(requires auth)_

Handler: [`internal/chat/adapters/chatHandler.go`](../internal/chat/adapters/chatHandler.go)

### Messages

#### `POST /api/v1/chat/messages`

Send a message to a room.

**Request**
```json
{
  "room_id": "…",
  "type": "text",
  "payload": { "text": "Hello!" }
}
```

**Response `200`** — echo of the saved message.

---

#### `GET /api/v1/chat/rooms/:room_id/messages`

Fetch recent messages in a room.

**Query params**

| Param | Type | Description |
|-------|------|-------------|
| `limit` | int | Max results (default 50) |

**Response `200`**
```json
[{ "id": "…", "type": "text", "user_id": "…", "payload": {…}, "timestamp": 1234567890 }]
```

---

#### `GET /api/v1/chat/rooms/:room_id/search`

Full-text search over messages in a room (PostgreSQL `tsvector`).
Indexer: [`internal/search/indexer.go`](../internal/search/indexer.go)

**Query params**

| Param | Type | Description |
|-------|------|-------------|
| `q` | string | Search query |
| `limit` | int | Max results (default 20) |

**Response `200`**
```json
[{ "message_id": "…", "room_id": "…", "user_id": "…", "content": "…" }]
```

---

### Rooms

#### `POST /api/v1/chat/rooms`

Create a new room.

**Request**
```json
{ "type": "group", "name": "Study Group A", "members": ["uid1", "uid2"] }
```

The creator is automatically added to `members`. A client-supplied `id` is
required.

**Response `201`** — created `Room` object.

---

#### `GET /api/v1/chat/rooms`

List rooms the authenticated user is a member of.

**Response `200`** — array of `Room` objects.

---

#### `GET /api/v1/chat/rooms/:room_id`

Get room details.

**Response `200`** — `Room` object.

---

#### `POST /api/v1/chat/rooms/join`

Join a room and gain membership. Accepts either a `krampus://join/<room_id>`
deep link or a bare room id as `token`. Membership is what `/ws` and `/call/ws`
authorize against, so this is the prerequisite for chatting/calling in a room
you didn't create.
Token extraction: [`internal/invites/deeplinks.go`](../internal/invites/deeplinks.go)

**Request**
```json
{ "token": "krampus://join/<room_id>" }
```

**Response `200`**
```json
{ "status": "joined", "room": { … } }
```

---

### Users

#### `GET /api/v1/chat/users`

List users (for rosters / member pickers).

**Query params**

| Param | Type | Description |
|-------|------|-------------|
| `limit` | int | Max results (default 100, max 500) |
| `offset` | int | Pagination offset (default 0) |

**Response `200`** — array of `User` objects.

---

#### `GET /api/v1/chat/users/:user_id`

Get a user's public profile (username, status).

**Response `200`** — `User` object.

---

### Polls

#### `POST /api/v1/chat/rooms/:room_id/polls/:poll_id/vote`

Cast a vote on a poll.
Service: [`internal/polls/service.go`](../internal/polls/service.go)

**Request**
```json
{ "option_id": "opt1" }
```

**Response `200`**
```json
{ "status": "voted" }
```

---

#### `GET /api/v1/chat/rooms/:room_id/polls/:poll_id`

Get poll results.
Projection: [`internal/polls/service.go`](../internal/polls/service.go) (`Projection`)

**Response `200`**
```json
{ "poll_id": "…", "options": [{ "id": "opt1", "votes": 5 }] }
```

---

### Reactions

#### `POST /api/v1/chat/messages/:message_id/reactions`

Add an emoji reaction to a message.
Service: [`internal/reactions/service.go`](../internal/reactions/service.go)

**Request**
```json
{ "emoji": "👍" }
```

**Response `200`**
```json
{ "status": "ok" }
```

---

### Stickers

#### `GET /api/v1/chat/stickers`

List all sticker packs with their stickers.
Service: [`internal/stickers/service.go`](../internal/stickers/service.go)

**Response `200`**
```json
[{ "id": "pack1", "title": "Fun Pack", "stickers": [{ "id": "s1", "emoji": "🎉" }] }]
```

---

### Sync

#### `GET /api/v1/chat/sync`

Return the device's last-seen event sequence for gap detection on reconnect.
Service: [`internal/sync/service.go`](../internal/sync/service.go)

**Query params**

| Param | Description |
|-------|-------------|
| `device_id` | Client device identifier |

**Response `200`**
```json
{ "last_event_sequence": 4201 }
```

---

### Moderation

#### `POST /api/v1/chat/moderation/ban`

Ban a user. Records a moderation action and adds a shadow-ban entry.
Tools: [`internal/moderation/tools.go`](../internal/moderation/tools.go)

**Request**
```json
{ "user_id": "…", "reason": "spam" }
```

**Response `200`**
```json
{ "status": "banned" }
```

---

#### `POST /api/v1/chat/moderation/mute`

Mute a user until a given time.

**Request**
```json
{ "user_id": "…", "until": "2026-07-01T00:00:00Z" }
```

**Response `200`**
```json
{ "status": "muted" }
```

---

### Profile

#### `POST /api/v1/chat/profile/avatar`

Upload (register) a media file as the caller's avatar.
Service: [`internal/profile/avatar/service.go`](../internal/profile/avatar/service.go)

**Request**
```json
{ "media_id": "…", "media_type": "image/webp" }
```

**Response `200`**
```json
{ "status": "ok" }
```

---

#### `GET /api/v1/chat/profile/:user_id`

Get a user's profile (currently returns avatar media ID).

**Response `200`**
```json
{ "user_id": "…", "avatar_media_id": "…" }
```

---

## WebSocket — `/ws`

Server: [`internal/message/adapters/WebSocketServer.go`](../internal/message/adapters/WebSocketServer.go)

**Upgrade**: standard `GET /ws` with `Upgrade: websocket`. Authentication is performed inside the upgrade handler via `WSAuthService` (JWT + session check).

**Query params**

| Param | Description |
|-------|-------------|
| `room_id` | Room to subscribe to |
| `token` | JWT access token (alternative to `Authorization` header) |

### Client → Server message

```json
{
  "id": "uuid",
  "type": "text",
  "user_id": "…",
  "room_id": "…",
  "timestamp": 1717488000000000000,
  "version": 1,
  "payload": { "text": "Hello" },
  "metadata": {}
}
```

Supported `type` values: `text`, `file`, `typing`, `read_receipt`, `ack`, `video_call`, `cursor`, `webrtc_signal`, `game`.

Types flagged `IsRealtimeOnly()` are broadcast without being persisted to Postgres.

### Server → Client message

Same `BaseMessage` schema. System messages use `type: "system"`.

### Delivery acknowledgement

The server tracks delivery status per client (table `message_delivery_status`). Failed deliveries are queued in a retry table and, after exhausting retries, moved to the DLQ.

---

## SSE — `/sse`

Server: [`internal/message/adapters/SSEServer.go`](../internal/message/adapters/SSEServer.go)

Read-only stream. Each event is a standard SSE `data:` line containing a JSON-encoded `BaseMessage`.

**Query params**

| Param | Description |
|-------|-------------|
| `room_id` | Room to subscribe to |
| `token` | JWT access token |

**Reconnect / gap replay**: on reconnect the client sends the `Last-Event-ID` header (or `last_event_id` query param). The server replays any messages with sequence > that value from the `replayRepository`.

---

## Error format

All REST error responses follow the `AppError` schema ([`pkg/apperror/error.go`](../pkg/apperror/error.go)):

```json
{ "code": "UNAUTHORIZED", "message": "token expired" }
```

---

## Error code table

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `INVALID_MESSAGE` | 400 | Malformed request body or missing required field |
| `UNAUTHORIZED` | 401 | Missing or expired JWT |
| `FORBIDDEN` | 403 | Authenticated but not permitted for this resource |
| `ROOM_NOT_FOUND` | 404 | Room does not exist |
| `USER_NOT_FOUND` | 404 | User does not exist |
| `DUPLICATE_ERROR` | 409 | Resource already exists (e.g. duplicate username) |
| `VALIDATION_ERROR` | 422 | Input failed validation rules |
| `RATE_LIMIT` | 429 | Too many requests |
| `PAYLOAD_TOO_LARGE` | 413 | Request body exceeds limit |
| `TIMEOUT_ERROR` | 504 | Upstream dependency timed out |
| `STORAGE_ERROR` | 500 | Database or object-store operation failed |
| `CONNECTION_ERROR` | 500 | Could not reach a required backing service |
| `INTERNAL_ERROR` | 500 | Unclassified server-side failure |

---

## Authentication flow

```
Client                        Server
  │                              │
  │  POST /auth/signup           │
  │ ────────────────────────►   │  hash password, insert users row
  │ ◄────────────────────────   │  201 { user_id, username, email }
  │                              │
  │  POST /auth/signin           │
  │ ────────────────────────►   │  verify hash, issue tokens
  │ ◄────────────────────────   │  200 { access_token, refresh_token }
  │   OR (2FA enabled)           │
  │ ◄────────────────────────   │  200 { temp_token, requires_two_fa: true }
  │                              │
  │  POST /auth/verify           │  (only when 2FA required)
  │  { temp_token, code }        │
  │ ────────────────────────►   │  verify TOTP code
  │ ◄────────────────────────   │  200 { access_token, refresh_token }
  │                              │
  │  GET /chat/…                 │  regular authenticated request
  │  Authorization: Bearer <at>  │
  │ ────────────────────────►   │  AuthMiddleware validates JWT + Redis session
  │                              │
  │  POST /auth/refresh          │  access token near expiry
  │  { refresh_token }           │
  │ ────────────────────────►   │  verify refresh token, issue new pair
  │ ◄────────────────────────   │  200 { access_token, refresh_token }
  │                              │
  │  POST /auth/logout           │
  │  { password }                │
  │ ────────────────────────►   │  delete Redis session + refresh token
  │ ◄────────────────────────   │  200 { success: true }
```

Token lifetimes:
- **Access token** — 15 minutes (HS256 JWT, verified by `AuthMiddleware`)
- **Refresh token** — 7 days (stored in `refresh_tokens` table)
- **Temp token (2FA)** — 10 minutes

---

## WebSocket message type catalogue

All messages use the `BaseMessage` envelope ([`internal/message/domain/message.go`](../internal/message/domain/message.go)):

```json
{
  "id": "<MessageID>",
  "type": "<MessageType>",
  "user_id": "<UserID>",
  "room_id": "<RoomID>",
  "timestamp": 1717488000000000000,
  "version": 1,
  "payload": { … },
  "metadata": {
    "trace_id": "…",
    "request_id": "…",
    "correlation_id": "…",
    "compression": "none|lz4|zstd|brotli"
  }
}
```

### Client → Server types

| Type | Persisted | Description | Payload fields |
|------|-----------|-------------|----------------|
| `text` | ✓ | Plain text message | `{ "text": "string" }` |
| `file` | ✓ | File attachment reference | `{ "media_id": "string", "mime_type": "string", "file_name": "string" }` |
| `typing` | ✗ | User is typing indicator | `{}` |
| `read_receipt` | ✗ | User has read up to a message | `{ "last_read_id": "MessageID" }` |
| `ack` | ✗ | Client acknowledges delivery | `{ "message_id": "MessageID", "status": "delivered\|read" }` |
| `video_call` | ✗ | VoIP signalling envelope | `{ "signal_type": "offer\|answer\|ice", "sdp": "string" }` |
| `cursor` | ✗ | Collaborative cursor position | `{ "position": number }` |
| `webrtc_signal` | ✗ | Raw WebRTC signal | `{ "signal": any }` |
| `game` | ✗ | In-chat game event | `{ "event": any }` |

### Server → Client types

| Type | Description | Payload fields |
|------|-------------|----------------|
| `text` / `file` / … | Echo of persisted messages | same as above |
| `system` | Server-generated notification | `{ "text": "string" }` |
| `ack_sent` | Server confirms message written to DB | `{ "message_id": "…", "status": "sent", "timestamp": … }` |
| `ack_failed` | Delivery permanently failed | `{ "message_id": "…", "status": "failed", "timestamp": … }` |

### Delivery states (`message_delivery_status` table)

`pending` → `sent` → `delivered` → `read`

If delivery fails after 5 attempts the message moves to `message_dlq` and a `ack_failed` message is sent to the sender.

---

## SSE event format

Each SSE frame:

```
id: <sequence-number>
event: <channel>
data: <JSON-encoded BaseMessage>

```

**Channels**: `messages`, `presence`, `system`

### Reconnect / gap replay protocol

1. Client reconnects and sends `Last-Event-ID: 4200` header (or `?last_event_id=4200` query param).
2. Server calls `replayRepository.GetEventsAfter(4200, roomID)`.
3. All events with sequence > 4200 are replayed in order before new events resume.

The `sequence` field is a monotonic counter maintained by `EventSequencer` in `ConnectionManager`.

---

## Pagination convention

All list endpoints use **offset-based pagination** via query parameters:

| Param | Type | Default | Max |
|-------|------|---------|-----|
| `limit` | int | 50 | 1000 (messages) / 200 (search) / 500 (sync) |
| `offset` | int | 0 | — |

Responses return a top-level array under a named key (e.g. `"messages"`, `"results"`, `"events"`). There is no `total` field — clients detect end-of-list when the returned count is less than `limit`.

---

## Request / response schemas

### `User` object

```json
{
  "id": 42,
  "username": "alice",
  "firstname": "Alice",
  "lastname": "Smith",
  "email": "alice@example.com",
  "two_fa_enabled": false,
  "created_at": "2026-01-01T00:00:00Z",
  "status": "online"
}
```

### `Room` object

```json
{
  "id": "room-uuid",
  "type": "group",
  "owner_id": "user-uuid",
  "name": "Study Group A",
  "members": ["uid1", "uid2"],
  "settings": {},
  "created_at": 1717488000,
  "updated_at": 1717488000
}
```

### `BaseMessage` object

```json
{
  "id": "msg-uuid",
  "type": "text",
  "user_id": "uid1",
  "room_id": "room-uuid",
  "timestamp": 1717488000000000000,
  "version": 1,
  "payload": { "text": "Hello!" },
  "reply_to_id": null,
  "forwarded_from_id": null,
  "topic_id": null,
  "metadata": { "trace_id": "", "compression": "none" }
}
```

### `TokenResponse` object

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9…",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9…"
}
```

### `Profile` object

```json
{
  "user_id": "uid1",
  "avatar_media_id": "media-uuid"
}
```

### `SearchResult` object

```json
{
  "message_id": "msg-uuid",
  "room_id": "room-uuid",
  "user_id": "uid1",
  "content": "Hello world",
  "created_at": "2026-01-01T00:00:00Z"
}
```

---

## Call / WebRTC Signaling

### ICE server config

#### `GET /api/v1/chat/call/ice-servers` _(requires auth)_

Returns the STUN/TURN list so the frontend never hard-codes ICE server URLs.
Configured via `STUN_SERVERS`, `TURN_URL`, `TURN_USER`, `TURN_PASS` env vars.

**Response `200`**
```json
{
  "iceServers": [
    { "urls": ["stun:stun.l.google.com:19302"] },
    { "urls": ["turn:turn.example.com:3478"], "username": "user", "credential": "pass" }
  ]
}
```

---

### Call signaling WebSocket — `/call/ws`

Server: [`internal/call/adapters/ws_call_server.go`](../internal/call/adapters/ws_call_server.go)

A dedicated ephemeral relay for WebRTC signaling. The server is a dumb fan-out
pipe: it authenticates the peer, then broadcasts every inbound frame verbatim to
all other peers in the room (sender excluded). All WebRTC semantics
(SDP offer/answer, ICE trickle, presence state machine) live in the client
([`frontend/app/composabels/WebRTC.ts`](../frontend/app/composabels/WebRTC.ts)).

Presence is tracked in Redis so the count survives across backend instances.
Cross-instance fan-out uses Redis Pub/Sub channel `call:signal`.

**Upgrade**: standard `GET /call/ws` with `Upgrade: websocket`.

**Query params**

| Param | Required | Description |
|-------|----------|-------------|
| `room_id` | ✓ | Room to join. Caller must be a room member. |
| `token` | ✓ | JWT access token (same as `/ws`). |
| `type` | ✓ | `call` for media peers; any other value for plain signaling. |

**Bootstrap (call peers only)**

Immediately after the upgrade a `call` peer must send:
```json
{ "type": "checkCountUserCall" }
```
The server replies before registering the peer:
```json
{ "type": "checkCountUserCall", "count": 2 }
```
`count` is the number of call-peers already in the room, **excluding** the
connecting peer. The client uses this to decide whether to `startCall()` (0
existing peers) or `connectionCall()` (≥ 1 existing peer).

**Subsequent frames — Client → Server**

Any valid JSON frame. The server relays it to all other peers in the room unchanged.
Key envelope types (defined in [`frontend/types/signaling.ts`](../frontend/types/signaling.ts)):

| `type` | Purpose |
|--------|---------|
| `StartStream` | Announce a call/stream has started |
| `Status` | Presence update + SDP offer carrier (`statusUser: Active\|Expectation\|Close`) |
| `Answer` | SDP answer / action acknowledgement |
| `checkUserActive` | Peer discovery handshake |
| `iceCandidate` | Trickle ICE candidate |

**Server → Client**

Relayed frames from other peers, unchanged. The only server-generated frame is
the `checkCountUserCall` bootstrap reply above.

**Auth errors**

Missing or invalid `token` → HTTP `401` before the upgrade (no WS frame is sent).
Non-member `room_id` → HTTP `401`.

---

## Rate limiting

Rate limiting is not yet implemented at the HTTP layer. The spam protector (`internal/moderation/spam.go`) enforces a **20 messages per minute** per-user limit at the WebSocket level — excess messages are silently dropped without acknowledgement.
