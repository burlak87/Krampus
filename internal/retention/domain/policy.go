package domain

import "time"

type Policy struct {
	ID            int64
	MediaType     string
	RetentionDays int
	AutoDelete    bool
	CreatedAt     time.Time
}
