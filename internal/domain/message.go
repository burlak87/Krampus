package domain

import (
	"encoding/json"
	"errors"
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
)

type BaseMessage struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	UserID    string          `json:"user_id"`
	RoomID    string          `json:"room_id"`
	Timestamp int64           `json:"timestamp"`
	Version   int             `json:"verison"`
	Payload   json.RawMessage `json:"payload"`
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
	Version     int               `json:"version"`
	Compression string            `json:"compression,omitempty"`
	Encryption  string            `json:"encryption,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
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
	m.ID = uuid.New().String()
}

func NewTextMessage(userID, roomID, text string) *BaseMessage {
	msg := &BaseMessage{
		Type:    TypeText,
		UserID:  userID,
		RoomID:  roomID,
		Version: 1,
	}
	payload, _ := json.Marshal(TextPayload{Text: text})
	msg.Payload = payload
	msg.SetTimestamp()
	return msg
}
