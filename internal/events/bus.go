package events

import (
	"context"
)

type Handler interface {
	Handle(
		ctx context.Context,
		event Event,
	) error
}

type Bus struct {
	handlers map[string][]Handler
}

func NewBus() *Bus {

	return &Bus{
		handlers: make(
			map[string][]Handler,
		),
	}
}

func (b *Bus) Subscribe(
	eventType string,
	handler Handler,
) {

	b.handlers[eventType] = append(
		b.handlers[eventType],
		handler,
	)
}

func (b *Bus) Publish(
	ctx context.Context,
	event Event,
) error {

	handlers := b.handlers[event.EventType]

	for _, handler := range handlers {

		err := handler.Handle(
			ctx,
			event,
		)

		if err != nil {
			return err
		}
	}

	return nil
}
