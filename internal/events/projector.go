package events

import (
	"context"
)

type Projector struct {
	projections []Projection
}

func NewProjector(
	projections []Projection,
) *Projector {

	return &Projector{
		projections: projections,
	}
}

func (p *Projector) Dispatch(
	ctx context.Context,
	event Event,
) error {

	for _, projection := range p.projections {

		err := projection.Handle(
			ctx,
			event,
		)

		if err != nil {
			return err
		}
	}

	return nil
}
