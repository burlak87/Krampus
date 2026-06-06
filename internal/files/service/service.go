package service

import (
	"context"
	"errors"
)

var (
	ErrInvalidChecksum = errors.New(
		"invalid checksum",
	)
)

type RepositoryInterface interface {
	AddChunk(
		ctx context.Context,
		sessionID string,
		chunkIndex int64,
		data []byte,
		checksum string,
	) error

	ChunkExists(
		ctx context.Context,
		sessionID string,
		chunkIndex int64,
	) (bool, error)
}

type Service struct {
	repo RepositoryInterface
}

func NewService(
	repo RepositoryInterface,
) *Service {

	return &Service{
		repo: repo,
	}
}

func (s *Service) UploadChunk(
	ctx context.Context,
	sessionID string,
	chunkIndex int64,
	data []byte,
	expectedChecksum string,
) error {

	valid := VerifyChecksum(
		data,
		expectedChecksum,
	)

	if !valid {
		return ErrInvalidChecksum
	}

	exists, err := s.repo.ChunkExists(
		ctx,
		sessionID,
		chunkIndex,
	)

	if err != nil {
		return err
	}

	if exists {
		return ErrChunkAlreadyUploaded
	}

	return s.repo.AddChunk(
		ctx,
		sessionID,
		chunkIndex,
		data,
		expectedChecksum,
	)
}
