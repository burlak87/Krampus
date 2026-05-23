package upload

import "time"

type Session struct {
	ID string

	UserID string

	FileName string

	MimeType string

	TotalSize int64

	TotalChunks int64

	UploadedChunks int64

	ExpectedHash string

	UploadStatus string

	Locked bool

	Completed bool

	ExpiresAt time.Time

	CreatedAt time.Time

	UpdatedAt time.Time
}
