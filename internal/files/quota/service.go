package quota

import (
	"context"
	"errors"
)

var ErrQuotaExceeded = errors.New(
	"storage quota exceeded",
)

type Repository interface {
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

type Service struct {
	repo Repository
}

func (s *Service) Check(
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
