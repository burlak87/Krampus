package storage

import (
	"context"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type StorageError struct {
	Message string
}

type Storage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &StorageError{"token expired"}
)

func NewStorage(queries *database.Queries) *Storage {
	return &Storage{queries: queries}
}

func (e *StorageError) Error() string {
	return e.Message
}

func (s *Storage) LogAttempt(email string, result bool, attemptTime time.Time) error {
	params := database.CreateLoginAttemptParams{
		Email:       email,
		Success:     result,
		AttemptedAt: pgtype.Timestamptz{Time: attemptTime, Valid: true},
	}

	return s.queries.CreateLoginAttempt(context.Background(), params)
}

func (s *Storage) GetFailedLogAttempts(email string, windowStart time.Time) (int, error) {
	count, err := s.queries.GetRecentFailedAttempts(context.Background(), database.GetRecentFailedAttemptsParams{
		Email:       email,
		AttemptedAt: pgtype.Timestamptz{Time: windowStart, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func (s *Storage) UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error) {
	blockedUntil, err := s.queries.GetBlockedStatus(context.Background(), email)
	if err != nil {
		return []map[string]interface{}{}, nil
	}

	var isBlocked bool
	if blockedUntil.Valid && blockedUntil.Time.After(time.Now()) {
		isBlocked = true
	}

	return []map[string]interface{}{
		{
			"blocked_until": blockedUntil.Time, // Нет такого поля в структуре
			"is_blocked":    isBlocked,
		},
	}, nil
}
