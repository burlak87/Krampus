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

func (s *UserStorage) InsertUser(user domain.User) (int64, error) {
	params := database.CreateUserParams{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFaEnabled: pgtype.Bool{Bool: user.TwoFAEnabled, Valid: true},
	}

	createdUser, err := s.queries.CreateUser(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return createdUser.ID, nil
}

func (s *UserStorage) SelectUserByEmail(email string) (domain.User, error) {
	user, err := s.queries.GetUserByEmail(context.Background(), email)
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

func (s *UserStorage) SelectUserByID(id int64) (domain.User, error) {
	user, err := s.queries.GetUserByID(context.Background(), id)
	if err != nil {
		return domain.User{}, nil
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

func (s *UserStorage) BlockUser(email, blockedUntil string) error {
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

	return s.queries.BlockUser(context.Background(), database.BlockUserParams{
		Email:        email,
		BlockedUntil: blockedUntilTime,
	})
}
