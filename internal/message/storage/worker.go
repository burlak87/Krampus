package storage

import (
	"context"
	"log"
	"time"
)

type Worker struct {
	projection *Projection
}

func NewWorker(
	projection *Projection,
) *Worker {

	return &Worker{
		projection: projection,
	}
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

			log.Println(
				"topic unread worker tick",
			)
		}
	}
}
