package adapters

import (
	"encoding/json"

	message "krampus/internal/message/domain"
)

func BuildEvent(
	seq int64,
	msg *message.BaseMessage,
) (*message.Event, error) {

	payload, err := json.Marshal(msg)

	if err != nil {
		return nil, err
	}

	eventType := "message"

	switch msg.Type {

	case message.TypePresenceRealtime:
		eventType = "presence"

	case message.TypeSystem:
		eventType = "room_update"
	}

	return &message.Event{
		ID:          string(msg.ID),
		Sequence:    seq,
		Type:        eventType,
		AggregateID: string(msg.RoomID),
		UserID:      msg.UserID,
		RoomID:      msg.RoomID,
		Timestamp:   msg.Timestamp,
		Payload:     payload,
	}, nil
}
