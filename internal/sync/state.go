package sync

import (
	"context"
	"database/sql"
)

type StateRepository struct {
	db *sql.DB
}

func NewStateRepository(
	db *sql.DB,
) *StateRepository {

	return &StateRepository{
		db: db,
	}
}

func (r *StateRepository) UpdateCursor(
	ctx context.Context,
	userID string,
	deviceID string,
	lastSequence int64,
) error {

	query := `
		INSERT INTO sync_state(
			user_id,
			device_id,
			last_event_sequence
		)
		VALUES($1,$2,$3)

		ON CONFLICT(
			user_id,
			device_id
		)

		DO UPDATE SET
			last_event_sequence =
				EXCLUDED.last_event_sequence,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		userID,
		deviceID,
		lastSequence,
	)

	return err
}
