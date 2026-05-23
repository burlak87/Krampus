package storage

import "context"

type Storage interface {
	Upload(
		ctx context.Context,
		key string,
		data []byte,
	) error

	Download(
		ctx context.Context,
		key string,
	) ([]byte, error)

	Delete(
		ctx context.Context,
		key string,
	) error

	Exists(
		ctx context.Context,
		key string,
	) (bool, error)
}
