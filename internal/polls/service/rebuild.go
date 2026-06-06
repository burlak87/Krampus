package service

import "context"

type Rebuilder struct {
	projection *Projection
}

func NewRebuilder(
	projection *Projection,
) *Rebuilder {

	return &Rebuilder{
		projection: projection,
	}
}

func (r *Rebuilder) Rebuild(
	ctx context.Context,
) error {

	return nil
}
