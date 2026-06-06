package service

import (
	"context"
	"errors"
	"time"

	"krampus/internal/user/domain"

	"github.com/golang-jwt/jwt/v5"
)

type RefreshTokenStorage interface {
	RefreshStore(ctx context.Context, userID int64, token string, expiresAt time.Time) error
	RefreshGet(ctx context.Context, token string) (int64, error)
	RefreshDelete(ctx context.Context, token string) error
	RefreshDeleteByUserID(ctx context.Context, userID int64) error
}

type RefreshToken struct {
	refreshTokenStorage RefreshTokenStorage
	jwtSecret           string
}

func NewRefreshToken(refreshToken RefreshTokenStorage, jwt string) *RefreshToken {
	return &RefreshToken{
		refreshTokenStorage: refreshToken,
		jwtSecret:           jwt,
	}
}

func (s *RefreshToken) GenerateRefreshToken(ctx context.Context, id int64) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = id
	claims["exp"] = time.Now().Add(7 * 24 * time.Hour).Unix()

	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	err = s.refreshTokenStorage.RefreshStore(ctx, id, signed, expiresAt)
	return signed, err
}

func (s *RefreshToken) UserRefresh(ctx context.Context, refreshToken string) (domain.TokenResponse, error) {
	userID, err := s.refreshTokenStorage.RefreshGet(ctx, refreshToken)
	if err != nil {
		return domain.TokenResponse{}, errors.New("Invalid refresh token")
	}

	accessToken, err := s.GenerateAccessToken(userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	newRefreshToken, err := s.GenerateRefreshToken(ctx, userID)
	if err != nil {
		return domain.TokenResponse{}, err
	}
	s.refreshTokenStorage.RefreshDelete(ctx, refreshToken)

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

func (s *RefreshToken) DeleteRefreshTokensByUserID(ctx context.Context, userID int64) error {
	return s.refreshTokenStorage.RefreshDeleteByUserID(ctx, userID)
}
