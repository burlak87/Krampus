package adapters

import "sync/atomic"

type EventSequencer struct {
	sequence atomic.Int64
}

func NewEventSequencer() *EventSequencer {
	return &EventSequencer{}
}

func (s *EventSequencer) Next() int64 {
	return s.sequence.Add(1)
}
