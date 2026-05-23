package events

import (
	"context"
	"database/sql"
)

type Failover struct {
	db *sql.DB
}

func NewFailover(
	db *sql.DB,
) *Failover {

	return &Failover{
		db: db,
	}
}

func (f *Failover) ReleaseExpiredLeases(
	ctx context.Context,
) error {

	query := `
		DELETE
		FROM event_partition_leases
		WHERE heartbeat_at <
			NOW() - INTERVAL '30 seconds'
	`

	_, err := f.db.ExecContext(
		ctx,
		query,
	)

	return err
}
