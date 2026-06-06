package service

import (
	"context"
	"errors"
)

var ErrQuotaExceeded = errors.New(
	"storage quota exceeded",
)

type QuotaRepository interface {
	GetUsage(
		ctx context.Context,
		userID string,
	) (int64, int64, error)

	AddUsage(
		ctx context.Context,
		userID string,
		bytes int64,
	) error
}

type QuotaService struct {
	repo QuotaRepository
}

func NewQuotaService(repo QuotaRepository) *QuotaService {
	return &QuotaService{repo: repo}
}

func (s *QuotaService) Check(
	ctx context.Context,
	userID string,
	size int64,
) error {

	used, max, err := s.repo.GetUsage(
		ctx,
		userID,
	)

	if err != nil {
		return err
	}

	if used+size > max {
		return ErrQuotaExceeded
	}

	return nil
}
