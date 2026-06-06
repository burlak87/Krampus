package service

import "context"

// GetChunksForIntegrityCheck returns up to limit chunks that need verification.
func (r *Repository) GetChunksForIntegrityCheck(ctx context.Context, limit int) ([]Chunk, error) {
	query := `
		SELECT
			session_id, chunk_index, chunk_size, checksum,
			data, verified, uploaded_at
		FROM upload_chunks
		WHERE verified = FALSE
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(
			&c.SessionID, &c.ChunkIndex, &c.ChunkSize, &c.Checksum,
			&c.Data, &c.Verified, &c.UploadedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// GetBrokenChunks returns chunks whose sessions are completed but checksums are unverified.
func (r *Repository) GetBrokenChunks(ctx context.Context) ([]Chunk, error) {
	query := `
		SELECT
			uc.session_id, uc.chunk_index, uc.chunk_size, uc.checksum,
			uc.data, uc.verified, uc.uploaded_at
		FROM upload_chunks uc
		JOIN upload_sessions us ON us.id = uc.session_id
		WHERE us.completed = TRUE
		AND uc.verified = FALSE
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(
			&c.SessionID, &c.ChunkIndex, &c.ChunkSize, &c.Checksum,
			&c.Data, &c.Verified, &c.UploadedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}
