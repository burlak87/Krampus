package service

import (
	"context"
	"log"
	"time"
)

type HeartbeatWorker struct {
	ownership *Ownership

	nodeID string

	partitions []int
}

func NewHeartbeatWorker(
	ownership *Ownership,
	nodeID string,
	partitions []int,
) *HeartbeatWorker {

	return &HeartbeatWorker{
		ownership:  ownership,
		nodeID:     nodeID,
		partitions: partitions,
	}
}

func (w *HeartbeatWorker) Start(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		10 * time.Second,
	)

	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			for _, partition := range w.partitions {

				err := w.ownership.RenewLease(
					ctx,
					partition,
					w.nodeID,
				)

				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}
