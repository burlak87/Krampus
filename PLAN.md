# PLAN.md — Migrating KrampusMessage Call/Signaling into the Krampus Backend

## 0. Context & layout

The KrampusMessage repo (`git@github.com:Hahog/KrampusMessage.git`) is now cloned
into **`frontend/`** at the root of this repo. It contains:

- `frontend/app/` — Nuxt 3 / Vue 3 / TS / Pinia client (currently on **mocked data**)
- `frontend/server/api/main.go` — the **standalone Go WebRTC signaling server** (161 lines) we are migrating
- `frontend/types/`, `frontend/schemas/`, `frontend/composabels/` — client-side signaling protocol

The Krampus backend is this repo. Its **authoritative API documentation is
[`docs/API.md`](docs/API.md)** — every integration contract below is taken from it.

| Repo / dir | Role | Stack |
|------------|------|-------|
| Krampus (root) | Production backend | Go, Gin, sqlc/Postgres, Redis, Kafka, layered `internal/<module>` |
| `frontend/` (KrampusMessage) | PoC client + PoC signaling server | Nuxt 3 client + 161-line Go signaling relay |

**Goal:** copy the call/WS handling out of `frontend/server/api/main.go` into a
new `internal/call` module of the Krampus backend, then — using `docs/API.md` —
wire the Nuxt front-end to the real backend (auth, chat, signaling) and make the
whole thing testable end-to-end.

---

## 1. The code being migrated (`frontend/server/api/main.go`)

A **stateless room relay** (Gin + gorilla/websocket):

- One route: `GET /ws/:idRoom/:typeWS` (`typeWS` = `"call"` or chat). **No auth, `CheckOrigin:true`.**
- In-memory `rooms map[string]map[*Client]bool`.
- On `typeWS=="call"` connect: reads one bootstrap frame; if `{"type":"checkCountUserCall"}`,
  replies with the count of call-peers already in the room (so the client decides *start* vs *join*).
- `readPump`: every inbound frame is **broadcast verbatim** to every other client in the room (sender excluded).
- `writePump`: 50s ping ticker, 10s write deadline.
- All WebRTC semantics live **client-side** (`frontend/composabels/Signaling.ts`,
  `WebRTC.ts`, `frontend/types/signaling.ts`).

### Signaling envelopes relayed through a room (server is a dumb pipe)

| `type` | Purpose | Key fields |
|--------|---------|-----------|
| `checkCountUserCall` | bootstrap: how many call peers? | — |
| `StartStream` | announce call/stream started | `idRoom`, `maxUser`, `system_option{isAudio,isVideo,adminRoom,timeStartStream}` |
| `Status` | presence + SDP offer carrier | `statusUser:Active|Expectation|Close`, `name`, `idUserTarget`, `idRoom`, `offer?`, `system_option` |
| `Answer` | SDP answer / action ack | `idUserAnswer`, `idUserTarget`, `action`, `status`, `answer?` |
| `checkUserActive` | discover a peer (handshake init) | `idUserTarget`, `idRoom`, `preliminary?` |
| `iceCandidate` | trickle ICE | `idUserTarget`, `idUserAnswer`, `iceCandidate` |

**Migration-critical fact:** the relay is pure fan-out, so we can reproduce it
exactly while adding auth + presence. The client protocol need not change for a
first working integration — only the connection URL + auth do.

---

## 2. Key discovery from `docs/API.md` — the WS already has a signaling channel

`docs/API.md` §"WebSocket message type catalogue" documents that the existing
`/ws` endpoint already accepts **realtime-only (non-persisted)** types:

| Type | Persisted | Payload |
|------|-----------|---------|
| `video_call` | ✗ | `{ "signal_type": "offer|answer|ice", "sdp": "string" }` |
| `webrtc_signal` | ✗ | `{ "signal": any }` |

