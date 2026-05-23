package search

import (
	"context"
	"encoding/json"

	"krampus/internal/events"
)

type Consumer struct {
	indexer *Indexer
}

func NewConsumer(
	indexer *Indexer,
) *Consumer {

	return &Consumer{
		indexer: indexer,
	}
}

type MessageCreatedPayload struct {
	MessageID string `json:"message_id"`

	RoomID string `json:"room_id"`

	UserID string `json:"user_id"`

	Content string `json:"content"`
}

func (c *Consumer) Handle(
	ctx context.Context,
	event events.Event,
) error {

	if event.EventType != "message_created" {
		return nil
	}

	var payload MessageCreatedPayload

	err := json.Unmarshal(
		event.Payload,
		&payload,
	)

	if err != nil {
		return err
	}

	return c.indexer.IndexMessage(
		ctx,
		payload.MessageID,
		payload.RoomID,
		payload.UserID,
		payload.Content,
	)
}
