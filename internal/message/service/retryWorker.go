package service

import (
	"context"
	"log"
	"math/rand"
	"time"

	"krampus/internal/message/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/types"
)

type RetryProcessor interface {
	ProcessDelivery(ctx context.Context, job *domain.DeliveryJob) error
}

type RetryRepository interface {
	Enqueue(ctx context.Context, job *domain.DeliveryJob) error
	GetReadyJobs(ctx context.Context, limit int) ([]*domain.DeliveryJob, error)
	Delete(ctx context.Context, messageID types.MessageID, userID types.UserID) error
}

type DLQRepository interface {
	Store(ctx context.Context, job *domain.DeliveryJob, reason string) error
}

type RetryWorker struct {
	repo         RetryRepository
	dlqRepo      DLQRepository
	processor    RetryProcessor
	pollInterval time.Duration
	maxAttempts  int
}

func NewRetryWorker(repo RetryRepository, dlqRepo DLQRepository, processor RetryProcessor) *RetryWorker {
	return &RetryWorker{
		repo:         repo,
		dlqRepo:      dlqRepo,
		processor:    processor,
		pollInterval: time.Second,
		maxAttempts:  5,
	}
}

func (w *RetryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *RetryWorker) processBatch(ctx context.Context) {
	jobs, err := w.repo.GetReadyJobs(ctx, 100)
	if err != nil {
		log.Printf("retry worker fetch error: %v", err)
		return
	}

	for _, job := range jobs {
		err := w.processor.ProcessDelivery(ctx, job)
		if err == nil {
			_ = w.repo.Delete(ctx, job.Message.ID, job.UserID)
			continue
		}

		if !apperror.IsRetryable(err) {
			storeErr := w.dlqRepo.Store(ctx, job, err.Error())
			if storeErr != nil {
				log.Printf("dlq store failed: %v", storeErr)
			}
			_ = w.repo.Delete(ctx, job.Message.ID, job.UserID)
			continue
		}

		job.Attempt++
		if job.Attempt >= w.maxAttempts {
			storeErr := w.dlqRepo.Store(ctx, job, "retry attempts exhausted")

			if storeErr != nil {
				log.Printf("dlq exhausted store failed: %v", storeErr)
			}
			_ = w.repo.Delete(ctx, job.Message.ID, job.UserID)
			continue
		}

		base := time.Second * time.Duration(1<<job.Attempt)
		jitter := time.Duration(rand.Int63n(int64(base / 2)))
		job.NextRetryAt = time.Now().Add(base + jitter)

		err = w.repo.Enqueue(ctx, job)
		if err != nil {
			log.Printf("retry re-enqueue failed: %v", err)
		}
	}
}
