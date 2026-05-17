package domain

import (
	"time"

	"krampus/pkg/types"
)

type OutboxEvent struct {
	ID string

	AggregateType string
	AggregateID   string

	EventType string

	Payload []byte

	CreatedAt time.Time

	PublishedAt *time.Time

	RetryCount int

	LastError string
}

const (
	OutboxEventMessageCreated = "message.created"
)

type MessageCreatedEvent struct {
	MessageID types.MessageID `json:"message_id"`
	RoomID    types.RoomID    `json:"room_id"`
	UserID    types.UserID    `json:"user_id"`
}
