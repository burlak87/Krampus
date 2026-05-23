package service

import (
	"context"
	"database/sql"
	"log"
	"time"
)

type CleanupWorker struct {
	db *sql.DB

	retention time.Duration
}

func NewCleanupWorker(
	db *sql.DB,
	retention time.Duration,
) *CleanupWorker {

	return &CleanupWorker{
		db:        db,
		retention: retention,
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

			err := w.cleanup(ctx)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (w *CleanupWorker) cleanup(
	ctx context.Context,
) error {

	query := `
		DELETE FROM messages
		WHERE deleted_at IS NOT NULL
		AND deleted_at < NOW() - ($1 * INTERVAL '1 second')
	`

	_, err := w.db.ExecContext(
		ctx,
		query,
		int64(w.retention.Seconds()),
	)

	return err
}
