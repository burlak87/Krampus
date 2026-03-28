package refreshToken

import (
	"errors"
	"time"

	"krampus/internal/domain"

	"github.com/golang-jwt/jwt/v5"
)

type RefreshTokenStorage interface {
	RefreshStore(userID int64, token string, expiresAt time.Time) error
	RefreshGet(token string) (int64, error)
	RefreshDelete(token string) error
	RefreshDeleteByUserID(userID int64) error
}

type RefreshToken struct {
	refreshTokenStorage RefreshTokenStorage
	jwtSecret           string
}

func NewUser(refreshToken RefreshTokenStorage, jwt string) *RefreshToken {
	return &RefreshToken{
		refreshTokenStorage: refreshToken,
		jwtSecret:           jwt,
	}
}

func (s *RefreshToken) GenerateRefreshToken(id int64) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = id
	claims["exp"] = time.Now().Add(7 * 24 * time.Hour).Unix()

	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.refreshTokenStorage.RefreshStore(id, signed, expiresAt)
	return signed, err
}

func (s *RefreshToken) UserRefresh(refreshToken string) (domain.TokenResponse, error) {
	userID, err := s.refreshTokenStorage.RefreshGet(refreshToken)
	if err != nil {
		return domain.TokenResponse{}, errors.New("Invalid refresh token")
	}

	accessToken, err := s.GenerateAccessToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	newRefreshToken, err := s.GenerateRefreshToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	s.refreshTokenStorage.RefreshDelete(refreshToken)

	return domain.TokenResponse{AccessToken: accessToken, RefreshToken: newRefreshToken}, nil
}

func (s *RefreshToken) GenerateAccessToken(id int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *RefreshToken) DeleteRefreshTokensByUserID(userID int64) error {
	return s.refreshTokenStorage.RefreshDeleteByUserID(userID)
}
