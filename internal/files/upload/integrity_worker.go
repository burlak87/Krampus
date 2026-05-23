package upload

import (
	"context"
	"log"
	"time"
)

type IntegrityRepository interface {
	GetChunksForIntegrityCheck(
		ctx context.Context,
		limit int,
	) ([]Chunk, error)
}

type IntegrityWorker struct {
	repo IntegrityRepository

	verifier *IntegrityVerifier
}

func NewIntegrityWorker(
	repo IntegrityRepository,
	verifier *IntegrityVerifier,
) *IntegrityWorker {

	return &IntegrityWorker{
		repo:     repo,
		verifier: verifier,
	}
}

func (w *IntegrityWorker) Start(
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

			chunks, err := w.repo.GetChunksForIntegrityCheck(
				ctx,
				100,
			)

			if err != nil {
				log.Println(err)
				continue
			}

			for _, chunk := range chunks {

				err := w.verifier.VerifyChunk(
					ctx,
					chunk,
				)

				if err != nil {

					log.Println(
						"corrupted chunk",
						chunk.SessionID,
						chunk.ChunkIndex,
					)
				}
			}
		}
	}
}
