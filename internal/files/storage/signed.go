package storage

import (
	"context"
	"time"
)

type SignedURLStorage interface {
	GenerateDownloadURL(
		ctx context.Context,
		key string,
		expires time.Duration,
	) (string, error)
}
