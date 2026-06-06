package workers

import (
	"context"
	"log"
	"time"
)

type Rebuilder interface {
	ReindexAll(
		ctx context.Context,
	) error
}

type RebuildWorker struct {
	rebuilder Rebuilder
}

func NewRebuildWorker(
	rebuilder Rebuilder,
) *RebuildWorker {

	return &RebuildWorker{
		rebuilder: rebuilder,
	}
}

func (w *RebuildWorker) Start(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		24 * time.Hour,
	)

	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			err := w.rebuilder.ReindexAll(
				ctx,
			)

			if err != nil {
				log.Println(err)
			}
		}
	}
}
