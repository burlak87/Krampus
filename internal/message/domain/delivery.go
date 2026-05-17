package domain

import (
	"time"

	"krampus/pkg/types"
)

type DeliveryStatus string

const (
	DeliverySent      DeliveryStatus = "sent"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryRead      DeliveryStatus = "read"
	DeliveryFailed    DeliveryStatus = "failed"
)

type DeliveryJob struct {
	Message *BaseMessage

	ClientID string

	UserID types.UserID
	RoomID types.RoomID

	Attempt int

	NextRetryAt time.Time
}

type AckPayload struct {
	MessageID types.MessageID `json:"message_id"`
	Status    DeliveryStatus  `json:"status"`
	Timestamp int64           `json:"timestamp"`
}
