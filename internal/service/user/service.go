package service

import (
	"errors"
	"fmt"
	"krampus/internal/domain"
	"krampus/internal/service/refreshToken"
	"krampus/internal/storage/redis"
	"math"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserStorage interface {
	InsertUser(user domain.User) (int64, error)
	SelectUserByEmail(email string) (domain.User, error)
	SelectUserByID(userID int64) (domain.User, error)
	BlockUser(email, blockedUntil string) error
	RedisSessionStorage() redis.SessionStorage
}

type LoginAttemptStorage interface {
	LogAttempt(email string, result bool, attemptTime time.Time) error
	GetFailedLogAttempts(email string, windowStart time.Time) (int64, error)
	UserBlocked(email string, windowStart time.Time) ([]map[string]interface{}, error)
}

type User struct {
	userStorage         UserStorage
	loginAttemptStorage LoginAttemptStorage
	refreshTokenService *refreshToken.RefreshToken
	redisStorage        *redis.SessionStorage
	jwtSecret           string
}

func NewUser(
	user UserStorage,
	loginAttempt LoginAttemptStorage,
	refreshToken *refreshToken.RefreshToken,
	redisStorage *redis.SessionStorage,
	jwt string,
) *User {
	return &User{
		userStorage:         user,
		loginAttemptStorage: loginAttempt,
		refreshTokenService: refreshToken,
		redisStorage:        redisStorage,
		jwtSecret:           jwt,
	}
}

func (s *User) UserRegister(user domain.User) (domain.User, error) {
	fmt.Printf("DEBUG SERVICE REGISTER: Starting registration for: %s\n", user.Email)

	if user.Username == "" || user.Firstname == "" || user.Lastname == "" || user.Email == "" {
		return domain.User{}, errors.New("Invalid input: all fields are required")
	}

	if user.Password == "" || len(user.Password) < 8 {
		return domain.User{}, errors.New("Invalid password input: password must br at least 8 characters")
	}

	hasLetters, _ := regexp.MatchString(`[a-zA-Zа-яА-Я]`, user.Password)
	hasDigits, _ := regexp.MatchString(`[0-9]`, user.Password)
	hasSpecial, _ := regexp.MatchString(`[^a-zA-Zа-яА-Я0-9\s]`, user.Password)

	if !hasLetters || !hasDigits || !hasSpecial {
		return domain.User{}, errors.New("Invalid password input: password must contain letters, digits and special characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, errors.New("Error hashing password")
	}

	userToSave := domain.User{
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: string(hash),
		TwoFAEnabled: user.TwoFAEnabled,
	}

	fmt.Printf("DEBUG SERVICE REGISTER: Calling storage.InsertUser\n")
	id, err := s.userStorage.InsertUser(userToSave)
	if err != nil {
		fmt.Printf("DEBUG SERVICE REGISTER: Storage error: %v\n", err)
		return domain.User{}, err
	}

	createdUser := domain.User{
		ID:           id,
		Username:     user.Username,
		Firstname:    user.Firstname,
		Lastname:     user.Lastname,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		TwoFAEnabled: user.TwoFAEnabled,
		CreatedAt:    time.Now(),
	}

	fmt.Printf("DEBUG SERVICE REGISTER: SUCCESS - Created student with ID: %d\n", id)
	return createdUser, nil
}

func (s *User) UserLogin(user domain.User) (domain.TokenResponse, domain.TwoFaCodes, error) {
	fmt.Printf("DEBUG LOGIN: Attempting login for email: '%s'\n", user.Email)
	fmt.Printf("DEBUG LOGIN: Password provided: '%s'\n", user.Password)
	fmt.Printf("DEBUG LOGIN: TwoFA enabled: '%v'\n", user.TwoFAEnabled)

	if user.Email == "" || user.Password == "" {
		fmt.Printf("DEBUG LOGIN: Email or password empty\n")
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("email and password are required")
	}

	blocked, minutesLeft, err := s.IsUserBlocked(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error checking block status: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	if blocked {
		fmt.Printf("DEBUG LOGIN: User is blocked for %d minutes\n", minutesLeft)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, fmt.Errorf("your account is blocked for %d minutes", minutesLeft)
	}

	fmt.Printf("DEBUG LOGIN: Searching user in database...\n")
	dbUser, err := s.userStorage.SelectUserByEmail(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Database error or user not found: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
	fmt.Printf("DEBUG LOGIN: Stored password hash: %s\n", dbUser.PasswordHash)
	fmt.Printf("DEBUG LOGIN: Provided password: %s\n", user.Password)

	fmt.Printf("DEBUG LOGIN: Comparing passwords...\n")
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(user.Password))
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Password comparison failed: %v\n", err)
		s.LogLoginAttempt(user.Email, false)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGIN: Password correct!\n")

	attempts, err := s.GetFailedAttempts(user.Email)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error getting failed attempts: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	maxAttempts := int64(5)
	if attempts >= maxAttempts {
		fmt.Printf("DEBUG LOGIN: Too many failed attempts: %d\n", attempts)
		s.BlockUser(user.Email)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, errors.New("too many failed attempts, account blocked")
	}

	// if dbUser.TwoFAEnabled {
	if dbUser.TwoFAEnabled != false {
		tempToken, err := s.GenerateTempToken(dbUser.ID)
		if err != nil {
			fmt.Printf("DEBUG LOGIN: Error generating temp token: %v\n", err)
			return domain.TokenResponse{}, domain.TwoFaCodes{}, err
		}
		return domain.TokenResponse{}, domain.TwoFaCodes{RequiresTwoFa: true, TempToken: tempToken}, nil
	}

	accessToken, err := s.GenerateAccessToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating access token: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(dbUser.ID)
	if err != nil {
		fmt.Printf("DEBUG LOGIN: Error generating refresh token: %v\n", err)
		return domain.TokenResponse{}, domain.TwoFaCodes{}, err
	}

	session := domain.CachedSession{
		UserID:       dbUser.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	if err = s.redisStorage.SetAccessToken(accessToken, session); err != nil {
		fmt.Printf("Cache set error: %v\n", err)
	}
	if err = s.redisStorage.SetSessionByUserID(dbUser.ID, session); err != nil {
		fmt.Printf("Session cache error: %v\n", err)
	}

	s.LogLoginAttempt(user.Email, true)
	fmt.Printf("DEBUG LOGIN: Login successful for user ID: %d\n", dbUser.ID)
	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, domain.TwoFaCodes{}, nil
}

func (s *User) UserLogout(userID int64, password string) error {
	fmt.Printf("DEBUG LOGOUT: Searching user in database...\n")
	dbUser, err := s.userStorage.SelectUserByID(userID)
	if err != nil {
		fmt.Printf("DEBUG LOGOUT: Database error or user not found: %v\n", err)
		s.LogLoginAttempt(dbUser.Email, false)
		return errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGOUT: User found - ID: %d, Email: %s\n", dbUser.ID, dbUser.Email)
	fmt.Printf("DEBUG LOGOUT: Stored password hash: %s\n", dbUser.PasswordHash)
	fmt.Printf("DEBUG LOGOUT: Provided password: %s\n", dbUser.Password)

	fmt.Printf("DEBUG LOGOUT: Comparing passwords...\n")
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(dbUser.Password))
	if err != nil {
		fmt.Printf("DEBUG LOGOUT: Password comparison failed: %v\n", err)
		s.LogLoginAttempt(dbUser.Email, false)
		return errors.New("invalid credentials")
	}

	fmt.Printf("DEBUG LOGOUT: Password correct!\n")

	if err = s.redisStorage.DeleteSessionByUserID(userID); err != nil {
		fmt.Printf("Redis session delete error: %v\n", err)
	}

	err = s.refreshTokenService.DeleteRefreshTokensByUserID(userID)
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG LOGOUT: User %d successful logged out\n", userID)
	return nil
}

func (s *User) BlockUser(email string) {
	now := time.Now()
	blockedUntil := now.Add(1 * time.Minute).Format(time.RFC3339)

	s.LogLoginAttempt(email, false)

	err := s.userStorage.BlockUser(email, blockedUntil)
	if err != nil {
		fmt.Printf("Ошибка блокировки: %v\n", err)
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

func (s *User) LogLoginAttempt(email string, result bool) {
	attemptTime := time.Now().UTC()

	err := s.loginAttemptStorage.LogAttempt(email, result, attemptTime)
	if err != nil {
		fmt.Printf("Ошибка логирования: %v\n", err)
	}
}

func (s *User) GetFailedAttempts(email string) (int64, error) {
	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Minute)

	count, err := s.loginAttemptStorage.GetFailedLogAttempts(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка подсчета попыток: %v\n", err)
		return int64(0), err
	}

	return int64(count), err
}

func (s *User) IsUserBlocked(email string) (bool, int64, error) {
	now := time.Now().UTC()
	windowStart := now

	result, err := s.loginAttemptStorage.UserBlocked(email, windowStart)
	if err != nil {
		fmt.Printf("Ошибка проверки блокировки: %v\n", err)
		return false, 0, err
	}

	if len(result) > 0 {
		blockedUntilStr, ok := result[0]["blocked_until"].(string)
		if !ok {
			return false, 0, errors.New("invalid format for blocked_until")
		}

		blockedUntil, err := time.Parse(time.RFC3339, blockedUntilStr)
		if err != nil {
			return false, 0, err
		}

		minutesLeft := math.Ceil(time.Until(blockedUntil).Minutes())
		if minutesLeft < 0 {
			minutesLeft = 0
		}

		return true, int64(minutesLeft), nil
	}

	return false, 0, nil
}
