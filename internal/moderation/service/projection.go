package service

import (
	"context"
	"encoding/json"

	evdomain "krampus/internal/events/domain"
	moderationDomain "krampus/internal/moderation/domain"
)

type Projection struct{}

func (p *Projection) Name() string { return "moderation_projection" }

func (p *Projection) Handle(ctx context.Context, event evdomain.Event) error {
	if event.EventType != "moderation_action_created" {
		return nil
	}

	var payload moderationDomain.ModerationPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	_ = payload
	return nil
}
