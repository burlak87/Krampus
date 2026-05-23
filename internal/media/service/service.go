package service

import (
	"context"

	"krampus/internal/media/domain"
)

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
}

type Repository interface {
	GetMediaByID(
		ctx context.Context,
		id string,
	) (*domain.MediaFile, error)

	UpdateProcessedMedia(
		ctx context.Context,
		id string,
		compressedSize int64,
		storageKey string,
		thumbnailKey *string,
		width *int,
		height *int,
		status string,
	) error

	CreateJob(
		ctx context.Context,
		job *domain.MediaJob,
	) error

	CompleteJob(
		ctx context.Context,
		jobID int64,
	) error

	FailJob(
		ctx context.Context,
		jobID int64,
		failure string,
	) error
}

type EventBus interface {
	Publish(
		ctx context.Context,
		eventType string,
		payload any,
	) error
}

type Service struct {
	storage Storage
	repo    Repository
	events  EventBus

	encryptionKey []byte
}

func New(
	storage Storage,
	repo Repository,
	events EventBus,
	encryptionKey []byte,
) *Service {

	return &Service{
		storage:       storage,
		repo:          repo,
		events:        events,
		encryptionKey: encryptionKey,
	}
}
