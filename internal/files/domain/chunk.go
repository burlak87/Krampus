package domain

import "time"

type Chunk struct {
	SessionID string

	ChunkIndex int64

	ChunkSize int64

	Checksum string

	Data []byte

	Verified bool

	UploadedAt time.Time
}
