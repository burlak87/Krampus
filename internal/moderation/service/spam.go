package service

import (
	"sync"
	"time"
)

type SpamProtector struct {
	mu sync.Mutex

	lastMessages map[string][]time.Time
}

func NewSpamProtector() *SpamProtector {

	return &SpamProtector{
		lastMessages: make(
			map[string][]time.Time,
		),
	}
}

func (s *SpamProtector) Allow(
	userID string,
) bool {

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	messages := s.lastMessages[userID]

	var filtered []time.Time

	for _, t := range messages {

		if now.Sub(t) < time.Minute {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= 20 {
		return false
	}

	filtered = append(filtered, now)

	s.lastMessages[userID] = filtered

	return true
}
