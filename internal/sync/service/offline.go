package service

import (
	"context"
)

type OfflineRestorer struct {
	replay *ReplayService
}

func NewOfflineRestorer(
	replay *ReplayService,
) *OfflineRestorer {

	return &OfflineRestorer{
		replay: replay,
	}
}

func (r *OfflineRestorer) Restore(
	ctx context.Context,
	userID string,
	deviceID string,
	lastAck int64,
) error {

	rows, err := r.replay.ReplayMissedEvents(
		ctx,
		userID,
		deviceID,
		lastAck,
	)

	if err != nil {
		return err
	}

	defer rows.Close()

	return nil
}
