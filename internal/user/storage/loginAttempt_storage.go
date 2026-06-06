package storage

import (
	"context"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type LoginAttemptStorageError struct {
	Message string
}

type LoginAttemptStorage struct {
	queries *database.Queries
}

var (
	ErrLoginAttemptExpired = &LoginAttemptStorageError{"token expired"}
)

func NewLoginAttemptStorage(queries *database.Queries) *LoginAttemptStorage {
	return &LoginAttemptStorage{queries: queries}
}

func (e *LoginAttemptStorageError) LoginAttemptError() string {
	return e.Message
}

func (s *LoginAttemptStorage) LogAttempt(ctx context.Context, email string, result bool, attemptTime time.Time) error {
	params := database.CreateLoginAttemptParams{
		Email:       email,
		Success:     result,
		AttemptedAt: pgtype.Timestamptz{Time: attemptTime, Valid: true},
	}

	return s.queries.CreateLoginAttempt(ctx, params)
}

func (s *LoginAttemptStorage) GetFailedLogAttempts(ctx context.Context, email string, windowStart time.Time) (int64, error) {
	count, err := s.queries.GetRecentFailedAttempts(ctx, database.GetRecentFailedAttemptsParams{
		Email:       email,
		AttemptedAt: pgtype.Timestamptz{Time: windowStart, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *LoginAttemptStorage) UserBlocked(ctx context.Context, email string, windowStart time.Time) ([]map[string]interface{}, error) {
	blockedUntil, err := s.queries.GetBlockedStatus(ctx, email)
	if err != nil {
		return []map[string]interface{}{}, nil
	}

	var isBlocked bool
	if blockedUntil.Valid && blockedUntil.Time.After(time.Now()) {
		isBlocked = true
	}

	return []map[string]interface{}{
		{
			"blocked_until": blockedUntil.Time,
			"is_blocked":    isBlocked,
		},
	}, nil
}
