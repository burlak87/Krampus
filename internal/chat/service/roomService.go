package service

import (
	"context"
	"fmt"
	"krampus/internal/chat/domain"
	messageDomain "krampus/internal/message/domain"
	userDomain "krampus/internal/user/domain"
	"krampus/pkg/types"
	"time"
)

type RoomStorage interface {
	SaveRoom(ctx context.Context, room *domain.Room) error
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	UpdateRoom(ctx context.Context, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
	ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error)
}

type RoomCache interface {
	GetRoom(ctx context.Context, id string) (*domain.Room, error)
	SetRoom(ctx context.Context, id string, room *domain.Room) error
	DeleteRoom(ctx context.Context, id string) error
}

type RoomService struct {
	storage       RoomStorage
	cache         RoomCache
	userClientSvc *UserClientService
}

func NewRoomService(s RoomStorage, cache RoomCache, userClientSvc *UserClientService) *RoomService {
	return &RoomService{
		storage:       s,
		cache:         cache,
		userClientSvc: userClientSvc,
	}
}

func (rs *RoomService) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	room, err := rs.cache.GetRoom(ctx, id)
	if err == nil && room != nil {
		return room, nil
	}

	room, err = rs.storage.GetRoom(ctx, id)
	if err != nil {
		return nil, err
	}

	go rs.cache.SetRoom(ctx, id, room)
	return room, nil
}

func (rs *RoomService) IsRoomMember(ctx context.Context, roomID, userID string) (bool, error) {
	room, err := rs.GetRoom(ctx, roomID)
	if err != nil {
		return false, err
	}

	return rs.isRoomMember(room, userID), nil
}

func (rs *RoomService) CanSendMessage(ctx context.Context, room *domain.Room, user *userDomain.User, msgType messageDomain.MessageType) bool {
	userID := types.UserIDFromInt64(user.ID).String()
	if !rs.isRoomMember(room, userID) {
		return msgType == messageDomain.TypeSystem
	}

	switch room.Type {
	case domain.RoomPersonal:
		return room.OwnerID == userID
	case domain.RoomPrivate:
		return true
	case domain.RoomGroup:
		return true
	case domain.RoomVideoCall:
		return rs.isCallActive(room)
	}

	if room.Settings.ReadOnly && msgType != messageDomain.TypeSystem {
		return false
	}
	if !room.Settings.AllowFiles && msgType == messageDomain.TypeFile {
		return false
	}
	return true
}

func (rs *RoomService) isRoomMember(room *domain.Room, userID string) bool {
	for _, memberID := range room.Members {
		if memberID == userID {
			return true
		}
	}
	return false
}

// Заглушка, написать логику проверки статуса звонка
func (rs *RoomService) isCallActive(room *domain.Room) bool {
	if room.Type != domain.RoomVideoCall {
		return false
	}

	return len(room.Members) >= 2
}

func (rs *RoomService) CreateRoom(ctx context.Context, room *domain.Room) error {
	if err := rs.validateRoom(room); err != nil {
		return err
	}

	now := time.Now().UnixNano()
	room.CreatedAt = now
	room.UpdatedAt = now

	if err := rs.storage.SaveRoom(ctx, room); err != nil {
		return err
	}

	go rs.cache.SetRoom(ctx, room.ID, room)
	return nil
}

func (rs *RoomService) validateRoom(room *domain.Room) error {
	if room.ID == "" {
		return fmt.Errorf("room ID cannot be empty")
	}
	if room.OwnerID == "" {
		return fmt.Errorf("room must have an owner")
	}

	switch room.Type {
	case domain.RoomPersonal, domain.RoomPrivate, domain.RoomGroup, domain.RoomVideoCall:
	default:
		return fmt.Errorf("invalid room type: %s", room.Type)
	}

	if room.Type == domain.RoomGroup && len(room.Members) == 0 {
		room.Members = append(room.Members, room.OwnerID)
	}

	return nil
}

func (rs *RoomService) UpdateRoom(ctx context.Context, room *domain.Room) error {
	room.UpdatedAt = time.Now().UnixNano()
	if err := rs.storage.UpdateRoom(ctx, room); err != nil {
		return err
	}
	return rs.cache.SetRoom(ctx, room.ID, room)
}

func (rs *RoomService) DeleteRoom(ctx context.Context, id string) error {
	if err := rs.storage.DeleteRoom(ctx, id); err != nil {
		return err
	}
	return rs.cache.DeleteRoom(ctx, id)
}

func (rs *RoomService) ListUserRooms(ctx context.Context, userID string) ([]*domain.Room, error) {
	return rs.storage.ListUserRooms(ctx, userID)
}
