package events

import (
	"context"
	"database/sql"
)

type SnapshotRepository struct {
	db *sql.DB
}

func NewSnapshotRepository(
	db *sql.DB,
) *SnapshotRepository {

	return &SnapshotRepository{
		db: db,
	}
}

func (r *SnapshotRepository) Save(
	ctx context.Context,
	snapshot Snapshot,
) error {

	query := `
		INSERT INTO aggregate_snapshots(
			aggregate_type,
			aggregate_id,
			last_sequence,
			snapshot
		)
		VALUES($1,$2,$3,$4)

		ON CONFLICT(
			aggregate_type,
			aggregate_id
		)

		DO UPDATE SET
			last_sequence = EXCLUDED.last_sequence,
			snapshot = EXCLUDED.snapshot,
			created_at = NOW()
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		snapshot.AggregateType,
		snapshot.AggregateID,
		snapshot.LastSequence,
		snapshot.Snapshot,
	)

	return err
}

func (r *SnapshotRepository) Get(
	ctx context.Context,
	aggregateType string,
	aggregateID string,
) (*Snapshot, error) {

	query := `
		SELECT
			id,
			aggregate_type,
			aggregate_id,
			last_sequence,
			snapshot,
			created_at
		FROM aggregate_snapshots
		WHERE aggregate_type = $1
		AND aggregate_id = $2
	`

	row := r.db.QueryRowContext(
		ctx,
		query,
		aggregateType,
		aggregateID,
	)

	var s Snapshot

	err := row.Scan(
		&s.ID,
		&s.AggregateType,
		&s.AggregateID,
		&s.LastSequence,
		&s.Snapshot,
		&s.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &s, nil
}
