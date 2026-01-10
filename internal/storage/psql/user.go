package psql

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/your-team/taskmanager-chat/backend/internal/domain"
	"github.com/your-team/taskmanager-chat/backend/internal/storage/psql/sqlc"
)

type StorageError struct {
	Message string
}

type Storage struct {
	queries *database.Queries
}

func NewStorage(queries *database.Queries) *Storage {
	return &Storage{queries: queries}
}

var (
	ErrTokenExpired = &StorageError{"token expired"}
)

func (e *StorageError) Error() string {
	return e.Message
}

func (s *Storage) InsertUser(user domain.User) (int64, error) {
	params := database.CreateUserParams{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:     	  user.Email,
		PasswordHash: user.PasswordHash,
		TwoFaEnabled: pgtype.Bool{Bool: user.TwoFAEnabled, Valid: true},
	}
	
	createdUser, err := s.queries.CreateUser(context.Background(), params)
	if err != nil {
		return 0, err
	}
	return createdUser.ID, nil
}

func (s *Storage) SelectUsers(email string) (domain.User, error) {
	user, err := s.queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return domain.User{}, err
	}
	
	return domain.User{
		ID: user.ID,
		Username: user.Username,
		Firstname: user.Firstname,
		Lastname: user.Lastname,
		Email: user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt: user.CreatedAt.Time,
	}, nil
}

func (s *Storage) SelectUserByID(id int64) (domain.User, error) {
	user, err := s.queries.GetUserByID(context.Background(), id)
	if err != nil {
		return domain.User{}, nil
	}
	
	return domain.User{
		ID: user.ID,
		Username: user.Username,
		Firstname: user.Firstname,
		Lastname: user.Lastname,
		Email: user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFaEnabled.Bool,
		CreatedAt: user.CreatedAt.Time,
	}, nil
}

func (s *Storage) RefreshStore(userID int64, token string, expiresAt time.Time) error {
	params := database.CreateRefreshTokenParams{
		UserID: userID,
		Token: token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}
	
	return s.queries.CreateRefreshToken(context.Background(), params)
}

func (s *Storage) RefreshGet(token string) (int64, error) {
	refreshToken, err := s.queries.GetRefreshToken(context.Background(), token)
	if err != nil {
		return 0, err
	}
	
	if time.Now().After(refreshToken.ExpiresAt.Time) {
		s.RefreshDelete(token)
		return 0, ErrTokenExpired
	}
	
	return refreshToken.UserID, nil
}

func (s *Storage) RefreshDelete(token string) error {
	return s.queries.DeleteRefreshToken(context.Background(), token)
}

func (s *Storage) LogAttempt(email string, result bool, attemptTime time.Time) error {
	params := database.CreateLoginAttemptParams{
		Email: email,
		Success: result,
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
			"blocked_until": blockedUntil.Time,
			"is_blocked": isBlocked,
		},
	}, nil
}

func (s *Storage) BlockUser(email, blockedUntil string) error {
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

func (s *Storage) RenovationTwoFAStatus(userID int64, enabled bool) error {
	return s.queries.UpdateTwoFAStatus(context.Background(), database.UpdateTwoFAStatusParams{
		ID:           userID,
		TwoFaEnabled: pgtype.Bool{Bool: enabled, Valid: true},
	})
}

func (s *Storage) ResetFailedAttempts(email string) error {
	return s.queries.ResetFailedAttempts(context.Background(), email)
}

func (s *Storage) UpdatePasswordHash(userID int64, passwordHash string) error {
	return s.queries.UpdatePasswordHash(context.Background(), database.UpdatePasswordHashParams{
		ID:           userID,
		PasswordHash: passwordHash,
	})
}

func (s *Storage) DeleteExpiredRefreshTokens() error {
	return s.queries.DeleteExpiredRefreshTokens(context.Background())
}

func (s *Storage) RefreshDeleteByUserID(userID int64) error {
	return s.queries.RefreshDeleteByUserID(context.Background(), userID)
}

func (s *Storage) InsertTwoFaCode(userID int64, code string, expiresAt time.Time) error {
	_, err := s.queries.CreateTwoFaCode(context.Background(), database.CreateTwoFaCodeParams{
		UserID:    userID,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return err
}

func (s *Storage) SelectTwoFaCodeByUserID(userID int64) (domain.TwoFaCode, error) {
	code, err := s.queries.GetTwoFaCodeByUserID(context.Background(), userID)
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

func (s *Storage) RenovationTwoFaCodeAttempts(codeID int64, attempts int) error {
	return s.queries.UpdateTwoFaCodeAttempts(context.Background(), database.UpdateTwoFaCodeAttemptsParams{
		ID:       codeID,
		Attempts: pgtype.Int4{Int32: int32(attempts), Valid: true},
	})
}

func (s *Storage) MarkTwoFaCodeUsed(codeID int64) error {
	return s.queries.MarkTwoFaCodeAsUsed(context.Background(), codeID)
}

func (s *Storage) SelectRecentCodeRequests(userID int64, since time.Time) (int, error) {
	count, err := s.queries.GetRecentCodeRequests(context.Background(), database.GetRecentCodeRequestsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	
	return int(count), nil
}

func (s *Storage) SelectRecentVerificationAttempts(userID int64, since time.Time) (int, error) {
	count, err := s.queries.GetRecentVerificationAttempts(context.Background(), database.GetRecentVerificationAttemptsParams{
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	
	return int(count), nil
}