package events

import (
	"context"
	"database/sql"
)

type CheckpointRepository struct {
	db *sql.DB
}

func NewCheckpointRepository(
	db *sql.DB,
) *CheckpointRepository {

	return &CheckpointRepository{
		db: db,
	}
}

func (r *CheckpointRepository) Save(
	ctx context.Context,
	consumer string,
	sequence int64,
) error {

	query := `
		INSERT INTO projection_checkpoints(
			consumer_name,
			last_sequence
		)
		VALUES($1,$2)

		ON CONFLICT(consumer_name)

		DO UPDATE SET
			last_sequence = EXCLUDED.last_sequence,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		consumer,
		sequence,
	)

	return err
}

func (r *CheckpointRepository) Get(
	ctx context.Context,
	consumer string,
) (int64, error) {

	query := `
		SELECT last_sequence
		FROM projection_checkpoints
		WHERE consumer_name = $1
	`

	row := r.db.QueryRowContext(
		ctx,
		query,
		consumer,
	)

	var sequence int64

	err := row.Scan(
		&sequence,
	)

	if err == sql.ErrNoRows {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return sequence, nil
}
