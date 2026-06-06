package service

import (
	"context"
	"database/sql"
)

const ReplayBatchSize = 1000

type ReplayService struct {
	db *sql.DB
}

func NewReplayService(
	db *sql.DB,
) *ReplayService {

	return &ReplayService{
		db: db,
	}
}

func (s *ReplayService) ReplayMissedEvents(
	ctx context.Context,
	userID string,
	deviceID string,
	lastAck int64,
) (*sql.Rows, error) {

	query := `
		SELECT *
		FROM sync_events
		WHERE sequence > $1
		AND user_id = $2
		ORDER BY sequence ASC
		LIMIT $3
	`

	return s.db.QueryContext(
		ctx,
		query,
		lastAck,
		userID,
		ReplayBatchSize,
	)
}
