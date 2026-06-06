package storage

import (
	"context"
	"time"

	database "krampus/internal/sqlc"
	"krampus/internal/user/domain"

	"github.com/jackc/pgx/v5/pgtype"
)

type UserStorageError struct {
	Message string
}

type UserStorage struct {
	queries *database.Queries
}

var (
	ErrTokenExpired = &UserStorageError{"token expired"}
)

func NewUserStorage(queries *database.Queries) *UserStorage {
	return &UserStorage{queries: queries}
}

func (e *UserStorageError) UserError() string {
	return e.Message
}

func (s *UserStorage) InsertUser(ctx context.Context, user domain.User) (int64, error) {
	params := database.CreateUserParams{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFaEnabled: pgtype.Bool{Bool: user.TwoFAEnabled, Valid: true},
	}

	createdUser, err := s.queries.CreateUser(ctx, params)
	if err != nil {
		return 0, err
	}
	return createdUser.ID, nil
}

func (s *UserStorage) SelectUserByEmail(ctx context.Context, email string) (domain.User, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:           user.ID,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

func (s *UserStorage) SelectUserByID(ctx context.Context, id int64) (domain.User, error) {
	user, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:           user.ID,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

func (s *UserStorage) BlockUser(ctx context.Context, email, blockedUntil string) error {
	var blockedUntilTime pgtype.Timestamptz
	if blockedUntil != "" {
		t, err := time.Parse(time.RFC3339, blockedUntil)
		if err != nil {
			return err
		}
		blockedUntilTime = pgtype.Timestamptz{Time: t, Valid: true}
	} else {
		blockedUntilTime = pgtype.Timestamptz{Valid: false}
	}

	return s.queries.BlockUser(ctx, database.BlockUserParams{
		Email:        email,
		BlockedUntil: blockedUntilTime,
	})
}
