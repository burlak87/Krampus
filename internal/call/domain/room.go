package domain

import (
	"context"

	"krampus/pkg/types"
)

// Participant is a single connected peer in a call room.
type Participant struct {
	UserID types.UserID
	RoomID types.RoomID
	// IsCall is true when the peer joined the media/call channel (typeWS=="call")
	// rather than a plain chat channel.
	IsCall bool
}

// PresenceStore tracks call-room membership. It is backed by Redis so presence
// survives across multiple backend instances (the PoC used an in-process map,
// which only works single-process).
type PresenceStore interface {
	// Add registers a call peer in a room and returns nothing; TTL is refreshed
	// on heartbeat via Refresh.
	Add(ctx context.Context, p Participant) error
	// Remove deregisters a peer from a room.
	Remove(ctx context.Context, p Participant) error
	// Refresh extends the TTL on a peer's presence key (called on ws ping).
	Refresh(ctx context.Context, p Participant) error
	// CallCount returns the number of call-peers in a room, excluding the given
	// user (pass an empty UserID to count all).
	CallCount(ctx context.Context, roomID types.RoomID, exclude types.UserID) (int, error)
}
