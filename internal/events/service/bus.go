package service

import (
	"context"

	evdomain "krampus/internal/events/domain"
)

type Handler interface {
	Handle(
		ctx context.Context,
		event evdomain.Event,
	) error
}

type Bus struct {
	handlers map[string][]Handler
}

func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]Handler),
	}
}

func (b *Bus) Subscribe(eventType string, handler Handler) {
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *Bus) Publish(ctx context.Context, event evdomain.Event) error {
	handlers := b.handlers[event.EventType]

	for _, handler := range handlers {
		if err := handler.Handle(ctx, event); err != nil {
			return err
		}
	}

	return nil
}
