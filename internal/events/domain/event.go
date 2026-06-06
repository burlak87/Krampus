package domain

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID            int64
	AggregateID   string
	AggregateType string
	EventType     string
	Payload       json.RawMessage
	EventVersion  int
	Sequence      int64
	CreatedAt     time.Time
}
