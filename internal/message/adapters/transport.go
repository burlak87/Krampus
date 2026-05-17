package adapters

import (
	message "krampus/internal/message/domain"
	"krampus/pkg/types"
)

type TransportClient interface {
	ConnID() string

	SafeSend(
		event *message.Event,
	) error

	GetUserID() types.UserID

	GetRoomID() types.RoomID

	Transport() TransportKind
}
