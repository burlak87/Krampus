package service

import (
	"context"
	chat "krampus/internal/chat/service"
	message "krampus/internal/message/domain"
	messageStorage "krampus/internal/message/storage"
	"krampus/pkg/apperror"
	"log"
	"time"
)

type MessageStorage interface {
	SaveMessage(ctx context.Context, msg *message.BaseMessage) error
	SaveMessageBatch(ctx context.Context, msgs []*message.BaseMessage) error
	GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*message.BaseMessage, error)
	GetMessage(ctx context.Context, id string) (*message.BaseMessage, error)
}

type MessageDistributor interface {
	Broadcast(ctx context.Context, msg *message.BaseMessage) error
	BroadcastToRoom(ctx context.Context, msg *message.BaseMessage, roomID string) error
	SendToUserClient(ctx context.Context, userID string, msg *message.BaseMessage) error
}

type MessageService struct {
	storage       *messageStorage.MessagePGStorage
	distributor   *messageStorage.MessageDistributor
	roomSvc       *chat.RoomService
	userClientSvc *chat.UserClientService
	rateLimiter   *RateLimiter
}

func NewMessageService(storage *messageStorage.MessagePGStorage, dist *messageStorage.MessageDistributor, roomSvc *chat.RoomService, userClientSvc *chat.UserClientService) *MessageService {
	return &MessageService{
		storage:       storage,
		distributor:   dist,
		roomSvc:       roomSvc,
		userClientSvc: userClientSvc,
		rateLimiter:   NewRateLimiter(),
	}
}

func (ms *MessageService) Process(ctx context.Context, msg *message.BaseMessage) error {
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
	if !ms.roomSvc.CanSendMessage(ctx, room, user, msg.Type) {
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

func (ms *MessageService) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*message.BaseMessage, error) {
	if _, err := ms.roomSvc.GetRoom(ctx, roomID); err != nil {
		return nil, apperror.New(apperror.ErrRoomNotFound, "room not found")
	}

	return ms.storage.GetRoomMessages(ctx, roomID, limit)
}

func (ms *MessageService) GetMessage(ctx context.Context, id string) (*message.BaseMessage, error) {
	msg, err := ms.storage.GetMessage(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrInternal, "failed to get message")
	}
	return msg, nil
}

func (ms *MessageService) SaveMessageBatch(ctx context.Context, msgs []*message.BaseMessage) error {
	return ms.storage.SaveMessageBatch(ctx, msgs)
}

func (ms *MessageService) BroadcastToRoom(ctx context.Context, msg *message.BaseMessage, roomID string) error {
	msg.RoomID = roomID
	return ms.distributor.Broadcast(ctx, msg)
}

func (ms *MessageService) SendToUserClient(ctx context.Context, userID string, msg *message.BaseMessage) error {
	msg.UserID = userID
	return ms.distributor.Broadcast(ctx, msg)
}
