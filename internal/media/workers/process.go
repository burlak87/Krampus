package workers

import (
	"context"
	"log"
	"time"

	"krampus/internal/media/domain"
)

type Repository interface {
	LockPendingJobs(
		ctx context.Context,
		limit int,
	) ([]domain.MediaJob, error)

	CompleteJob(
		ctx context.Context,
		jobID int64,
	) error

	FailJob(
		ctx context.Context,
		jobID int64,
		err string,
	) error
}

type Processor interface {
	ProcessMedia(
		ctx context.Context,
		mediaFileID string,
	) error
}

type Worker struct {
	repo Repository

	processor Processor
}

func (w *Worker) Start(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		5 * time.Second,
	)

	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			jobs, err := w.repo.LockPendingJobs(
				ctx,
				10,
			)

			if err != nil {
				log.Println(err)
				continue
			}

			for _, job := range jobs {

				err := w.processor.ProcessMedia(
					ctx,
					job.MediaFileID,
				)

				if err != nil {

					_ = w.repo.FailJob(
						ctx,
						job.ID,
						err.Error(),
					)

					continue
				}

				_ = w.repo.CompleteJob(
					ctx,
					job.ID,
				)
			}
		}
	}
}
