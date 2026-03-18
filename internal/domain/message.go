package domain

import "encoding/json"

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
	
}

type CommandPayload struct {
	
}

type SystemPayload struct {
	
}

type Metadata struct {
	
}