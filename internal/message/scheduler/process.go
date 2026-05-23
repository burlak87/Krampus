package scheduler

import "context"

func (s *Scheduler) processScheduledMessages(
	ctx context.Context,
) error {

	messages, err := s.repo.LockPendingMessages(
		ctx,
		100,
	)

	if err != nil {
		return err
	}

	for _, msg := range messages {

		err := s.publisher.PublishMessage(
			ctx,
			msg.MessageID,
		)

		if err != nil {
			continue
		}
	}

	return nil
}
