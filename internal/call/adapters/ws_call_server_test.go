package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"krampus/internal/call/domain"
	"krampus/internal/call/service"
	identityDomain "krampus/internal/identity/domain"
	"krampus/pkg/logging"
	"krampus/pkg/types"

	"github.com/gorilla/websocket"
)

// TestMain initializes the package-level logger; the call handlers log on
// connect/disconnect and GetLogger() returns a nil-backed entry until Init runs.
func TestMain(m *testing.M) {
	logging.Init()
	os.Exit(m.Run())
}

// ── test doubles ────────────────────────────────────────────────────────────

// fakeAuth authorizes any request whose token query param is "valid", mapping it
// to the user id carried in the "as" param. Everything else is rejected.
type fakeAuth struct{}

func (fakeAuth) Authenticate(_ context.Context, r *http.Request) (*identityDomain.AuthContext, error) {
	if r.URL.Query().Get("token") != "valid" {
		return nil, errors.New("unauthorized")
	}
	return &identityDomain.AuthContext{UserID: types.UserID(r.URL.Query().Get("as"))}, nil
}

// memPresence is an in-memory PresenceStore good enough for handler tests.
type memPresence struct {
	mu    sync.Mutex
	rooms map[types.RoomID]map[types.UserID]bool
}

func newMemPresence() *memPresence {
	return &memPresence{rooms: make(map[types.RoomID]map[types.UserID]bool)}
}

func (m *memPresence) Add(_ context.Context, p domain.Participant) error {
	if !p.IsCall {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rooms[p.RoomID] == nil {
		m.rooms[p.RoomID] = make(map[types.UserID]bool)
	}
	m.rooms[p.RoomID][p.UserID] = true
	return nil
}

func (m *memPresence) Remove(_ context.Context, p domain.Participant) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.rooms[p.RoomID]; r != nil {
		delete(r, p.UserID)
	}
	return nil
}

func (m *memPresence) Refresh(context.Context, domain.Participant) error { return nil }

func (m *memPresence) CallCount(_ context.Context, roomID types.RoomID, exclude types.UserID) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for uid := range m.rooms[roomID] {
		if uid != exclude {
			n++
		}
	}
	return n, nil
}

func (m *memPresence) memberCount(roomID types.RoomID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms[roomID])
}

// ── helpers ─────────────────────────────────────────────────────────────────

func newTestServer(presence domain.PresenceStore, maxPeers int) *httptest.Server {
	hub := service.NewHub(presence, nil, maxPeers)
	srv := NewCallWSServer(hub, fakeAuth{})
	return httptest.NewServer(http.HandlerFunc(srv.HandleCallWebSocket))
}

// dialCall opens a /call/ws connection with an allowed Origin (the upgrader
// rejects others).
func dialCall(t *testing.T, base, room, typ, token, as string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(base, "http") +
		"/call/ws?room_id=" + room + "&type=" + typ + "&token=" + token + "&as=" + as
	header := http.Header{"Origin": []string{"http://localhost:3000"}}
	return websocket.DefaultDialer.Dial(url, header)
}

func readJSON(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %q: %v", data, err)
	}
	return m
}

