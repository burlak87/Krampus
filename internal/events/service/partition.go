package service

import "hash/fnv"

type PartitionCoordinator struct {
	partitions int
}

func NewPartitionCoordinator(
	partitions int,
) *PartitionCoordinator {

	return &PartitionCoordinator{
		partitions: partitions,
	}
}

func (p *PartitionCoordinator) Partition(
	aggregateID string,
) int {

	hash := fnv.New32a()

	_, _ = hash.Write(
		[]byte(aggregateID),
	)

	return int(
		hash.Sum32(),
	) % p.partitions
}
