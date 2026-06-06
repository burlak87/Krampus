package workers

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

type RepairWorker struct {
	repairer Repairer
}

func NewRepairWorker(
	repairer Repairer,
) *RepairWorker {

	return &RepairWorker{
		repairer: repairer,
	}
}

func (w *RepairWorker) Start(
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
