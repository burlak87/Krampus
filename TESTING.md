# TESTING.md — Verifying the Krampus call integration

This covers the automated suites and the exact steps to run a real WebRTC call
between two users — **entirely inside Docker containers**, tested from two
browsers. It is the runnable companion to [PLAN.md](PLAN.md) §7.

---

## 1. Automated tests (no containers needed)

```bash
go build ./...                   # whole backend compiles
go test ./...                    # all suites   (or: make test)
go test ./internal/call/... -v   # the call module specifically
```

What's covered today (`internal/call/`):

| Suite | File | Asserts |
|-------|------|---------|
| Hub unit | `service/signaling_test.go` | broadcast excludes sender, room isolation, `CALL_MAX_PEERS` cap (incl. non-call peers + slot reuse), cross-node delivery, count delegation, bootstrap reply JSON |
| WS integration | `adapters/ws_call_server_test.go` | 401 on bad/missing token, 400 on missing `room_id`, `checkCountUserCall` bootstrap (0 → 1), verbatim frame relay (sender excluded), presence cleared on disconnect |
| ICE endpoint | `adapters/ice_handler_test.go` | STUN list, TURN+credentials, blanks dropped, empty `[]` not `null` |

These use in-memory fakes (auth + presence) — **no Postgres/Redis/Kafka**.

---

## 2. The full Docker stack

Everything — backend, frontend, and all infrastructure — runs as containers
from a single `docker-compose.yml`.

### 2.1 Prerequisites

- Docker + Docker Compose v2.
- A `.env` file at the repo root (`cp .env.example .env`). The `app` service
  loads it via `env_file`. The default `JWT_SECRET` is fine for local testing.

> **Known blocker on this host:** the local Docker runtime currently fails to
> start *any* container with `failed to start shim … unsupported protocol:
> Yunix` (a host containerd/runc problem, not a project issue). Until that's
> repaired, `docker compose up` cannot run here. The compose stack below is
> correct and validated (`docker compose config` passes); it will work on a host
> with a healthy Docker runtime. A native (no-Docker) fallback is in §5.

### 2.2 Bring it all up

```bash
docker compose up -d         # or: make up
docker compose ps            # watch services become healthy
docker compose logs -f app frontend   # follow backend + frontend logs
```

This starts **8 services** on one bridge network (`krampus-net`):

| Service | Container | Host port | Role |
|---------|-----------|-----------|------|
| `app` | krampus_app | **8080** | Go backend (REST + `/ws` + `/call/ws`) |
| `frontend` | krampus_frontend | **3000** | Nuxt 3 dev server (`npm run dev`) |
| `postgres` | krampus_postgres | 5432 | primary store (seeded from `sql/schema/init.sql`) |
| `redis` | krampus_redis | 6379 | sessions + **call presence** |
| `kafka` / `zookeeper` | krampus_kafka | 9092 | message transport |
| `minio` (+`minio-init`) | krampus_minio | 9000/9001 | object storage |

Key wiring already in compose:
- `app` reaches infra by **service name** (`postgres:5432`, `redis:6379`,
  `kafka:29092`, `minio:9000`) and carries the call vars
  (`CALL_PRESENCE_TTL`, `CALL_MAX_PEERS`, `STUN_SERVERS`, `TURN_*`).
- `frontend` sets `NUXT_PUBLIC_API_BASE=http://localhost:8080/api/v1` and
  `NUXT_PUBLIC_WS_BASE=ws://localhost:8080`. These point at **localhost**, not
  `app`, because they are used by the **browser** on your host (which reaches
  the backend through the published `8080` port), not from inside the network.
- The backend's CORS + WebSocket origin whitelist already allows
  `http://localhost:3000` (the Nuxt origin), so the containerized frontend can
  call the API and open both WebSockets.

> First `frontend` boot runs `npm install` inside the container (cached in the
> `frontend_node_modules` volume), so it may take a minute before
> `http://localhost:3000` responds. Watch `docker compose logs -f frontend`.

### 2.3 Smoke-test the API (against the containerized backend)

Once `app` is healthy:

```bash
./scripts/smoke_test.sh          # or: make smoke
```

It registers two users, signs them in, creates a shared room **with both as
members**, and exercises `GET /chat/users`, `GET /chat/rooms`,
`POST /chat/rooms/join`, `GET /chat/call/ice-servers`, message send/fetch, and
the 401 auth gate. On success it prints a ready-to-use **room id + both
logins** for the browser test. Requires `curl` and `jq` on the host.

(If you prefer to run it from inside the network instead of the host, the same
script works with `BASE=http://app:8080` from a container on `krampus-net`.)

---

## 3. Manual end-to-end call — two browsers

With the stack up (§2.2) and ideally the smoke test passed (§2.3):

