package service

import (
	"context"
	"encoding/json"
	message "krampus/internal/message/domain"
	"log"
	"time"
)

type OutboxPublisher interface {
	Publish(ctx context.Context, msg *message.BaseMessage) error
}

type MessageRepository interface {
	GetMessage(ctx context.Context, id string) (*message.BaseMessage, error)
}

type OutboxRepository interface {
	SaveEvent(
		ctx context.Context,
		event *message.OutboxEvent,
	) error

	GetUnpublishedEvents(
		ctx context.Context,
		limit int,
	) ([]*message.OutboxEvent, error)

	MarkPublished(
		ctx context.Context,
		eventID string,
	) error

	IncrementRetry(
		ctx context.Context,
		eventID string,
		errMsg string,
	) error
}

type OutboxWorker struct {
	repo        OutboxRepository
	messageRepo MessageRepository
	publisher   OutboxPublisher
}

func NewOutboxWorker(
	repo OutboxRepository,
	msgRepo MessageRepository,
	publisher OutboxPublisher,
) *OutboxWorker {

	return &OutboxWorker{
		repo:        repo,
		messageRepo: msgRepo,
		publisher:   publisher,
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *OutboxWorker) process(ctx context.Context) {
	events, err := w.repo.GetUnpublishedEvents(ctx, 100)
	if err != nil {
		log.Printf("outbox load failed: %v", err)
		return
	}

	for _, event := range events {
		var eventPayload message.MessageCreatedEvent
		err := json.Unmarshal(event.Payload, &eventPayload)
		if err != nil {
			log.Printf("outbox payload decode failed event_id=%s err=%v", event.ID, err)
			_ = w.repo.IncrementRetry(ctx, event.ID, err.Error())
			continue
		}

		msg, err := w.messageRepo.GetMessage(ctx, eventPayload.MessageID.String())
		if err != nil {
			log.Printf("message load failed message_id=%s err=%v", eventPayload.MessageID, err)
			_ = w.repo.IncrementRetry(ctx, event.ID, err.Error())
			continue
		}

		err = w.publisher.Publish(ctx, msg)
		if err != nil {
			log.Printf("outbox publish failed event_id=%s message_id=%s err=%v", event.ID, msg.ID, err)
			_ = w.repo.IncrementRetry(ctx, event.ID, err.Error())
			continue
		}

		err = w.repo.MarkPublished(ctx, event.ID)
		if err != nil {
			log.Printf("outbox mark published failed event_id=%s err=%v", event.ID, err)
			continue
		}
	}
}
