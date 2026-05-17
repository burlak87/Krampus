package adapters

import (
	"context"

	message "krampus/internal/message/domain"
)

type PubSub interface {
	Publish(
		ctx context.Context,
		channel string,
		msg *message.BaseMessage,
	) error

	Subscribe(
		ctx context.Context,
		channel string,
		handler func(*message.BaseMessage),
	) error
}
