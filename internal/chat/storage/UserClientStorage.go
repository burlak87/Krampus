package storage

import (
	"context"
	"fmt"
	"strconv"
	"time"

	database "krampus/internal/sqlc"
	"krampus/internal/user/domain"

	"github.com/jackc/pgx/v5/pgtype"
)

type UserClientPGStorage struct {
	queries *database.Queries
}

func NewUserClientPGStorage(queries *database.Queries) *UserClientPGStorage {
	return &UserClientPGStorage{queries: queries}
}

func (s *UserClientPGStorage) GetUserClient(ctx context.Context, id string) (*domain.User, error) {
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user id format: %w", err)
	}

	user, err := s.queries.GetUserByID(ctx, idInt)
	if err != nil {
		return nil, err
	}
	return &domain.User{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		LastActive: user.CreatedAt.Time.UnixNano(),
	}, nil
}

func (s *UserClientPGStorage) UpdateLastActivity(ctx context.Context, userID string, timestamp int64) error {
	idInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return err
	}

	updateTime := time.Unix(0, timestamp)

	return s.queries.UpdateUserLastActive(ctx, database.UpdateUserLastActiveParams{
		ID:        idInt,
		UpdatedAt: pgtype.Timestamptz{Time: updateTime, Valid: true},
	})
}

func (s *UserClientPGStorage) SaveUserClient(ctx context.Context, user *domain.User) error {
	return s.queries.CreateUserClient(ctx, database.CreateUserClientParams{
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: "system_generated",
	})
}

func (s *UserClientPGStorage) UpdateUserClient(ctx context.Context, user *domain.User) error {
	return s.queries.UpdateUserClient(ctx, database.UpdateUserClientParams{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}
