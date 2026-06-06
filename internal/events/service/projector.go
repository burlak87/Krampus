package service

import (
	"context"

	evdomain "krampus/internal/events/domain"
)

type Projector struct {
	projections []Projection
}

func NewProjector(projections []Projection) *Projector {
	return &Projector{projections: projections}
}

func (p *Projector) Dispatch(ctx context.Context, event evdomain.Event) error {
	for _, projection := range p.projections {
		if err := projection.Handle(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
