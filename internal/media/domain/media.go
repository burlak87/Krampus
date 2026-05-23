package domain

import "time"

type MediaFile struct {
	ID               string
	OwnerID          string
	MediaType        string
	MimeType         string
	OriginalName     string
	OriginalSize     int64
	CompressedSize   int64
	StorageKey       string
	ThumbnailKey     string
	SHA256           string
	Width            int
	Height           int
	DurationSeconds  int
	ProcessingStatus string
	CreatedAt        time.Time
}
