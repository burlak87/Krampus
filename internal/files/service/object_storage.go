package service

import "context"

type ObjectStorage interface {
	PutObject(
		ctx context.Context,
		key string,
		data []byte,
	) error

	GetObject(
		ctx context.Context,
		key string,
	) ([]byte, error)

	DeleteObject(
		ctx context.Context,
		key string,
	) error
}