func waitForMembers(p *memPresence, room types.RoomID, want int) {
	for i := 0; i < 100; i++ {
		if p.memberCount(room) == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// ── tests ───────────────────────────────────────────────────────────────────

// C (auth): missing/invalid token is rejected at the HTTP layer with 401.
func TestCallWS_RejectsUnauthenticated(t *testing.T) {
	srv := newTestServer(newMemPresence(), 0)
	defer srv.Close()

	_, resp, err := dialCall(t, srv.URL, "room1", "call", "bad", "u1")
	if err == nil {
		t.Fatal("expected handshake to fail with a bad token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

// Missing room_id is a 400 before auth.
func TestCallWS_RequiresRoomID(t *testing.T) {
	srv := newTestServer(newMemPresence(), 0)
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/call/ws?type=call&token=valid&as=u1"
	_, resp, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err == nil {
		t.Fatal("expected failure without room_id")
	}
	if resp == nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", resp)
	}
}

// A (count bootstrap): first caller sees 0; second sees 1.
func TestCallWS_CountBootstrap(t *testing.T) {
	presence := newMemPresence()
	srv := newTestServer(presence, 0)
	defer srv.Close()

	connA, _, err := dialCall(t, srv.URL, "room1", "call", "valid", "alice")
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	defer connA.Close()

	if err := connA.WriteMessage(websocket.TextMessage, []byte(`{"type":"checkCountUserCall"}`)); err != nil {
		t.Fatalf("A write bootstrap: %v", err)
	}
	respA := readJSON(t, connA)
	if respA["type"] != domain.TypeCheckCount || int(respA["count"].(float64)) != 0 {
		t.Fatalf("A expected count 0, got %v", respA)
	}

	// Ensure A is registered before B's bootstrap runs.
	waitForMembers(presence, "room1", 1)

	connB, _, err := dialCall(t, srv.URL, "room1", "call", "valid", "bob")
	if err != nil {
		t.Fatalf("dial B: %v", err)
	}
	defer connB.Close()

	if err := connB.WriteMessage(websocket.TextMessage, []byte(`{"type":"checkCountUserCall"}`)); err != nil {
		t.Fatalf("B write bootstrap: %v", err)
	}
	respB := readJSON(t, connB)
	if int(respB["count"].(float64)) != 1 {
		t.Fatalf("B expected count 1, got %v", respB)
	}
}

// B (relay): a frame from one call peer reaches the other, not the sender.
func TestCallWS_RelaysFrames(t *testing.T) {
	presence := newMemPresence()
	srv := newTestServer(presence, 0)
	defer srv.Close()

	connA, _, err := dialCall(t, srv.URL, "room1", "call", "valid", "alice")
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	defer connA.Close()
	// Consume A's bootstrap so subsequent frames are relayed.
	_ = connA.WriteMessage(websocket.TextMessage, []byte(`{"type":"checkCountUserCall"}`))
	readJSON(t, connA)
	waitForMembers(presence, "room1", 1)

	connB, _, err := dialCall(t, srv.URL, "room1", "call", "valid", "bob")
	if err != nil {
		t.Fatalf("dial B: %v", err)
	}
	defer connB.Close()
	_ = connB.WriteMessage(websocket.TextMessage, []byte(`{"type":"checkCountUserCall"}`))
	readJSON(t, connB)
	waitForMembers(presence, "room1", 2)

	// A sends a Status frame; B must receive it verbatim.
	frame := `{"type":"Status","statusUser":"Active"}`
	if err := connA.WriteMessage(websocket.TextMessage, []byte(frame)); err != nil {
		t.Fatalf("A relay write: %v", err)
	}

	got := readJSON(t, connB)
	if got["type"] != "Status" || got["statusUser"] != "Active" {
		t.Fatalf("B did not receive relayed frame, got %v", got)
	}

	// A must NOT receive its own frame (short read deadline → timeout expected).
	_ = connA.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := connA.ReadMessage(); err == nil {
		t.Fatal("sender unexpectedly received its own frame")
	}
}

// D (disconnect): when a peer drops, presence for the room decrements.
func TestCallWS_DisconnectClearsPresence(t *testing.T) {
	presence := newMemPresence()
	srv := newTestServer(presence, 0)
	defer srv.Close()

	connA, _, err := dialCall(t, srv.URL, "room1", "call", "valid", "alice")
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	_ = connA.WriteMessage(websocket.TextMessage, []byte(`{"type":"checkCountUserCall"}`))
	readJSON(t, connA)
	waitForMembers(presence, "room1", 1)

	_ = connA.Close()
	waitForMembers(presence, "room1", 0)

	if got := presence.memberCount("room1"); got != 0 {
		t.Fatalf("expected presence cleared after disconnect, got %d", got)
	}
}
