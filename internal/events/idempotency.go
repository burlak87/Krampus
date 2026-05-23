package events

import (
	"context"
	"database/sql"
)

type IdempotencyRepository struct {
	db *sql.DB
}

func NewIdempotencyRepository(
	db *sql.DB,
) *IdempotencyRepository {

	return &IdempotencyRepository{
		db: db,
	}
}

func (r *IdempotencyRepository) IsProcessed(
	ctx context.Context,
	consumer string,
	eventID int64,
) (bool, error) {

	query := `
		SELECT EXISTS(
			SELECT 1
			FROM processed_events
			WHERE consumer_name = $1
			AND event_id = $2
		)
	`

	row := r.db.QueryRowContext(
		ctx,
		query,
		consumer,
		eventID,
	)

	var exists bool

	err := row.Scan(&exists)

	return exists, err
}

func (r *IdempotencyRepository) MarkProcessed(
	ctx context.Context,
	consumer string,
	eventID int64,
) error {

	query := `
		INSERT INTO processed_events(
			consumer_name,
			event_id
		)
		VALUES($1,$2)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		consumer,
		eventID,
	)

	return err
}
