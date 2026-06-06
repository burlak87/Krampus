package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"krampus/internal/call/domain"
	"krampus/pkg/logging"
	"krampus/pkg/types"
)

// ErrRoomFull is returned by Register when the call-peer cap has been reached.
var ErrRoomFull = errors.New("call: room is full")

// Peer is a single connected call/chat client, as seen by the hub. The WS
// adapter's client implements this interface.
type Peer interface {
	UserID() types.UserID
	RoomID() types.RoomID
	IsCall() bool
	// Send enqueues a frame for delivery to this peer (non-blocking; drops on
	// full buffer, mirroring the PoC's `select { case send <- data: default: }`).
	Send(data []byte)
}

// CrossNodePublisher fans a frame out to other backend instances. In a
// single-instance deployment this is nil and the hub does pure in-process
// fan-out (identical to the PoC). With Redis Pub/Sub wired in it lets two peers
// on different instances signal each other.
type CrossNodePublisher interface {
	Publish(ctx context.Context, roomID types.RoomID, frame []byte) error
}

// Hub is the in-process room registry + relay, ported from the PoC's
// `rooms map[string]map[*Client]bool` + broadcast/addClient/removeClient, with
// presence persisted to Redis and optional cross-node fan-out added.
type Hub struct {
	mu       sync.RWMutex
	rooms    map[types.RoomID]map[Peer]bool
	presence domain.PresenceStore
	pub      CrossNodePublisher
	maxPeers int
	log      *logging.Logger
}

func NewHub(presence domain.PresenceStore, pub CrossNodePublisher, maxPeers int) *Hub {
	return &Hub{
		rooms:    make(map[types.RoomID]map[Peer]bool),
		presence: presence,
		pub:      pub,
		maxPeers: maxPeers,
		log:      logging.GetLogger(),
	}
}

// Register adds a peer to its room and records call presence in Redis.
// Returns ErrRoomFull if the call-peer cap (maxPeers) would be exceeded.
func (h *Hub) Register(ctx context.Context, p Peer) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if p.IsCall() && h.maxPeers > 0 {
		callPeers := 0
		for peer := range h.rooms[p.RoomID()] {
			if peer.IsCall() {
				callPeers++
			}
		}
		if callPeers >= h.maxPeers {
			return ErrRoomFull
		}
	}

	if h.rooms[p.RoomID()] == nil {
		h.rooms[p.RoomID()] = make(map[Peer]bool)
	}
	h.rooms[p.RoomID()][p] = true

	return h.presence.Add(ctx, h.participant(p))
}

// Unregister removes a peer and clears its presence.
func (h *Hub) Unregister(ctx context.Context, p Peer) {
	h.mu.Lock()
	if peers, ok := h.rooms[p.RoomID()]; ok {
		delete(peers, p)
		if len(peers) == 0 {
			delete(h.rooms, p.RoomID())
		}
	}
	h.mu.Unlock()

	if err := h.presence.Remove(ctx, h.participant(p)); err != nil {
		h.log.Errorf("call: presence remove failed room=%s user=%s: %v",
			p.RoomID(), p.UserID(), err)
	}
}

// Broadcast relays a frame to every other peer in the room (sender excluded),
// then publishes to other instances if a cross-node publisher is configured.
func (h *Hub) Broadcast(ctx context.Context, roomID types.RoomID, frame []byte, sender Peer) {
	h.deliverLocal(roomID, frame, sender)

	if h.pub != nil {
		if err := h.pub.Publish(ctx, roomID, frame); err != nil {
			h.log.Errorf("call: cross-node publish failed room=%s: %v", roomID, err)
		}
	}
}

// DeliverFromCrossNode delivers a frame that arrived from another instance to
// all local peers in the room. There is no local sender to exclude.
func (h *Hub) DeliverFromCrossNode(roomID types.RoomID, frame []byte) {
	h.deliverLocal(roomID, frame, nil)
}

func (h *Hub) deliverLocal(roomID types.RoomID, frame []byte, sender Peer) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for peer := range h.rooms[roomID] {
		if peer != sender {
			peer.Send(frame)
		}
	}
}

// CallCount returns the number of call-peers in a room, excluding the given user.
func (h *Hub) CallCount(ctx context.Context, roomID types.RoomID, exclude types.UserID) (int, error) {
	return h.presence.CallCount(ctx, roomID, exclude)
}

// Refresh extends a peer's presence TTL (called on ws ping).
func (h *Hub) Refresh(ctx context.Context, p Peer) error {
	return h.presence.Refresh(ctx, h.participant(p))
}

// CheckCountResponse builds the JSON reply to a TypeCheckCount bootstrap frame.
func CheckCountResponse(count int) []byte {
	b, _ := json.Marshal(domain.CheckCountResponse{
		Type:  domain.TypeCheckCount,
		Count: count,
	})
	return b
}

func (h *Hub) participant(p Peer) domain.Participant {
	return domain.Participant{
		UserID: p.UserID(),
		RoomID: p.RoomID(),
		IsCall: p.IsCall(),
	}
}
