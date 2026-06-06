package service

import (
	"context"
	"encoding/json"

	evdomain "krampus/internal/events/domain"
)

type Consumer struct {
	indexer *Indexer
}

func NewConsumer(indexer *Indexer) *Consumer {
	return &Consumer{indexer: indexer}
}

type MessageCreatedPayload struct {
	MessageID string `json:"message_id"`
	RoomID    string `json:"room_id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
}

func (c *Consumer) Name() string { return "search" }

func (c *Consumer) Handle(ctx context.Context, event evdomain.Event) error {
	if event.EventType != "message_created" {
		return nil
	}

	var payload MessageCreatedPayload

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	return c.indexer.IndexMessage(ctx, payload.MessageID, payload.RoomID, payload.UserID, payload.Content)
}