Types flagged `IsRealtimeOnly()` are **broadcast without being persisted to Postgres** —
exactly the behaviour the PoC relay provides. This gives us **two viable designs**;
the plan picks B and notes A as the fast fallback:

- **Design A (reuse `/ws`):** tunnel every PoC envelope inside a `webrtc_signal`
  `BaseMessage.payload.signal` and let the existing `ConnectionManager` broadcast
  it. Zero new endpoint. Downside: signaling rides the durable/sequenced delivery
  path (ack/retry/DLQ machinery, head-of-line latency) which is wrong for ICE.
- **Design B (dedicated `internal/call` WS, chosen):** a separate ephemeral
  endpoint that mirrors the PoC pump model but adds Krampus auth + Redis presence.
  Keeps signaling off the durable pipeline. **This is the migration target.**

---

## 3. Target design in Krampus (`internal/call`)

Follows the repo's layered convention (see `docs/Architecture.md`):

```
internal/call/
  domain/
    signal.go        # envelope types from §1 (json tags matching the FE client)
    room.go          # CallRoom, CallParticipant + interfaces
  service/
    signaling.go     # relay/broadcast logic, room lifecycle (ported from main.go)
    presence.go      # call-peer membership + callCount (ported from PoC callCount)
  storage/
    redis_presence.go# membership in Redis (TTL'd, multi-instance safe)
  adapters/
    ws_call_server.go# HandleCallWebSocket — copied/adapted from frontend/server/api/main.go
    client.go        # per-connection read/write pumps (copied from main.go pumps)
    ice_handler.go   # GET /api/v1/chat/call/ice-servers (STUN/TURN list)
```

### 3.1 What to copy vs change when porting `main.go`

| PoC (`frontend/server/api/main.go`) | Krampus `internal/call` |
|-------------------------------------|-------------------------|
| `Client` struct, `addClient`/`removeClient`/`broadcast`/`callCount` | copied near-verbatim into `service/signaling.go` |
| `readPump`/`writePump` | copied into `adapters/client.go` |
| `wsHandler` route `/ws/:idRoom/:typeWS` | becomes `HandleCallWebSocket` on `GET /call/ws?room_id&type&token` |
| `upgrader{CheckOrigin: true}` | origin whitelist (reuse the `http://localhost:3000` list from `message/adapters`) |
| no auth | `WSAuthService.Authenticate` (JWT+session) — same as `/ws`, per `docs/API.md` |
| in-memory `rooms` map | Phase 1: in-process map (identical). Phase 2: Redis Pub/Sub `call:room:<id>` for multi-instance |
| `checkCountUserCall` from map | same, counted from Redis presence (exclude self) |

### 3.2 Endpoint & auth (consistent with `docs/API.md`)

`docs/API.md` specifies WS auth via `?token=<jwt>` query param (or
`Authorization` header) + `room_id` query param, validated by `WSAuthService`.
The call endpoint follows the same contract:

```
GET /call/ws?room_id=<id>&type=call&token=<jwt>
```

Register in `cmd/app/main.go` beside the existing `/ws` and `/sse`:
```go
engine.GET("/call/ws", func(c *gin.Context) {
    callServer.HandleCallWebSocket(c.Writer, c.Request)
})
```

### 3.3 Relay semantics to preserve
1. Connect → authenticate → upgrade → register peer in presence for `room_id` (`is_call = type=="call"`).
2. `checkCountUserCall` bootstrap → reply with live call-peer count (exclude self).
3. Inbound frame → broadcast to all other peers in the room (Phase-1 in-process; Phase-2 Redis Pub/Sub).
4. Disconnect → remove from presence; relay the client's `Status:Close` so peers tear down `RTCPeerConnection`s.

### 3.4 Config (`pkg/config/config.go`, env-driven, per `docs/Configuration.md`)

