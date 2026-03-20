package message

import (
	"context"
	"krampus/internal/domain"
	"krampus/internal/storage"
	"krampus/pkg/apperror"
	"log"
	"time"
)

type MessageService struct {
	storage       storage.MessageStorage
	distributor   storage.MessageDistributor
	roomSvc       *RoomService
	userClientSvc *UserClientService
	rateLimiter   *RateLimiter
}

func NewMessageService(storage domain.MessageStorage, dist domain.MessageDistributor, roomSvc *RoomService, userClientSvc *UserClientService) *MessageService {
	return &MessageService{
		storage:       storage,
		distributor:   dist,
		roomSvc:       roomSvc,
		userClientSvc: userClientSvc,
		rateLimiter:   NewRateLimiter(),
	}
}

func (ms *MessageService) Process(ctx context.Context, msg *domain.BaseMessage) error {
	if msg.Timestamp == 0 {
		msg.SetTimestamp()
	}
	if err := msg.Validate(); err != nil {
		return apperror.New(apperror.ErrInvalidMessage, err.Error())
	}

	if err := ms.rateLimiter.Check(ctx, msg.UserID, msg.Type); err != nil {
		return err
	}

	user, err := ms.userClientSvc.GetUser(ctx, msg.UserID)
	if err != nil {
		return apperror.New(apperror.ErrUserNotFound, "user client not found")
	}

	room, err := ms.roomSvc.GetRoom(ctx, msg.RoomID)
	if err != nil {
		return apperror.New(apperror.ErrRoomNotFound, "room not found")
	}
	if err := ms.roomSvc.CanSendMessage(ctx, room, user, msg.Type); err != nil {
		return apperror.New(apperror.ErrForbidden, "no permission to send")
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ms.storage.SaveMessage(ctx, msg); err != nil {
			log.Printf("Failed to save message %s: %v", msg.ID, err)
		}
	}()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := ms.distributor.Broadcast(ctx, msg); err != nil {
			log.Printf("Failed to broadcast message %s: %v", msg.ID, err)
		}
	}()

	go ms.userClientSvc.UpdateLastActivity(ctx, msg.UserID)

	return nil
}

func (ms *MessageService) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
	if _, err := ms.roomSvc.GetRoom(ctx, roomID); err != nill {
		return nil, apperror.New(apperror.ErrRoomNotFound, "room not found")
	}

	return ms.storage.GetRoomMessages(ctx, roomID, limit)
}
