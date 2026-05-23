package events

import "context"

type Projection interface {
	Name() string

	Handle(
		ctx context.Context,
		event Event,
	) error
}
