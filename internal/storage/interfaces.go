package storage

import (
	"context"
	"krampus/internal/domain"
)

type MessageStorage interface {
	SaveMessage(ctx context.Context, msg *domain.BaseMessage) error
	GetRoomMessage(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error)
}

type RoomStorage interface {
	SaveRoom(ctx context.Context, room *domain.Room) error
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
}

type UserClientStorage interface {
	GetUserClient(ctx context.Context, id string) (*domain.UserClient, error)
	UpdateLastActivity(ctx context.Context, id string, ts int64) error
}

type RoomCache interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	SetRoom(ctx context.Context, id string, room *domain.Room) error
}

type MessageDistributor interface {
	Broadcast(ctx context.Context, msg *domain.BaseMessage) error
	BroadcastToRoom(ctx context.Context, msg *domain.BaseMessage, roomID string) error
}
