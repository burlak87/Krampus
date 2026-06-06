package service

import (
	"context"

	evdomain "krampus/internal/events/domain"
)

type Projection interface {
	Name() string

	Handle(
		ctx context.Context,
		event evdomain.Event,
	) error
}
