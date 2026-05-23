package events

import (
	"context"
	"database/sql"
)

type Rebuilder struct {
	db *sql.DB

	replay *ReplayEngine
}

func NewRebuilder(
	db *sql.DB,
	replay *ReplayEngine,
) *Rebuilder {

	return &Rebuilder{
		db:     db,
		replay: replay,
	}
}

func (r *Rebuilder) RebuildSearchProjection(
	ctx context.Context,
) error {

	_, err := r.db.ExecContext(
		ctx,
		`TRUNCATE TABLE message_search_projection`,
	)

	if err != nil {
		return err
	}

	return r.replay.Replay(
		ctx,
		0,
		1000,
	)
}
