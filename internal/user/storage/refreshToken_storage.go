package storage

import (
	"context"
	"errors"
	"time"

	database "krampus/internal/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

type RefreshTokenStorageError struct {
	Message string
}

type RefreshTokenStorage struct {
	queries *database.Queries
}

var (
	ErrRefreshTokenExpired = errors.New("token expired")
)

func NewRefreshTokenStorage(queries *database.Queries) *RefreshTokenStorage {
	return &RefreshTokenStorage{queries: queries}
}

func (e *RefreshTokenStorageError) RefreshTokenError() string {
	return e.Message
}

func (s *RefreshTokenStorage) RefreshStore(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	params := database.CreateRefreshTokenParams{
		UserID:    userID,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}

	return s.queries.CreateRefreshToken(ctx, params)
}

func (s *RefreshTokenStorage) RefreshGet(ctx context.Context, token string) (int64, error) {
	refreshToken, err := s.queries.GetRefreshToken(ctx, token)
	if err != nil {
		return 0, err
	}

	if time.Now().After(refreshToken.ExpiresAt.Time) {
		s.RefreshDelete(ctx, token)
		return 0, ErrRefreshTokenExpired
	}

	return refreshToken.UserID, nil
}

func (s *RefreshTokenStorage) RefreshDelete(ctx context.Context, token string) error {
	return s.queries.DeleteRefreshToken(ctx, token)
}

func (s *RefreshTokenStorage) RefreshDeleteByUserID(ctx context.Context, userID int64) error {
	return s.queries.RefreshDeleteByUserID(ctx, userID)
}
