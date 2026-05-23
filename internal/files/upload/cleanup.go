package upload

import (
	"context"
	"database/sql"
	"time"
)

type CleanupWorker struct {
	db *sql.DB
}

func NewCleanupWorker(
	db *sql.DB,
) *CleanupWorker {

	return &CleanupWorker{
		db: db,
	}
}

func (w *CleanupWorker) Start(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		1 * time.Hour,
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
				DELETE FROM upload_sessions
				WHERE expires_at < NOW()
				`,
			)
		}
	}
}
