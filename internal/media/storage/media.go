package storage

import (
	"context"
	"database/sql"
	"fmt"

	"krampus/internal/media/domain"
)

func (r *Repository) GetMediaByID(ctx context.Context, id string) (*domain.MediaFile, error) {
	query := `
		SELECT
			id, owner_id, media_type, mime_type, original_name,
			original_size, COALESCE(compressed_size, 0),
			storage_key, COALESCE(thumbnail_key, ''),
			sha256,
			COALESCE(width, 0), COALESCE(height, 0),
			COALESCE(duration_seconds, 0),
			processing_status, created_at
		FROM media_files
		WHERE id = $1
	`

	var m domain.MediaFile
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.OwnerID, &m.MediaType, &m.MimeType, &m.OriginalName,
		&m.OriginalSize, &m.CompressedSize,
		&m.StorageKey, &m.ThumbnailKey,
		&m.SHA256,
		&m.Width, &m.Height,
		&m.DurationSeconds,
		&m.ProcessingStatus, &m.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("media file not found: %s", id)
	}
	return &m, err
}

func (r *Repository) UpdateProcessedMedia(
	ctx context.Context,
	id string,
	compressedSize int64,
	storageKey string,
	thumbnailKey *string,
	width *int,
	height *int,
	status string,
) error {
	query := `
		UPDATE media_files
		SET
			compressed_size   = $2,
			storage_key       = $3,
			thumbnail_key     = $4,
			width             = $5,
			height            = $6,
			processing_status = $7
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query,
		id, compressedSize, storageKey, thumbnailKey, width, height, status,
	)
	return err
}
