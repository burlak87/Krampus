package service

import (
	"context"
	"log"
	"time"
)

type Coordinator struct {
	ownership *Ownership

	nodeID string

	partitions int
}

func NewCoordinator(
	ownership *Ownership,
	nodeID string,
	partitions int,
) *Coordinator {

	return &Coordinator{
		ownership:  ownership,
		nodeID:     nodeID,
		partitions: partitions,
	}
}

func (c *Coordinator) Start(
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

			c.rebalance(ctx)
		}
	}
}

func (c *Coordinator) rebalance(
	ctx context.Context,
) {

	for partition := 0; partition < c.partitions; partition++ {

		ok, err := c.ownership.AcquirePartition(
			ctx,
			partition,
			c.nodeID,
		)

		if err != nil {
			log.Println(err)
			continue
		}

		if ok {

			log.Println(
				"partition acquired",
				partition,
			)
		}
	}
}