| Var | Default | Meaning |
|-----|---------|---------|
| `CALL_WS_PATH` | `/call/ws` | call signaling endpoint |
| `CALL_PRESENCE_TTL` | `30s` | Redis presence TTL (refreshed on ping) |
| `CALL_MAX_PEERS` | `8` | per-room cap (PoC `maxUser`) |
| `STUN_SERVERS` | `stun:stun.l.google.com:19302` | surfaced to client via ICE endpoint |
| `TURN_URL`/`TURN_USER`/`TURN_PASS` | empty | optional TURN |

---

## 4. Connecting front-end ↔ back-end (using `docs/API.md`)

The Nuxt client currently uses mocked data (`composabels/Auth.ts#dataUser`, empty
`fetch("")`; `composabels/messageManagment.ts` returns mock arrays). Connecting =
replacing the mocks with the documented endpoints. Base URL `http://<host>:8080`;
`/api/v1/chat/*` requires `Authorization: Bearer <token>`.

### 4.1 Documented endpoints the client must call (from `docs/API.md`)

**Auth — `/api/v1/auth` (open):**
| Method | Path | Request → Response | FE site to de-mock |
|--------|------|--------------------|--------------------|
| POST | `/auth/signup` | `{username,email,password}` → `201 User` | `frontend/app/pages/register.vue` |
| POST | `/auth/signin` | `{email,password}` → `{access_token,refresh_token}` **or** `{temp_token,requires_two_fa}` | `frontend/app/composabels/Auth.ts#startAuth` |
| POST | `/auth/verify` | `{temp_token,code}` → `TokenResponse` | 2FA step (new in FE) |
| POST | `/auth/refresh` | `{refresh_token}` → `TokenResponse` | session refresh / `middleware/auth.ts` |
| POST | `/auth/logout` | `{password}` → `{success:true}` | logout |

**Chat — `/api/v1/chat` (Bearer auth):**
| Method | Path | Request/Query → Response | FE site to de-mock |
|--------|------|--------------------------|--------------------|
| POST | `/chat/messages` | `{room_id,type:"text",payload:{text}}` → saved `BaseMessage` | `messageManagment.ts#createNewMessage` |
| GET | `/chat/rooms/:room_id/messages?limit=` | → `BaseMessage[]` | `requestAllMessageGroupChat`/`getUserChatMessage` (mock arrays) |
| GET | `/chat/rooms/:room_id/search?q=&limit=` | → `SearchResult[]` | search UI |
| POST | `/chat/rooms` | `{type,name,members[]}` → `201 Room` | `modal/createGroup.vue`, `modal/createChat.vue` |
| GET | `/chat/rooms/:room_id` | → `Room` | room open |
| POST | `/chat/rooms/join` | `{token}` → `{status:"joined"}` | invite join |
| GET | `/chat/users/:user_id` | → `User` | `composabels/User.ts`, sidebar |
| POST | `/chat/messages/:message_id/reactions` | `{emoji}` → `{status:"ok"}` | reactions |
| GET | `/chat/stickers` | → packs | sticker picker |
| GET | `/chat/sync?device_id=` | → `{last_event_sequence}` | reconnect/gap detection |
| POST | `/chat/profile/avatar` / GET `/chat/profile/:user_id` | avatar | profile |

**Realtime text — `/ws`** (`docs/API.md` §WebSocket): `GET /ws?room_id=<id>&token=<jwt>`,
`BaseMessage` envelope, `type:"text"` persisted; `typing`/`read_receipt`/`ack` realtime-only.
**Realtime call — `/call/ws`** (new, §3.2): the migrated signaling relay.
(Optional Design A: send `type:"video_call"`/`"webrtc_signal"` over `/ws` instead.)

**Errors** follow the `AppError` schema `{code,message}` with the documented code
table (`UNAUTHORIZED`→401, `VALIDATION_ERROR`→422, etc.) — FE error handling
should branch on `code`, not HTTP status alone.

### 4.2 Front-end changes (in `frontend/`)

