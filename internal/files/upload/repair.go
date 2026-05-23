package upload

import (
	"context"
	"log"
	"time"
)

type RepairRepository interface {
	GetBrokenChunks(
		ctx context.Context,
	) ([]Chunk, error)
}

type RepairWorker struct {
	repo RepairRepository

	verifier *ResumeVerifier
}

func NewRepairWorker(
	repo RepairRepository,
	verifier *ResumeVerifier,
) *RepairWorker {

	return &RepairWorker{
		repo:     repo,
		verifier: verifier,
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

			chunks, err := w.repo.GetBrokenChunks(
				ctx,
			)

			if err != nil {
				log.Println(err)
				continue
			}

			for _, chunk := range chunks {

				ok := w.verifier.VerifyChunkExists(
					ctx,
					chunk,
				)

				if !ok {

					log.Println(
						"broken chunk",
						chunk.SessionID,
						chunk.ChunkIndex,
					)
				}
			}
		}
	}
}
