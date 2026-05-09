package service

import (
	"errors"
	"krampus/pkg/types"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret string
}

func NewJWTService(secret string) *JWTService {

	return &JWTService{
		secret: secret,
	}
}

type Claims struct {
	UserID    types.UserID    `json:"user_id"`
	SessionID types.SessionID `json:"session_id"`

	jwt.RegisteredClaims
}

func (s *JWTService) Validate(
	tokenString string,
) (*Claims, error) {

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {

			return []byte(s.secret), nil
		},
	)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)

	if !ok {
		return nil, errors.New("невалидные claims")
	}

	if claims.ExpiresAt == nil {
		return nil, errors.New("отсутствует expiration")
	}

	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, errors.New("токен истёк")
	}

	return claims, nil
}
