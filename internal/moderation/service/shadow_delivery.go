package service

import "context"

type ShadowChecker interface {
	IsShadowBanned(
		ctx context.Context,
		userID string,
	) (bool, error)
}

type DeliverySuppressor struct {
	checker ShadowChecker
}

func NewDeliverySuppressor(
	checker ShadowChecker,
) *DeliverySuppressor {

	return &DeliverySuppressor{
		checker: checker,
	}
}

func (d *DeliverySuppressor) AllowBroadcast(
	ctx context.Context,
	userID string,
) bool {

	banned, err := d.checker.IsShadowBanned(
		ctx,
		userID,
	)

	if err != nil {
		return false
	}

	return !banned
}
