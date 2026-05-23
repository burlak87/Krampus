package domain

import "time"

type SavedMessage struct {
	UserID    string
	MessageID string
	SavedAt   time.Time
}
