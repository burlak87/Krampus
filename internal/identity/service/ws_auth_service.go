package service

import (
	"context"
	"errors"
	"net/http"

	identityDomain "krampus/internal/identity/domain"
	"krampus/pkg/ctxmeta"
	"krampus/pkg/types"
)

type SessionValidator interface {
	ValidateSession(
		ctx context.Context,
		sessionID types.SessionID,
		userID types.UserID,
	) error
}

type RoomAccessValidator interface {
	IsRoomMember(
		ctx context.Context,
		roomID types.RoomID,
		userID types.UserID,
	) (bool, error)
}

type WSAuthService struct {
	jwtService *JWTService
	sessions   SessionValidator
	rooms      RoomAccessValidator
}

func NewWSAuthService(
	jwtService *JWTService,
	sessions SessionValidator,
	rooms RoomAccessValidator,
) *WSAuthService {

	return &WSAuthService{
		jwtService: jwtService,
		sessions:   sessions,
		rooms:      rooms,
	}
}

func (s *WSAuthService) Authenticate(
	ctx context.Context,
	r *http.Request,
) (*identityDomain.AuthContext, error) {

	token := r.URL.Query().Get("token")

	if token == "" {
		return nil, errors.New("token missing")
	}

	roomID := types.RoomID(
		r.URL.Query().Get("room_id"),
	)

	if roomID == "" {
		return nil, errors.New("room_id missing")
	}

	claims, err := s.jwtService.Validate(token)

	if err != nil {
		return nil, err
	}

	err = s.sessions.ValidateSession(
		ctx,
		claims.SessionID,
		claims.UserID,
	)

	if err != nil {
		return nil, err
	}

	isMember, err := s.rooms.IsRoomMember(
		ctx,
		roomID,
		claims.UserID,
	)

	if err != nil {
		return nil, err
	}

	if !isMember {
		return nil, errors.New("access denied")
	}

	meta := ctxmeta.Extract(ctx)

	return &identityDomain.AuthContext{
		UserID:        claims.UserID,
		SessionID:     claims.SessionID,
		TraceID:       meta.TraceID,
		RequestID:     meta.RequestID,
		CorrelationID: meta.CorrelationID,
	}, nil
}
