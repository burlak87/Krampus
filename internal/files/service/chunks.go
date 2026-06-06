package service

import (
	"context"
	"database/sql"

	filesDomain "krampus/internal/files/domain"
)

type Chunk = filesDomain.Chunk

type ChunkRepository struct {
	db *sql.DB
}

func NewChunkRepository(
	db *sql.DB,
) *ChunkRepository {

	return &ChunkRepository{
		db: db,
	}
}

func (r *ChunkRepository) AddChunk(
	ctx context.Context,
	sessionID string,
	chunkIndex int64,
	chunkSize int64,
	checksum string,
) error {

	query := `
		INSERT INTO upload_chunks(
			session_id,
			chunk_index,
			chunk_size,
			checksum
		)
		VALUES($1,$2,$3,$4)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		sessionID,
		chunkIndex,
		chunkSize,
		checksum,
	)

	return err
}