1. **`nuxt.config.ts`** — add `runtimeConfig.public.apiBase` (`http://localhost:8080/api/v1`)
   and `wsBase` (`ws://localhost:8080`); remove hard-coded URLs.
2. **`composabels/Auth.ts`** — replace `#dataUser` + empty `fetch("")` with real
   `POST /auth/signin`/`signup`; handle the **2FA branch** (`requires_two_fa` →
   `/auth/verify`); persist `access_token`/`refresh_token` in `useUserStore`
   (needed for WS `?token=`); keep the existing client-side validation.
   Honour the 15-min access / 7-day refresh lifetimes via `/auth/refresh`.
3. **`composabels/messageManagment.ts`** — replace mock arrays with
   `GET /chat/rooms/:room_id/messages` + `POST /chat/messages` (map FE shape
   `{name,data,srcImg,time}` ↔ documented `BaseMessage`).
4. **`composabels/WebRTC.ts`** — change socket URL from
   `ws://localhost:8080/ws/${idRoom}/${typeWS}` to
   `${wsBase}/call/ws?room_id=${idRoom}&type=${typeWS}&token=${jwt}`; fix the
   reconnect-URL bug (§6). Envelopes unchanged.
5. **`middleware/auth.ts`** — gate on a real session (`/auth/refresh` success)
   instead of the mock.
6. Thread ICE config from the new endpoint into `callStore.newUserPC` PCs.

### 4.3 New REST endpoint: ICE config
`GET /api/v1/chat/call/ice-servers` (Bearer auth) → `{ "iceServers":[…] }` from
the STUN/TURN config (§3.4). Implemented in `internal/call/adapters/ice_handler.go`,
registered under the existing auth'd `chatGroup` in `cmd/app/main.go`.

### 4.4 CORS / origin
Reuse the `message/adapters` upgrader whitelist (`http://localhost:3000`, the Nuxt
dev port) for the call upgrader; confirm `apperror.CORSMiddleware()` allows the
Nuxt origin for REST.

---

## 5. Implementation steps (ordered)

1. **Scaffold** `internal/call/{domain,service,storage,adapters}`; port the
   envelope types (§1) into `domain/signal.go`.