1. Open **`http://localhost:3000` in two separate browser profiles** — e.g. a
   normal window and an incognito/private window, or two different browsers.
   Each profile needs its **own session/localStorage**, so two tabs in the same
   profile will **not** work (they'd share one login).

2. **Sign in as two different users** (one per profile):
   - Reuse the pair the smoke test created (printed at the end of its run), or
   - Register two new users on the Register page.
   - If a user has 2FA enabled, the login form reveals a code field
     (`/auth/verify`).

3. **Both users must be members of the same room** — that's what `/call/ws`
   authorizes against:
   - Easiest: use the smoke-test room (both users already members), **or**
   - User A creates a group (sidebar → create), then User B joins it via the
     join flow with the room id or a `krampus://join/<room_id>` deep link.

4. **Place the call:**
   - User A opens the shared room and **starts the call** (`startCall()` — the
     call/voice button).
   - User B opens the same room and **joins** (`connectionCall()`).

5. **Verify (in both browser dev-consoles):**
   - Signaling frames flow both ways: `checkCountUserCall` → `StartStream` →
     `Status` / `Answer` / `iceCandidate`.
   - `RTCPeerConnection.connectionState` reaches `connected`.
   - Audio is audible; toggling mic / video / desktop-share renegotiates
     (`createOfferRegenerate`).
   - `closeCall()` tears both sides down; the `call:room:*` / `call:peer:*` keys
     disappear from Redis (`docker compose exec redis redis-cli keys 'call:*'`).

## 3a. Full feature checklist (what to test, in order)

Tick each box. "Wired in UI" = exercisable from the browser; "API only" = no UI
button yet, verify via `scripts/smoke_test.sh` or curl.

### Authentication
- [ ] **Register** a new user (Register page) → redirected in, session created.
- [ ] **Login** with email+password (Login page) → lands in the app.
- [ ] **Wrong password** → error shown, not logged in.
- [ ] **2FA login** (only if the user enabled 2FA): login reveals a code field →
      `/auth/verify` → success. *(2FA enable endpoint exists but has no UI button.)*
- [ ] **Session persists across reload**: refresh the page → still logged in
      (tokens in localStorage, `/auth/refresh` keeps the session).
- [ ] **Logout** clears the session. *(API `/auth/logout` exists; wire/трigger if
      a button is present, otherwise API-only.)*

### Sidebar / rooms / users
- [ ] **Room list** (left sidebar) loads the rooms you're a member of
      (`GET /chat/rooms`).
- [ ] **User list** loads real users (`GET /chat/users`), not mock data.
- [ ] **Create group** (sidebar → create) → new room appears; creator is a member.
- [ ] **Create chat** (text/voice) → room created.
- [ ] **Open a room** → its message history loads.
- [ ] **Join a room** by id / `krampus://join/<room_id>` deep link → membership
      granted. *(API-only: `POST /chat/rooms/join` — UI button may be absent.)*

### Text messaging  *(REST-based; see the realtime caveat below)*
- [ ] **Send a text message** in an open room → it persists
      (`POST /chat/messages`).
- [ ] **History loads** when (re)opening the room (`GET /chat/rooms/:id/messages`).
- [ ] **Cross-user delivery:** a message sent by user A becomes visible to user B
      **after B reopens/refreshes the room**. ⚠️ The frontend fetches messages
      over REST only — there is **no live WebSocket push for text yet**, so new
      messages do not appear in real time; reopening the room is expected.

### Voice / video call  *(all wired in `Voice.vue`)*
- [ ] **Start call** (user A) / **Join call** (user B) → `connected`.
- [ ] **Mic off / on** (`micsOffAudio` / `micsOnAudio`) — other side hears it stop/start.
- [ ] **Camera off / on** (`micsOffVideo` / `micsOnVideo`).
- [ ] **Screen share on / off** (`desktopTranslateOn` / `desktopTranslateOff`).
- [ ] **Mute** toggle (`muthAudioOn` / `muthAudioOff`).
- [ ] **End call** (`closeCall`) → both sides tear down; Redis `call:*` keys clear.
- [ ] **Room-full guard:** with `CALL_MAX_PEERS` reached, the next caller is closed
      with WS code 1013.

### Backend-only features (no dedicated UI — verify via curl / smoke test)
- [ ] **ICE servers** `GET /api/v1/chat/call/ice-servers` returns `iceServers[]`.
- [ ] **Message search** `GET /chat/rooms/:id/search?q=` (search UI not wired).
- [ ] **Reactions** `POST /chat/messages/:id/reactions`.
- [ ] **Stickers** `GET /chat/stickers`.
- [ ] **Sync** `GET /chat/sync?device_id=`.
- [ ] **Get single user** `GET /chat/users/:id`.

### Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| `http://localhost:3000` not loading | `frontend` still running `npm install` — check its logs |
| WS closes immediately with 401 | Not signed in / token missing / user not a room member |
| WS close code **1013** "room full" | `CALL_MAX_PEERS` reached for that room |
| Signaling OK but no remote audio | STUN/TURN unreachable — check `GET /chat/call/ice-servers`; cross-NAT needs a TURN server (`TURN_URL/USER/PASS`) |
| `count` always 0 | `app` can't reach Redis — check `docker compose logs app` |
| CORS error in console | Frontend not on `http://localhost:3000`, or backend origin whitelist changed |

---

## 4. Useful container commands

```bash
docker compose ps                          # status / health
docker compose logs -f app                 # backend logs
docker compose exec redis redis-cli keys 'call:*'   # live presence keys
docker compose exec postgres psql -U krampus -d krampus -c '\dt'   # tables
docker compose down                        # stop (keep data)   | make down
docker compose down -v                     # stop + wipe volumes | make db-reset re-creates
```

---

## 5. Native fallback (no Docker)

If the Docker runtime is unavailable, run the pieces directly against locally
installed services and point the backend at them via env vars in
`pkg/config/config.go` (`POSTGRES_DSN`, `REDIS_ADDR`, `KAFKA_BROKERS`; seed
Postgres with `sql/schema/init.sql`):

```bash
make run          # backend on :8080  (reads .env)
make smoke        # API contract checks
make frontend     # cd frontend && npm install && npm run dev  → :3000
```

Then follow §3 from step 1.

---

## 6. Acceptance checklist (PLAN §7.6)

- [x] Frontend no longer references `localhost:8080/ws/...` or the `#dataUser` mock.
- [x] `/call/ws` rejects unauthenticated connections (401) — covered by tests.
- [x] Call signaling unit + integration suites green.
- [x] `docs/API.md` documents the call endpoints and the updated room/user APIs.
- [ ] Two authenticated peers establish a working WebRTC audio call (manual, §3).
- [ ] §4.1 endpoints return documented shapes against a live backend (`make smoke`).
