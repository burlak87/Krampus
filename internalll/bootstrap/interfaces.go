// wiring services (NewServices(cfg))
package bootstrap

import (
	"context"
	"krampus/internal/domain"
)

type MessageStorage interface {
	SaveMessage(ctx context.Context, msg *domain.BaseMessage) error
	SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error
	GetRoomMessage(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error)
	GetMessage(ctx context.Context, id string) (*domain.BaseMessage, error)
}

type RoomStorage interface {
	SaveRoom(ctx context.Context, room *domain.Room) error
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	UpdateRoom(ctx context.Context, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
	ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type UserClientStorage interface {
	SaveUserClient(ctx context.Context, user *domain.User) error
	GetUserClient(ctx context.Context, id string) (*domain.UserClient, error)
	UpdateUserClient(ctx context.Context, user *domain.User) error
	UpdateLastActivity(ctx context.Context, id string, ts int64) error
}

type RoomCache interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	SetRoom(ctx context.Context, id string, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
}

type UserClientCache interface {
	GetUserClient(ctx context.Context, id string) (*domain.User, error)
	SetUserClient(ctx context.Context, id string, user *domain.User) error
	DeleteUserClient(ctx context.Context, id string) error
}

type MessageDistributor interface {
	Broadcast(ctx context.Context, msg *domain.BaseMessage) error
	BroadcastToRoom(ctx context.Context, msg *domain.BaseMessage, roomID string) error
	SendToUserClient(ctx context.Context, userID string, msg *domain.BaseMessage) error
}

// // 📨 MessageStorage — сообщения (Postgres + FileStorage)
// type MessageStorage interface {
//   SaveMessage(ctx context.Context, msg *domain.BaseMessage) error
//   SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error
//   GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error)
//   GetMessage(ctx context.Context, id string) (*domain.BaseMessage, error)
// }

// // 🏠 RoomStorage — комнаты (Postgres)
// type RoomStorage interface {
//   SaveRoom(ctx context.Context, room *domain.Room) error
//   GetRoom(ctx context.Context, id string) (*domain.Room, error)
//   UpdateRoom(ctx context.Context, room *domain.Room) error
//   DeleteRoom(ctx context.Context, id string) error
// }

// // 👤 UserStorage — пользователи (Postgres)
// type UserStorage interface {
//   SaveUser(ctx context.Context, user *domain.User) error
//   GetUser(ctx context.Context, id string) (*domain.User, error)
//   UpdateUser(ctx context.Context, user *domain.User) error
//   UpdateLastActivity(ctx context.Context, id string, timestamp int64) error
// }

// // 🌐 MessageDistributor — рассылка
// type MessageDistributor interface {
//   Broadcast(ctx context.Context, msg *domain.BaseMessage) error
//   BroadcastToRoom(ctx context.Context, roomID string, msg *domain.BaseMessage) error
//   SendToUser(ctx context.Context, userID string, msg *domain.BaseMessage) error
// }

// // ⚡ RoomCache — кэш комнат (Redis TTL 10min)
// type RoomCache interface {
//   GetRoom(ctx context.Context, id string) (*domain.Room, error)
//   SetRoom(ctx context.Context, id string, room *domain.Room) error
//   DeleteRoom(ctx context.Context, id string) error
// }

// // 👥 UserCache — кэш пользователей (Redis TTL 10min)
// type UserCache interface {
//   GetUser(ctx context.Context, id string) (*domain.User, error)
//   SetUser(ctx context.Context, id string, user *domain.User) error
//   DeleteUser(ctx context.Context, id string) error
// }