2. **Copy the relay** out of `frontend/server/api/main.go`: `Client`/`broadcast`/
   `callCount` → `service/signaling.go`; pumps → `adapters/client.go`; `wsHandler`
   → `adapters/ws_call_server.go` (now auth'd via `WSAuthService`, origin-whitelisted).
3. **`checkCountUserCall`** count from presence (exclude self).
4. **Wire `cmd/app/main.go`**: build `callServer` (config, Redis, `WSAuthService`),
   register `GET /call/ws` and `GET /api/v1/chat/call/ice-servers`.
5. **Config**: add §3.4 vars to `pkg/config/config.go`, `.env.example`, compose files.
6. **Phase-2 fan-out**: Redis Pub/Sub bridge for multi-instance signaling.
7. **Front-end wiring** (§4.2): de-mock Auth (incl. 2FA) + messages, repoint WS URL, inject ICE config.
8. **Docs**: add a "Calls / signaling" section to `docs/API.md` documenting
   `/call/ws` and `/chat/call/ice-servers` so the contract stays authoritative.
9. **Observability**: structured logs (`pkg/logging`) for connect/disconnect/relay
   counts; mark `frontend/server/api/` deprecated (Krampus is now source of truth).

---

## 6. Known PoC bugs to fix during migration (don't port blindly)

1. `WebRTC.ts` reconnect loop targets `ws://localhost:8080/${idRoom}/${typeWS}`
   (missing `/ws`) and never rebinds `onmessage` — repoint to `/call/ws?...` and re-attach handlers.
2. `type` string mismatch: client emits/checks `"StartStream"` vs `"StartSream"`
   and `"Status"` — normalize to one constant set shared by FE types and the Go `domain` tags.
3. `Auth.ts` / `messageManagment.ts` ship hard-coded mocks + empty `fetch("")` — replaced per §4.2.
4. PoC has no auth and `CheckOrigin:true` — replaced by `WSAuthService` + origin whitelist.
5. PoC presence is in-process only — replaced by Redis (§3.3) for multi-instance.

---

## 7. Testing plan (get everything ready for testing)

### 7.1 Prerequisites
- `docker-compose up -d` (Postgres, Redis, Kafka — Redis required for presence).
- `sqlc generate` only if schema changes (call module needs none initially).
- Backend: `make build` / `go run ./cmd/app` → `:8080`.
- Front-end: `cd frontend && npm install && npm run dev` → `:3000`.
- Seed: register two users via `POST /api/v1/auth/signup` (see `docs/API.md`).

### 7.2 Go unit tests (repo currently has none — establish the first suite)
- `service/signaling_test.go`: broadcast hits all-but-sender; `callCount` excludes self; max-peer cap.
- `domain/signal_test.go`: JSON round-trip of every envelope incl. the client's exact field names (golden fixtures from the FE).
- `storage/redis_presence_test.go`: TTL expiry, room isolation (miniredis).

### 7.3 Integration tests (`httptest` + gorilla/websocket clients)
- **A — count bootstrap:** A connects (`type=call`) + `checkCountUserCall` → `count:0`; B → `count:1`.
- **B — relay:** A sends `Status:Active` with `offer` → B gets identical frame, A does not.
- **C — auth (per `docs/API.md`):** no `token` → 401 `UNAUTHORIZED`; bad token → 401; valid → upgrade OK.
- **D — disconnect:** A drops → presence decrements; B receives relayed `Status:Close`.
- **E — multi-instance (Phase 2):** A on instance 1, B on instance 2 (shared Redis) → relay crosses instances.

### 7.4 API contract tests (front-end ↔ documented backend)
- Assert each call in §4.1 against a running backend: status codes + `AppError`
  `{code}` on failure paths (e.g. expired JWT → `UNAUTHORIZED`, dup signup → `DUPLICATE_ERROR`).
- 2FA path: `signin` → `requires_two_fa` → `verify` → `TokenResponse`.

### 7.5 Manual / E2E (two browsers)
1. Register + log in two users via the real auth API (handle 2FA if enabled).
2. Open the same room in two profiles; user 1 `startCall()`, user 2 `connectionCall()`.
3. **Verify:** signaling frames in both consoles; `RTCPeerConnection.connectionState`
   reaches `connected`; audio flows; mic/video/desktop toggles renegotiate via
   `createOfferRegenerate`; `closeCall()` tears down both sides.
4. Cross-NAT test for STUN, then TURN.

### 7.6 Acceptance criteria
- [ ] FE no longer references `localhost:8080/ws/...` or mocked `#dataUser`.
- [ ] `/call/ws` rejects unauthenticated connections (401 `UNAUTHORIZED`).
- [ ] Two authenticated peers establish a working WebRTC audio call via Krampus-hosted signaling.
- [ ] All §4.1 endpoints return the documented shapes; errors use the `AppError` code table.
- [ ] Go unit + integration suites green; `docs/API.md` updated with the call endpoints.

---

## 8. Open decisions
- **Design A vs B:** plan implements B (dedicated `/call/ws`) for latency; A
  (`video_call`/`webrtc_signal` over `/ws`) is the documented fallback if a
  separate endpoint is undesirable.
- **Mesh vs SFU:** FE is full-mesh P2P (the commented `PeerConectionMCU` hints at
  a future SFU). Mesh is fine at `CALL_MAX_PEERS=8`; SFU is the documented scaling path, out of scope here.
- **Room authorization:** require chat-room membership (via `chat`/`permissions`)
  before upgrade, or just a valid session? Recommend membership check.
- **TURN hosting:** needed for real cross-NAT calls — decide coturn deployment
  (`docker-compose`/Ansible); STUN-only works on the same LAN for demos.
