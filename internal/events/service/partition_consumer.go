package service

import (
	"context"
	"sync"
)

type PartitionConsumer struct {
	partitions int

	handler func(
		context.Context,
		int,
	)
}

func NewPartitionConsumer(
	partitions int,
	handler func(
		context.Context,
		int,
	),
) *PartitionConsumer {

	return &PartitionConsumer{
		partitions: partitions,
		handler:    handler,
	}
}

func (c *PartitionConsumer) Start(
	ctx context.Context,
) {

	var wg sync.WaitGroup

	for i := 0; i < c.partitions; i++ {

		wg.Add(1)

		go func(
			partition int,
		) {

			defer wg.Done()

			c.handler(
				ctx,
				partition,
			)

		}(i)
	}

	wg.Wait()
}
