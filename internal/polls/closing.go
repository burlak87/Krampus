package polls

import (
	"context"
	"database/sql"
	"time"
)

type ClosingWorker struct {
	db *sql.DB
}

func NewClosingWorker(
	db *sql.DB,
) *ClosingWorker {

	return &ClosingWorker{
		db: db,
	}
}

func (w *ClosingWorker) Start(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		1 * time.Minute,
	)

	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			_, _ = w.db.ExecContext(
				ctx,
				`
				UPDATE polls
				SET closed = TRUE
				WHERE closed = FALSE
				AND closes_at <= NOW()
				`,
			)
		}
	}
}
