package service

import (
	"context"
	"encoding/json"
	"log"

	chat "krampus/internal/chat/service"
	message "krampus/internal/message/domain"
	messageStorage "krampus/internal/message/storage"
	"krampus/pkg/apperror"
	"krampus/pkg/types"

	"github.com/google/uuid"
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

type IdempotencyRepository interface {
	IsDuplicate(ctx context.Context, key string) (bool, error)
	Save(ctx context.Context, key string, messageID string) error
}

type FileMessageStore interface {
	SaveMessage(roomID string, msg *message.BaseMessage) error
}

type MessageService struct {
	storage         *messageStorage.MessagePGStorage
	distributor     *messageStorage.MessageDistributor
	roomSvc         *chat.RoomService
	userClientSvc   *chat.UserClientService
	rateLimiter     *RateLimiter
	outboxRepo      OutboxRepository
	idempotencyRepo IdempotencyRepository
	fileStore       FileMessageStore
}

func NewMessageService(storage *messageStorage.MessagePGStorage, dist *messageStorage.MessageDistributor, roomSvc *chat.RoomService, userClientSvc *chat.UserClientService, outboxRepo OutboxRepository, idempotencyRepo IdempotencyRepository) *MessageService {
	return &MessageService{
		storage:         storage,
		distributor:     dist,
		roomSvc:         roomSvc,
		userClientSvc:   userClientSvc,
		rateLimiter:     NewRateLimiter(),
		outboxRepo:      outboxRepo,
		idempotencyRepo: idempotencyRepo,
	}
}

func (ms *MessageService) SetFileStore(fs FileMessageStore) {
	ms.fileStore = fs
}

func (ms *MessageService) Process(ctx context.Context, msg *message.BaseMessage) error {
	if msg.Timestamp == 0 {
		msg.SetTimestamp()
	}
	if err := msg.Validate(); err != nil {
		return apperror.New(apperror.ErrInvalidMessage, err.Error())
	}

	if err := ms.rateLimiter.Check(ctx, msg.UserID.String(), msg.Type); err != nil {
		return err
	}

	user, err := ms.userClientSvc.GetUser(ctx, msg.UserID.String())
	if err != nil {
		return apperror.New(apperror.ErrUserNotFound, "user client not found")
	}

	room, err := ms.roomSvc.GetRoom(ctx, msg.RoomID.String())
	if err != nil {
		return apperror.New(apperror.ErrRoomNotFound, "room not found")
	}
	if !ms.roomSvc.CanSendMessage(ctx, room, user, msg.Type) {
		return apperror.New(apperror.ErrForbidden, "no permission to send")
	}

	idempotencyKey := msg.ID.String()

	duplicate, err := ms.idempotencyRepo.
		IsDuplicate(ctx, idempotencyKey)
	if err != nil {
		return apperror.New(apperror.ErrInternal, "idempotency check failed")
	}

	if duplicate {
		log.Printf("duplicate message skipped message_id=%s", msg.ID)
		return nil
	}

	err = ms.storage.SaveMessage(ctx, msg)

	if err != nil {
		return apperror.New(apperror.ErrStorage, "failed to save message")
	}

	if ms.fileStore != nil {
		go func() {
			_ = ms.fileStore.SaveMessage(msg.RoomID.String(), msg)
		}()
	}

	err = ms.idempotencyRepo.Save(ctx, idempotencyKey, msg.ID.String())

	if err != nil {
		return apperror.New(apperror.ErrInternal, "failed to save idempotency key")
	}

	eventPayload, err := json.Marshal(message.MessageCreatedEvent{
		MessageID: msg.ID,
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
	})

	if err != nil {
		return apperror.New(apperror.ErrInternal, "failed to marshal outbox payload")
	}

	event := &message.OutboxEvent{
		ID:            uuid.NewString(),
		AggregateType: "message",
		AggregateID:   msg.ID.String(),
		EventType:     message.OutboxEventMessageCreated,
		Payload:       eventPayload,
	}

	err = ms.outboxRepo.SaveEvent(ctx, event)

	if err != nil {
		return apperror.New(apperror.ErrInternal, "failed to save outbox event")
	}

	go func() {
		ms.userClientSvc.UpdateLastActivity(ctx, msg.UserID.String())
	}()

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
	msg.RoomID = types.RoomIDFromString(roomID)
	return ms.distributor.Broadcast(ctx, msg)
}

func (ms *MessageService) SendToUserClient(ctx context.Context, userID string, msg *message.BaseMessage) error {
	msg.UserID = types.UserIDFromString(userID)
	return ms.distributor.Broadcast(ctx, msg)
}
