package room

import (
	"context"
	"krampus/internal/domain"
	"krampus/internal/storage"
	"time"
)

type RoomService struct {
	storage       storage.RoomStorage
	cache         storage.RoomCache
	userClientSvc *UserClientService
}

func NewRoomService(s storage.RoomStorage, cache storage.RoomCache, UserClientSvc *UserClientService) *RoomService {
	return &RoomService{
		storage:       s,
		cache:         cache,
		userClientSvc: UserClientSvc,
	}
}

func (rs *RoomService) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	if room := rs.cache.GetRoom(ctx, id); room != nil {
		return room, nil
	}

	room, err := rs.storage.GetRoom(ctx, id)
	if err != nil {
		return nil, err
	}

	go rs.cache.SetRoom(ctx, id, room)
	return room, nil
}

func (rs *RoomService) CanSendMesssage(ctx context.Context, room *domain.Room, user *domain.User, msgType domain.MessageType) bool {
	if !rs.isRoomMember(room, user.ID) {
		return msgType == domain.TypeSystem
	}

	switch room.Type {
	case domain.RoomPersonal:
		return room.OwnerID == user.ID
	case domain.RoomPrivate:
		return true
	case domain.RoomGroup:
		return true
	case domain.RoomVideoCall:
		return rs.isCallActive(room)
	}

	if room.Settings.ReadOnly && msgType != domain.TypeSystem {
		return false
	}

	if !room.Settings.AllowFiles && msgType == domain.TypeFile {
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
