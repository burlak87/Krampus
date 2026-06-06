package service

import (
	"context"
	"errors"
	"math"
	"regexp"
	"time"

	twofa "krampus/internal/auth/domain"
	"krampus/internal/user/domain"
	redis "krampus/internal/user/storage"
	"krampus/pkg/logging"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserStorage interface {
	InsertUser(ctx context.Context, user domain.User) (int64, error)
	SelectUserByEmail(ctx context.Context, email string) (domain.User, error)
	SelectUserByID(ctx context.Context, userID int64) (domain.User, error)
	BlockUser(ctx context.Context, email, blockedUntil string) error
}

type LoginAttemptStorage interface {
	LogAttempt(ctx context.Context, email string, result bool, attemptTime time.Time) error
	GetFailedLogAttempts(ctx context.Context, email string, windowStart time.Time) (int64, error)
	UserBlocked(ctx context.Context, email string, windowStart time.Time) ([]map[string]any, error)
}

type User struct {
	userStorage         UserStorage
	loginAttemptStorage LoginAttemptStorage
	refreshTokenService *RefreshToken
	redisStorage        *redis.RedisSessionStorage
	jwtSecret           string
	logger              *logging.Logger
}

func NewUser(
	user UserStorage,
	loginAttempt LoginAttemptStorage,
	refreshToken *RefreshToken,
	redisStorage *redis.RedisSessionStorage,
	jwt string,
) *User {
	return &User{
		userStorage:         user,
		loginAttemptStorage: loginAttempt,
		refreshTokenService: refreshToken,
		redisStorage:        redisStorage,
		jwtSecret:           jwt,
		logger:              logging.GetLogger(),
	}
}

func (s *User) UserRegister(ctx context.Context, req domain.RegisterRequest) (domain.User, error) {
	if req.Username == "" || req.Firstname == "" || req.Lastname == "" || req.Email == "" {
		return domain.User{}, errors.New("invalid input: all fields are required")
	}

	if req.Password == "" || len(req.Password) < 8 {
		return domain.User{}, errors.New("invalid password: must be at least 8 characters")
	}

	hasLetters, _ := regexp.MatchString(`[a-zA-Zа-яА-Я]`, req.Password)
	hasDigits, _ := regexp.MatchString(`[0-9]`, req.Password)
	hasSpecial, _ := regexp.MatchString(`[^a-zA-Zа-яА-Я0-9\s]`, req.Password)

	if !hasLetters || !hasDigits || !hasSpecial {
		return domain.User{}, errors.New("invalid password: must contain letters, digits and special characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, errors.New("error hashing password")
	}

	userToSave := domain.User{
		Username:     req.Username,
		Firstname:    req.Firstname,
		Lastname:     req.Lastname,
		Email:        req.Email,
		PasswordHash: string(hash),
		TwoFAEnabled: req.TwoFAEnabled,
	}

	id, err := s.userStorage.InsertUser(ctx, userToSave)
	if err != nil {
		return domain.User{}, err
	}

	createdUser := domain.User{
		ID:           id,
		Username:     req.Username,
		Firstname:    req.Firstname,
		Lastname:     req.Lastname,
		Email:        req.Email,
		TwoFAEnabled: req.TwoFAEnabled,
		CreatedAt:    time.Now(),
	}

	s.logger.Infof("user registered: id=%d email=%s", id, req.Email)
	return createdUser, nil
}

func (s *User) UserLogin(ctx context.Context, req domain.LoginRequest) (domain.TokenResponse, twofa.TwoFaCodes, error) {
	if req.Email == "" || req.Password == "" {
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("email and password are required")
	}

	blocked, minutesLeft, err := s.IsUserBlocked(ctx, req.Email)
	if err != nil {
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}
	if blocked {
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("account temporarily blocked")
	}
	_ = minutesLeft

	dbUser, err := s.userStorage.SelectUserByEmail(ctx, req.Email)
	if err != nil {
		s.LogLoginAttempt(ctx, req.Email, false)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("invalid credentials")
	}

	if err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(req.Password)); err != nil {
		s.LogLoginAttempt(ctx, req.Email, false)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("invalid credentials")
	}

	attempts, err := s.GetFailedAttempts(ctx, req.Email)
	if err != nil {
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	if attempts >= int64(5) {
		s.BlockUser(ctx, req.Email)
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, errors.New("too many failed attempts, account blocked")
	}

	if dbUser.TwoFAEnabled {
		tempToken, err := s.GenerateTempToken(dbUser.ID)
		if err != nil {
			return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
		}
		return domain.TokenResponse{}, twofa.TwoFaCodes{RequiresTwoFa: true, TempToken: tempToken}, nil
	}

	accessToken, err := s.GenerateAccessToken(dbUser.ID)
	if err != nil {
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(ctx, dbUser.ID)
	if err != nil {
		return domain.TokenResponse{}, twofa.TwoFaCodes{}, err
	}

	session := domain.CachedSession{
		UserID:       dbUser.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	if err = s.redisStorage.SetAccessToken(ctx, accessToken, session); err != nil {
		s.logger.Warnf("cache set error for user %d: %v", dbUser.ID, err)
	}
	if err = s.redisStorage.SetSessionByUserID(ctx, dbUser.ID, session); err != nil {
		s.logger.Warnf("session cache error for user %d: %v", dbUser.ID, err)
	}

	s.LogLoginAttempt(ctx, req.Email, true)
	s.logger.Infof("user logged in: id=%d", dbUser.ID)
	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, twofa.TwoFaCodes{}, nil
}

func (s *User) UserLogout(ctx context.Context, userID int64, password string) error {
	dbUser, err := s.userStorage.SelectUserByID(ctx, userID)
	if err != nil {
		return errors.New("invalid credentials")
	}

	if err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(password)); err != nil {
		s.LogLoginAttempt(ctx, dbUser.Email, false)
		return errors.New("invalid credentials")
	}

	if err = s.redisStorage.DeleteSessionByUserID(ctx, userID); err != nil {
		s.logger.Warnf("redis session delete error for user %d: %v", userID, err)
	}

	if err = s.refreshTokenService.DeleteRefreshTokensByUserID(ctx, userID); err != nil {
		return err
	}

	s.logger.Infof("user logged out: id=%d", userID)
	return nil
}

func (s *User) BlockUser(ctx context.Context, email string) {
	now := time.Now()
	blockedUntil := now.Add(1 * time.Minute).Format(time.RFC3339)
	s.LogLoginAttempt(ctx, email, false)
	if err := s.userStorage.BlockUser(ctx, email, blockedUntil); err != nil {
		s.logger.Warnf("block user error for %s: %v", email, err)
	}
}

func (s *User) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) GenerateTempToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(10 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *User) LogLoginAttempt(ctx context.Context, email string, result bool) {
	if err := s.loginAttemptStorage.LogAttempt(ctx, email, result, time.Now().UTC()); err != nil {
		s.logger.Warnf("login attempt log error for %s: %v", email, err)
	}
}

func (s *User) GetFailedAttempts(ctx context.Context, email string) (int64, error) {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Minute)
	return s.loginAttemptStorage.GetFailedLogAttempts(ctx, email, windowStart)
}

func (s *User) IsUserBlocked(ctx context.Context, email string) (bool, int64, error) {
	result, err := s.loginAttemptStorage.UserBlocked(ctx, email, time.Now().UTC())
	if err != nil {
		return false, 0, err
	}

	if len(result) > 0 {
		// UserBlocked always returns one row; the is_blocked flag tells us
		// whether the account is actually blocked. blocked_until is a time.Time
		// (zero value when never blocked).
		if blocked, _ := result[0]["is_blocked"].(bool); !blocked {
			return false, 0, nil
		}

		blockedUntil, ok := result[0]["blocked_until"].(time.Time)
		if !ok {
			return false, 0, errors.New("invalid format for blocked_until")
		}

		minutesLeft := math.Ceil(time.Until(blockedUntil).Minutes())
		if minutesLeft < 0 {
			minutesLeft = 0
		}

		return true, int64(minutesLeft), nil
	}

	return false, 0, nil
}
