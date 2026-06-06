package service

import "time"

type Snapshot struct {
	ID            int64
	AggregateType string
	AggregateID   string
	LastSequence  int64
	Snapshot      []byte
	CreatedAt     time.Time
}
