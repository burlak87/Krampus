package domain

import "time"

type MediaJob struct {
	ID int64

	MediaFileID string

	JobType string

	Status string

	Attempts int

	ScheduledAt time.Time

	StartedAt *time.Time

	CompletedAt *time.Time

	Error *string

	CreatedAt time.Time
}
