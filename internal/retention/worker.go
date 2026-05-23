package retention

import (
	"context"
	"database/sql"
	"log"
	"time"
)

type Worker struct {
	db *sql.DB
}

func NewWorker(
	db *sql.DB,
) *Worker {

	return &Worker{
		db: db,
	}
}

func (w *Worker) Start(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		12 * time.Hour,
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

func (w *Worker) cleanup(
	ctx context.Context,
) error {

	query := `
		DELETE FROM media_files
		WHERE created_at < NOW() - INTERVAL '365 days'
		AND processing_status = 'deleted'
	`

	_, err := w.db.ExecContext(
		ctx,
		query,
	)

	return err
}
