package service

import (
	"context"
	"database/sql"
)

type Rebalancer struct {
	db *sql.DB
}

func NewRebalancer(
	db *sql.DB,
) *Rebalancer {

	return &Rebalancer{
		db: db,
	}
}

func (r *Rebalancer) CountPartitions(
	ctx context.Context,
	nodeID string,
) (int, error) {

	query := `
		SELECT COUNT(*)
		FROM event_partition_leases
		WHERE node_id = $1
	`

	var count int

	err := r.db.QueryRowContext(
		ctx,
		query,
		nodeID,
	).Scan(&count)

	return count, err
}
