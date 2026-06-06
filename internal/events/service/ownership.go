package service

import (
	"context"
	"database/sql"
	"time"
)

type Ownership struct {
	db *sql.DB
}

func NewOwnership(
	db *sql.DB,
) *Ownership {

	return &Ownership{
		db: db,
	}
}

func (o *Ownership) AcquirePartition(
	ctx context.Context,
	partition int,
	nodeID string,
) (bool, error) {

	query := `
		INSERT INTO event_partition_leases(
			partition_id,
			node_id,
			heartbeat_at
		)
		VALUES($1,$2,NOW())
		ON CONFLICT(partition_id)
		DO NOTHING
	`

	res, err := o.db.ExecContext(
		ctx,
		query,
		partition,
		nodeID,
	)

	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

func (o *Ownership) RenewLease(
	ctx context.Context,
	partition int,
	nodeID string,
) error {

	query := `
		UPDATE event_partition_leases
		SET heartbeat_at = NOW()
		WHERE partition_id = $1
		AND node_id = $2
	`

	_, err := o.db.ExecContext(
		ctx,
		query,
		partition,
		nodeID,
	)

	return err
}

func (o *Ownership) ReleasePartition(
	ctx context.Context,
	partition int,
	nodeID string,
) error {

	query := `
		DELETE
		FROM event_partition_leases
		WHERE partition_id = $1
		AND node_id = $2
	`

	_, err := o.db.ExecContext(
		ctx,
		query,
		partition,
		nodeID,
	)

	return err
}

func (o *Ownership) LeaseExpired(
	heartbeat time.Time,
) bool {

	return time.Since(
		heartbeat,
	) > 30*time.Second
}
