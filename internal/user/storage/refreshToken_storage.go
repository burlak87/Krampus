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

func (s *RefreshTokenStorage) RefreshStore(userID int64, token string, expiresAt time.Time) error {
	params := database.CreateRefreshTokenParams{
		UserID:    userID,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}

	return s.queries.CreateRefreshToken(context.Background(), params)
}

func (s *RefreshTokenStorage) RefreshGet(token string) (int64, error) {
	refreshToken, err := s.queries.GetRefreshToken(context.Background(), token)
	if err != nil {
		return 0, err
	}

	if time.Now().After(refreshToken.ExpiresAt.Time) {
		s.RefreshDelete(token)
		return 0, ErrRefreshTokenExpired
	}

	return refreshToken.UserID, nil
}

func (s *RefreshTokenStorage) RefreshDelete(token string) error {
	return s.queries.DeleteRefreshToken(context.Background(), token)
}

func (s *RefreshTokenStorage) RefreshDeleteByUserID(userID int64) error {
	return s.queries.RefreshDeleteByUserID(context.Background(), userID)
}
