package domain

import (
	"encoding/json"
	"errors"
	"krampus/pkg/types"
	"time"

	"github.com/google/uuid"
)

type MessageType string

const (
	TypeText        MessageType = "text"
	TypeFile        MessageType = "file"
	TypeCommand     MessageType = "command"
	TypeSystem      MessageType = "system"
	TypeVideoCall   MessageType = "video_call"
	TypeTyping      MessageType = "typing"
	TypeReadReceipt MessageType = "read_receipt"

	TypeCursor           MessageType = "cursor"
	TypePresenceRealtime MessageType = "presence_realtime"
	TypeGame             MessageType = "game"
	TypeWebRTCSignal     MessageType = "webrtc_signal"

	TypeAck          MessageType = "ack"
	TypeAckSent      MessageType = "ack_sent"
	TypeAckDelivered MessageType = "ack_delivered"
	TypeAckRead      MessageType = "ack_read"
	TypeAckFailed    MessageType = "ack_failed"
)

type BaseMessage struct {
	ID        types.MessageID `json:"id"`
	Type      MessageType     `json:"type"`
	UserID    types.UserID    `json:"user_id"`
	RoomID    types.RoomID    `json:"room_id"`
	Timestamp int64           `json:"timestamp"`
	Version   int             `json:"version"`
	Payload   json.RawMessage `json:"payload"`
	Metadata  Metadata        `json:"metadata"`
	Signature string          `json:"signature"`
}

type TextPayload struct {
	Text      string              `json:"text"`
	Format    string              `json:"format"`
	Mentions  []string            `json:"mentions"`
	ReplyTo   string              `json:"reply_to"`
	Reactions map[string][]string `json:"reactions"`
}

type CommandPayload struct {
	Cmd     string                 `json:"cmd"`
	Args    []string               `json:"args"`
	Options map[string]interface{} `json:"options"`
}

type SystemPayload struct {
	Actions string      `json:"actions"`
	Data    interface{} `json:"data"`
}

type Metadata struct {
	TraceID       string            `json:"trace_id,omitempty"`
	RequestID     string            `json:"request_id,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
	Version       int               `json:"version"`
	Compression   string            `json:"compression,omitempty"`
	Encryption    string            `json:"encryption,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

var ErrInvalidTimestamp = errors.New("invalid timestamp")

func (m *BaseMessage) Validate() error {
	if m.ID == "" || m.UserID == "" || m.RoomID == "" {
		return errors.New("missing required fields")
	}
	if m.Timestamp <= 0 {
		return ErrInvalidTimestamp
	}
	if m.Timestamp > time.Now().Add(5*time.Minute).UnixNano() {
		return ErrInvalidTimestamp
	}
	return nil
}

func (m *BaseMessage) SetTimestamp() {
	m.Timestamp = time.Now().UnixNano()
	m.ID = types.MessageID(uuid.New().String())
}

func NewTextMessage(userID types.UserID, roomID types.RoomID, text string) *BaseMessage {
	msg := &BaseMessage{
		Type:    TypeText,
		UserID:  userID,
		RoomID:  roomID,
		Version: 1,
	}
	payload, _ := json.Marshal(TextPayload{
		Text: text,
	})
	msg.Payload = payload
	msg.SetTimestamp()
	return msg
}

func (t MessageType) IsRealtimeOnly() bool {
	switch t {
	case TypeTyping,
		TypeVideoCall,
		TypeCursor,
		TypePresenceRealtime,
		TypeGame,
		TypeWebRTCSignal:
		return true
	default:
		return false
	}
}

func (m *BaseMessage) ToEvent(sequence int64) (*Event, error) {
	payload, err := json.Marshal(m)

	if err != nil {
		return nil, err
	}
	return &Event{
		ID:          string(m.ID),
		Sequence:    sequence,
		Type:        string(m.Type),
		AggregateID: string(m.RoomID),
		UserID:      m.UserID,
		RoomID:      m.RoomID,
		Timestamp:   m.Timestamp,
		Payload:     payload,
	}, nil
}
