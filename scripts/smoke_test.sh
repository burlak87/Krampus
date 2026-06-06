#!/usr/bin/env bash
#
# smoke_test.sh — exercises the documented Krampus REST contracts end-to-end
# against a running backend (see docs/API.md). It registers two users, creates a
# shared room, validates membership, and hits the call/chat endpoints the
# frontend depends on. Use it to confirm the backend is healthy before manual
# browser testing.
#
# Prereqs: backend running on $BASE, plus `curl` and `jq`.
# Usage:   ./scripts/smoke_test.sh            (defaults to localhost:8080)
#          BASE=http://host:8080 ./scripts/smoke_test.sh
#
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
API="$BASE/api/v1"
PASS="Sup3rs3cret!"
STAMP="$(date +%s)"
A_EMAIL="alice_$STAMP@example.com"
B_EMAIL="bob_$STAMP@example.com"

command -v jq >/dev/null || { echo "jq is required"; exit 1; }

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1"; echo "    $2"; exit 1; }

echo "==> Health"
curl -sf "$BASE/health" >/dev/null && pass "GET /health" || fail "health" "is the backend up on $BASE?"

echo "==> Signup (2 users)"
curl -sf -X POST "$API/auth/signup" -H 'Content-Type: application/json' \
  -d "{\"username\":\"alice_$STAMP\",\"firstname\":\"Alice\",\"lastname\":\"A\",\"email\":\"$A_EMAIL\",\"password\":\"$PASS\"}" >/dev/null \
  && pass "alice signup" || fail "alice signup" "check /auth/signup"
curl -sf -X POST "$API/auth/signup" -H 'Content-Type: application/json' \
  -d "{\"username\":\"bob_$STAMP\",\"firstname\":\"Bob\",\"lastname\":\"B\",\"email\":\"$B_EMAIL\",\"password\":\"$PASS\"}" >/dev/null \
  && pass "bob signup" || fail "bob signup" "check /auth/signup"

echo "==> Signin"
A_TOK="$(curl -sf -X POST "$API/auth/signin" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$A_EMAIL\",\"password\":\"$PASS\"}" | jq -r '.access_token // empty')"
[ -n "$A_TOK" ] || fail "alice signin" "no access_token (2FA enabled? token shape changed?)"
pass "alice signin → access_token"
B_TOK="$(curl -sf -X POST "$API/auth/signin" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$B_EMAIL\",\"password\":\"$PASS\"}" | jq -r '.access_token // empty')"
[ -n "$B_TOK" ] || fail "bob signin" "no access_token"
pass "bob signin → access_token"

# user_id is the JWT 'user_id' claim (base64url payload, middle segment).
# base64url → base64, then re-pad to a multiple of 4 before decoding.
jwt_uid() {
  local p; p="$(echo "$1" | cut -d. -f2 | tr '_-' '/+')"
  while [ $(( ${#p} % 4 )) -ne 0 ]; do p="${p}="; done
  echo "$p" | base64 -d 2>/dev/null | jq -r '.user_id'
}
A_UID="$(jwt_uid "$A_TOK")"; B_UID="$(jwt_uid "$B_TOK")"
pass "alice uid=$A_UID  bob uid=$B_UID"

echo "==> List users (GET /chat/users)"
N="$(curl -sf "$API/chat/users?limit=200" -H "Authorization: Bearer $A_TOK" | jq 'length')"
[ "$N" -ge 2 ] && pass "listed $N users" || fail "list users" "expected >=2, got $N"

echo "==> Create room with both members (POST /chat/rooms)"
ROOM_ID="room-$STAMP"
ROOM="$(curl -sf -X POST "$API/chat/rooms" -H "Authorization: Bearer $A_TOK" -H 'Content-Type: application/json' \
  -d "{\"id\":\"$ROOM_ID\",\"type\":\"group\",\"name\":\"Smoke $STAMP\",\"members\":[\"$B_UID\"]}")"
echo "$ROOM" | jq -e --arg id "$ROOM_ID" '.id == $id' >/dev/null \
  && pass "room created id=$ROOM_ID" || fail "create room" "$ROOM"
echo "$ROOM" | jq -e --arg b "$B_UID" '.members | index($b)' >/dev/null \
  && pass "bob is a member" || fail "membership" "bob not in members: $(echo "$ROOM" | jq -c .members)"

echo "==> Bob lists his rooms (GET /chat/rooms)"
curl -sf "$API/chat/rooms" -H "Authorization: Bearer $B_TOK" | jq -e --arg id "$ROOM_ID" 'any(.id == $id)' >/dev/null \
  && pass "bob sees the shared room" || fail "list rooms" "shared room not visible to bob"

echo "==> Join flow (POST /chat/rooms/join)"
curl -sf -X POST "$API/chat/rooms/join" -H "Authorization: Bearer $B_TOK" -H 'Content-Type: application/json' \
  -d "{\"token\":\"krampus://join/$ROOM_ID\"}" | jq -e '.status == "joined"' >/dev/null \
  && pass "join returns joined" || fail "join" "join did not succeed"

echo "==> ICE servers (GET /chat/call/ice-servers)"
curl -sf "$API/chat/call/ice-servers" -H "Authorization: Bearer $A_TOK" | jq -e '.iceServers | type == "array"' >/dev/null \
  && pass "ice-servers returns iceServers[]" || fail "ice-servers" "bad shape"

echo "==> Send + fetch message"
curl -sf -X POST "$API/chat/messages" -H "Authorization: Bearer $A_TOK" -H 'Content-Type: application/json' \
  -d "{\"room_id\":\"$ROOM_ID\",\"type\":\"text\",\"payload\":{\"text\":\"hello from smoke test\"}}" >/dev/null \
  && pass "message sent" || fail "send message" "POST /chat/messages failed"
curl -sf "$API/chat/rooms/$ROOM_ID/messages?limit=10" -H "Authorization: Bearer $A_TOK" | jq -e 'type == "array"' >/dev/null \
  && pass "messages fetched" || fail "get messages" "GET messages failed"

echo "==> Auth negative: no token must be 401"
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$API/chat/users")"
[ "$CODE" = "401" ] && pass "unauthenticated → 401" || fail "auth gate" "expected 401, got $CODE"

echo
echo "ALL SMOKE CHECKS PASSED ✅"
echo "Shared room for manual browser testing: $ROOM_ID"
echo "  alice: $A_EMAIL / $PASS   (uid $A_UID)"
echo "  bob:   $B_EMAIL / $PASS   (uid $B_UID)"
