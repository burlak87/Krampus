package service

import (
	"context"
	"database/sql"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(
	db *sql.DB,
) *Repository {

	return &Repository{
		db: db,
	}
}

func (r *Repository) AddChunk(
	ctx context.Context,
	sessionID string,
	chunkIndex int64,
	data []byte,
	checksum string,
) error {

	query := `
		INSERT INTO upload_chunks(
			session_id,
			chunk_index,
			chunk_size,
			checksum,
			data,
			verified
		)
		VALUES($1,$2,$3,$4,$5,TRUE)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		sessionID,
		chunkIndex,
		int64(len(data)),
		checksum,
		data,
	)

	return err
}

func (r *Repository) ChunkExists(
	ctx context.Context,
	sessionID string,
	chunkIndex int64,
) (bool, error) {

	query := `
		SELECT EXISTS(
			SELECT 1
			FROM upload_chunks
			WHERE session_id = $1
			AND chunk_index = $2
		)
	`

	var exists bool

	err := r.db.QueryRowContext(
		ctx,
		query,
		sessionID,
		chunkIndex,
	).Scan(&exists)

	return exists, err
}

func (r *Repository) GetChunks(
	ctx context.Context,
	sessionID string,
) ([]Chunk, error) {

	query := `
		SELECT
			session_id,
			chunk_index,
			chunk_size,
			checksum,
			data,
			verified,
			uploaded_at
		FROM upload_chunks
		WHERE session_id = $1
		ORDER BY chunk_index ASC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		sessionID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var chunks []Chunk

	for rows.Next() {

		var c Chunk

		err := rows.Scan(
			&c.SessionID,
			&c.ChunkIndex,
			&c.ChunkSize,
			&c.Checksum,
			&c.Data,
			&c.Verified,
			&c.UploadedAt,
		)

		if err != nil {
			return nil, err
		}

		chunks = append(
			chunks,
			c,
		)
	}

	return chunks, nil
}

func (r *Repository) CompleteSession(
	ctx context.Context,
	tx *sql.Tx,
	sessionID string,
) error {

	query := `
		UPDATE upload_sessions
		SET
			completed = TRUE,
			upload_status = 'completed',
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		sessionID,
	)

	return err
}
