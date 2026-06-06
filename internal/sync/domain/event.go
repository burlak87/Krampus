package domain

import "time"

type Event struct {
	ID int64

	AggregateID string

	AggregateType string

	EventType string

	Payload []byte

	CreatedAt time.Time
}
