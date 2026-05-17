package domain

import (
	"encoding/json"
	"krampus/pkg/types"
)

type Event struct {
	ID          string          `json:"id"`
	Sequence    int64           `json:"sequence"`
	Type        string          `json:"type"`
	AggregateID string          `json:"aggregate_id"`
	UserID      types.UserID    `json:"user_id,omitempty"`
	RoomID      types.RoomID    `json:"room_id,omitempty"`
	Timestamp   int64           `json:"timestamp"`
	Payload     json.RawMessage `json:"payload"`
}
