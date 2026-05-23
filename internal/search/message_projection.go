package search

import (
	"context"
	"encoding/json"

	"krampus/internal/events"
)

type MessageProjection struct {
	indexer *Indexer
}

func NewMessageProjection(
	indexer *Indexer,
) *MessageProjection {

	return &MessageProjection{
		indexer: indexer,
	}
}

type MessagePayload struct {
	MessageID string `json:"message_id"`

	RoomID string `json:"room_id"`

	UserID string `json:"user_id"`

	Content string `json:"content"`
}

func (p *MessageProjection) Name() string {
	return "message_search_projection"
}

func (p *MessageProjection) Handle(
	ctx context.Context,
	event events.Event,
) error {

	switch event.EventType {

	case "message_created":

		var payload MessagePayload

		err := json.Unmarshal(
			event.Payload,
			&payload,
		)

		if err != nil {
			return err
		}

		return p.indexer.IndexMessage(
			ctx,
			payload.MessageID,
			payload.RoomID,
			payload.UserID,
			payload.Content,
		)

	case "message_deleted":

		return p.indexer.DeleteMessage(
			ctx,
			event.AggregateID,
		)
	}

	return nil
}
