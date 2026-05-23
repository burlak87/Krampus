package events

import (
	"context"
	"database/sql"
)

type LeaseRepository struct {
	db *sql.DB
}

func NewLeaseRepository(
	db *sql.DB,
) *LeaseRepository {

	return &LeaseRepository{
		db: db,
	}
}

func (r *LeaseRepository) AcquireLease(
	ctx context.Context,
	partition int,
	consumerID string,
) error {

	query := `
		INSERT INTO event_partition_leases(
			partition_id,
			owner_id,
			heartbeat_at
		)
		VALUES($1,$2,NOW())

		ON CONFLICT(partition_id)

		DO UPDATE SET
			owner_id = EXCLUDED.owner_id,
			heartbeat_at = NOW()
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		partition,
		consumerID,
	)

	return err
}
