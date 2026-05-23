package repair

import (
	"context"
	"log"
	"time"
)

type Repairer interface {
	RepairBrokenMedia(
		ctx context.Context,
	) error
}

type Worker struct {
	repairer Repairer
}

func NewWorker(
	repairer Repairer,
) *Worker {

	return &Worker{
		repairer: repairer,
	}
}

func (w *Worker) Start(
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

			err := w.repairer.RepairBrokenMedia(
				ctx,
			)

			if err != nil {
				log.Println(err)
			}
		}
	}
}
