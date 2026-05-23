package moderation

import (
	"context"
	"encoding/json"

	"krampus/internal/events"
)

type Projection struct{}

func (p *Projection) Name() string {
	return "moderation_projection"
}

type ModerationPayload struct {
	TargetUserID string `json:"target_user_id"`

	ActionType string `json:"action_type"`
}

func (p *Projection) Handle(
	ctx context.Context,
	event events.Event,
) error {

	if event.EventType != "moderation_action_created" {
		return nil
	}

	var payload ModerationPayload

	err := json.Unmarshal(
		event.Payload,
		&payload,
	)

	if err != nil {
		return err
	}

	_ = payload

	return nil
}
