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
		Firstname:  user.Firstname,
		Lastname:   user.Lastname,
		Email:      user.Email,
		Avatar:     user.Avatar.String,
		LastActive: user.CreatedAt.Time.UnixNano(),
	}, nil
}

func (s *UserClientPGStorage) SetAvatar(ctx context.Context, userID string, avatar string) error {
	idInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user id format: %w", err)
	}
	return s.queries.SetUserAvatar(ctx, database.SetUserAvatarParams{
		ID:     idInt,
		Avatar: pgtype.Text{String: avatar, Valid: avatar != ""},
	})
}

func (s *UserClientPGStorage) ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	rows, err := s.queries.ListUsers(ctx, database.ListUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	users := make([]*domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, &domain.User{
			ID:           row.ID,
			Username:     row.Username,
			Firstname:    row.Firstname,
			Lastname:     row.Lastname,
			Email:        row.Email,
			Avatar:       row.Avatar.String,
			TwoFAEnabled: row.TwoFaEnabled.Bool,
			LastActive:   row.CreatedAt.Time.UnixNano(),
		})
	}
	return users, nil
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
