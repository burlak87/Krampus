package twofa

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"krampus/internal/domain"
	"krampus/internal/service/refreshToken"
	"krampus/internal/storage/redis"

	"github.com/golang-jwt/jwt/v5"
)

type TwoFAStorage interface {
	RenovationTwoFAStatus(userID int64, enabled bool) error
	InsertTwoFaCode(userID int64, code string, expiresAt time.Time) error
	SelectTwoFaCodeByUserID(userID int64) (domain.TwoFaCode, error)
	RenovationTwoFaCodeAttempts(codeID int64, attempts int64) error
	MarkTwoFaCodeUsed(codeID int64) error
	SelectRecentCodeRequests(userID int64, since time.Time) (int64, error)
	SelectRecentVerificationAttempts(userID int64, since time.Time) (int64, error)
}

type TwoFA struct {
	twoFAStorage        TwoFAStorage
	refreshTokenService *refreshToken.RefreshToken
	redisStorage        *redis.SessionStorage
	jwtSecret           string
}

func NewTwoFA(twoFA TwoFAStorage, refreshToken *refreshToken.RefreshToken, redisStorage *redis.SessionStorage, jwt string) *TwoFA {
	return &TwoFA{
		twoFAStorage:        twoFA,
		refreshTokenService: refreshToken,
		redisStorage:        redisStorage,
		jwtSecret:           jwt,
	}
}

func (s *TwoFA) EnableTwoFA(userID int64) error {
	return s.twoFAStorage.RenovationTwoFAStatus(userID, true)
}

func (s *TwoFA) UsersSendEmailCode(tempToken string) error {
	var userID int64

	cachedTemp, err := s.redisStorage.GetTempToken(tempToken)
	if err == nil && time.Now().Before(cachedTemp.ExpiresAt) {
		userID = cachedTemp.UserID
	} else {
		userID, err = s.extractUserIDFromToken(tempToken)
		if err != nil {
			return errors.New("Invalid temp token")
		}

		tempTokenData := domain.CachedTempToken{
			UserID:    userID,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		s.redisStorage.SetTempToken(tempToken, tempTokenData)
	}

	fifteenMinutesAgo := time.Now().Add(-15 * time.Minute)
	recentRequests, err := s.twoFAStorage.SelectRecentCodeRequests(userID, fifteenMinutesAgo)
	if err != nil {
		return err
	}

	if recentRequests >= 3 {
		return errors.New("too many code requests, please try again later")
	}

	code, err := s.generateSixDigitCode()
	if err != nil {
		return errors.New("failed to generate code")
	}

	expiresAt := time.Now().Add(5 * time.Minute)
	err = s.twoFAStorage.InsertTwoFaCode(userID, code, expiresAt)
	if err != nil {
		return err
	}

	err = s.sendEmail(userID, code)
	if err != nil {
		return err
	}

	return nil
}

func (s *TwoFA) VerifyCode(code domain.Code) (domain.TokenResponse, error) {
	userID, err := s.extractUserIDFromToken(code.TempToken)
	if err != nil {
		return domain.TokenResponse{}, errors.New("invalid temp token")
	}

	tenMinuteAgo := time.Now().Add(-10 * time.Minute)
	recentAttempts, err := s.twoFAStorage.SelectRecentVerificationAttempts(userID, tenMinuteAgo)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	if recentAttempts >= 5 {
		return domain.TokenResponse{}, errors.New("too many verification attempts, please try again later")
	}

	twoFaCode, err := s.twoFAStorage.SelectTwoFaCodeByUserID(userID)
	if err != nil {
		return domain.TokenResponse{}, errors.New("invalid temp token or code not found")
	}

	if twoFaCode.IsUsed {
		return domain.TokenResponse{}, errors.New("code already used")
	}

	if twoFaCode.Attempts >= 3 {
		return domain.TokenResponse{}, errors.New("too many attempts")
	}

	if time.Now().After(twoFaCode.ExpiresAt) {
		return domain.TokenResponse{}, errors.New("code expires")
	}

	if twoFaCode.Code != code.Code {
		err = s.twoFAStorage.RenovationTwoFaCodeAttempts(twoFaCode.ID, int64(twoFaCode.Attempts+1))
		if err != nil {
			return domain.TokenResponse{}, err
		}

		remainingAttempts := 3 - (twoFaCode.Attempts + 1)
		return domain.TokenResponse{}, fmt.Errorf("invalid code, %d attempts remaining", remainingAttempts)
	}

	err = s.twoFAStorage.MarkTwoFaCodeUsed(twoFaCode.ID)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	accessToken, err := s.GenerateAccessToken(twoFaCode.UserID)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	refreshToken, err := s.refreshTokenService.GenerateRefreshToken(twoFaCode.UserID)
	if err != nil {
		return domain.TokenResponse{}, err
	}

	return domain.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *TwoFA) generateSixDigitCode() (string, error) {
	max := big.NewInt(899999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func (s *TwoFA) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *TwoFA) extractUserIDFromToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("Invalid token claims")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("Invalid user_id in token")
	}

	return int64(userIDFloat), nil
}

func (s *TwoFA) sendEmail(userID int64, code string) error {
	fmt.Printf("Sending email to user %d: Your code: %s (valid for 5 minutes)\n", userID, code)
	return nil
}
