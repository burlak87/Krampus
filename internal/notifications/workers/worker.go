package workers

import (
	"context"
	"log"
	"time"
)

type Dispatcher interface {
	DispatchPending(
		ctx context.Context,
	) error
}

type Worker struct {
	dispatcher Dispatcher
}

func NewWorker(
	dispatcher Dispatcher,
) *Worker {

	return &Worker{
		dispatcher: dispatcher,
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

			err := w.dispatcher.DispatchPending(
				ctx,
			)

			if err != nil {
				log.Println(err)
			}
		}
	}
}
