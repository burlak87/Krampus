package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"krampus/internal/call/domain"
	"krampus/pkg/types"
)

// fakePeer is a test double for the WS client implementing Peer. It records
// every frame delivered to it.
type fakePeer struct {
	userID types.UserID
	roomID types.RoomID
	isCall bool

	mu       sync.Mutex
	received [][]byte
}

func (p *fakePeer) UserID() types.UserID { return p.userID }
func (p *fakePeer) RoomID() types.RoomID { return p.roomID }
func (p *fakePeer) IsCall() bool         { return p.isCall }
func (p *fakePeer) Send(data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, data)
}
func (p *fakePeer) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.received)
}

// fakePresence is a no-op PresenceStore with a stubbed CallCount.
type fakePresence struct {
	count int
	err   error
}

func (f *fakePresence) Add(context.Context, domain.Participant) error     { return nil }
func (f *fakePresence) Remove(context.Context, domain.Participant) error  { return nil }
func (f *fakePresence) Refresh(context.Context, domain.Participant) error { return nil }
func (f *fakePresence) CallCount(context.Context, types.RoomID, types.UserID) (int, error) {
	return f.count, f.err
}

func newCallPeer(user, room string) *fakePeer {
	return &fakePeer{userID: types.UserID(user), roomID: types.RoomID(room), isCall: true}
}

// Broadcast must reach every peer in the room except the sender.
func TestBroadcastExcludesSender(t *testing.T) {
	hub := NewHub(&fakePresence{}, nil, 0)
	ctx := context.Background()

	a := newCallPeer("a", "room1")
	b := newCallPeer("b", "room1")
	c := newCallPeer("c", "room1")
	other := newCallPeer("d", "room2") // different room — must not receive

	for _, p := range []*fakePeer{a, b, c, other} {
		if err := hub.Register(ctx, p); err != nil {
			t.Fatalf("Register(%s): %v", p.userID, err)
		}
	}

	hub.Broadcast(ctx, "room1", []byte(`{"type":"Status"}`), a)

	if a.count() != 0 {
		t.Errorf("sender received its own frame: got %d", a.count())
	}
	if b.count() != 1 || c.count() != 1 {
		t.Errorf("peers did not each receive one frame: b=%d c=%d", b.count(), c.count())
	}
	if other.count() != 0 {
		t.Errorf("peer in another room received frame: got %d", other.count())
	}
}

// The maxPeers cap rejects the call-peer that would exceed it.
func TestRegisterEnforcesMaxPeers(t *testing.T) {
	hub := NewHub(&fakePresence{}, nil, 2)
	ctx := context.Background()

	if err := hub.Register(ctx, newCallPeer("a", "r")); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := hub.Register(ctx, newCallPeer("b", "r")); err != nil {
		t.Fatalf("second register: %v", err)
	}
	err := hub.Register(ctx, newCallPeer("c", "r"))
	if !errors.Is(err, ErrRoomFull) {
		t.Fatalf("expected ErrRoomFull, got %v", err)
	}
}

// Non-call peers (chat presence listeners) must not count against the cap.
func TestRegisterIgnoresNonCallPeersForCap(t *testing.T) {
	hub := NewHub(&fakePresence{}, nil, 1)
	ctx := context.Background()

	chat := &fakePeer{userID: "chat", roomID: "r", isCall: false}
	if err := hub.Register(ctx, chat); err != nil {
		t.Fatalf("register chat peer: %v", err)
	}
	// A single call peer is still allowed despite the chat peer present.
	if err := hub.Register(ctx, newCallPeer("a", "r")); err != nil {
		t.Fatalf("call peer should be admitted: %v", err)
	}
	// The next call peer exceeds the cap of 1.
	if err := hub.Register(ctx, newCallPeer("b", "r")); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("expected ErrRoomFull, got %v", err)
	}
}

// maxPeers == 0 means unlimited.
func TestRegisterUnlimitedWhenMaxZero(t *testing.T) {
	hub := NewHub(&fakePresence{}, nil, 0)
	ctx := context.Background()
	for i, name := range []string{"a", "b", "c", "d", "e"} {
		if err := hub.Register(ctx, newCallPeer(name, "r")); err != nil {
			t.Fatalf("register %d: %v", i, err)
		}
	}
}

// After Unregister, a freed slot lets a new call peer in.
func TestUnregisterFreesCapSlot(t *testing.T) {
	hub := NewHub(&fakePresence{}, nil, 1)
	ctx := context.Background()

	a := newCallPeer("a", "r")
	if err := hub.Register(ctx, a); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := hub.Register(ctx, newCallPeer("b", "r")); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("expected room full before unregister, got %v", err)
	}

	hub.Unregister(ctx, a)

	if err := hub.Register(ctx, newCallPeer("b", "r")); err != nil {
		t.Fatalf("register after freeing slot: %v", err)
	}
}

// DeliverFromCrossNode fans a frame to all local peers with no sender excluded.
func TestDeliverFromCrossNode(t *testing.T) {
	hub := NewHub(&fakePresence{}, nil, 0)
	ctx := context.Background()

	a := newCallPeer("a", "room1")
	b := newCallPeer("b", "room1")
	_ = hub.Register(ctx, a)
	_ = hub.Register(ctx, b)

	hub.DeliverFromCrossNode("room1", []byte(`{"type":"iceCandidate"}`))

	if a.count() != 1 || b.count() != 1 {
		t.Errorf("cross-node frame not delivered to all: a=%d b=%d", a.count(), b.count())
	}
}

// CallCount delegates to the presence store.
func TestCallCountDelegatesToPresence(t *testing.T) {
	hub := NewHub(&fakePresence{count: 3}, nil, 0)
	got, err := hub.CallCount(context.Background(), "r", "self")
	if err != nil {
		t.Fatalf("CallCount: %v", err)
	}
	if got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

// CheckCountResponse serializes the documented bootstrap reply shape.
func TestCheckCountResponse(t *testing.T) {
	got := string(CheckCountResponse(2))
	want := `{"type":"checkCountUserCall","count":2}`
	if got != want {
		t.Errorf("CheckCountResponse mismatch:\n got %s\nwant %s", got, want)
	}
}
