package storage

import (
	"context"
	"time"

	"krampus/internal/auth/domain"
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

func (s *Storage) RenovationTwoFAStatus(ctx context.Context, userID int64, enabled bool) error {
	return s.queries.UpdateTwoFAStatus(ctx, database.UpdateTwoFAStatusParams{
		ID:           userID,
		TwoFaEnabled: pgtype.Bool{Bool: enabled, Valid: true},
	})
}

func (s *Storage) InsertTwoFaCode(ctx context.Context, userID int64, code string, expiresAt time.Time) error {
	_, err := s.queries.CreateTwoFaCode(ctx, database.CreateTwoFaCodeParams{
		UserID:    userID,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return err
}

func (s *Storage) SelectTwoFaCodeByUserID(ctx context.Context, userID int64) (domain.TwoFaCode, error) {
	code, err := s.queries.GetTwoFaCodeByUserID(ctx, userID)
	if err != nil {
		return domain.TwoFaCode{}, err
	}

	return domain.TwoFaCode{
		ID:        code.ID,
		UserID:    code.UserID,
		Code:      code.Code,
		ExpiresAt: code.ExpiresAt.Time,
		Attempts:  int(code.Attempts.Int32),
		IsUsed:    code.IsUsed.Bool,
		CreatedAt: code.CreatedAt.Time,
	}, nil
}

func (s *Storage) RenovationTwoFaCodeAttempts(ctx context.Context, codeID int64, attempts int64) error {
	return s.queries.UpdateTwoFaCodeAttempts(ctx, database.UpdateTwoFaCodeAttemptsParams{
		ID:       codeID,
		Attempts: pgtype.Int4{Int32: int32(attempts), Valid: true},
	})
}

func (s *Storage) MarkTwoFaCodeUsed(ctx context.Context, codeID int64) error {
	return s.queries.MarkTwoFaCodeAsUsed(ctx, codeID)
}

func (s *Storage) SelectRecentCodeRequests(ctx context.Context, userID int64, since time.Time) (int64, error) {
	count, err := s.queries.GetRecentCodeRequests(ctx, database.GetRecentCodeRequestsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *Storage) SelectRecentVerificationAttempts(ctx context.Context, userID int64, since time.Time) (int64, error) {
	count, err := s.queries.GetRecentVerificationAttempts(ctx, database.GetRecentVerificationAttemptsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}

	return count, nil
}
