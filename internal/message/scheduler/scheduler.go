package scheduler

import (
	"context"
	"log"
	"time"
)

type Publisher interface {
	PublishMessage(
		ctx context.Context,
		messageID string,
	) error
}

type Scheduler struct {
	repo      *Repository
	publisher Publisher
}

func NewScheduler(
	repo *Repository,
	publisher Publisher,
) *Scheduler {

	return &Scheduler{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)

	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			err := s.processScheduledMessages(ctx)
			if err != nil {
				log.Println(err)
			}
		}
	}
}
