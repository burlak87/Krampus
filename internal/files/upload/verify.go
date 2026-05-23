package upload

import (
	"context"
)

func (r *ChunkRepository) MissingChunks(
	ctx context.Context,
	sessionID string,
	totalChunks int64,
) ([]int64, error) {

	query := `
		SELECT chunk_index
		FROM upload_chunks
		WHERE session_id = $1
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

	exists := map[int64]bool{}

	for rows.Next() {

		var index int64

		err := rows.Scan(&index)
		if err != nil {
			return nil, err
		}

		exists[index] = true
	}

	var missing []int64

	for i := int64(0); i < totalChunks; i++ {

		if !exists[i] {
			missing = append(
				missing,
				i,
			)
		}
	}

	return missing, nil
}
