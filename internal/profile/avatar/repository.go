package avatar

import (
	"context"
	"database/sql"
)

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetAvatarMediaID(ctx context.Context, userID string) (string, error) {
	var mediaID string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(avatar_media_id, '') FROM user_profiles WHERE user_id = $1::uuid`,
		userID,
	).Scan(&mediaID)
	return mediaID, err
}

func (r *postgresRepository) SetAvatar(ctx context.Context, userID string, mediaID string) error {
	query := `
		INSERT INTO user_profiles (user_id, avatar_media_id, payload, settings)
		VALUES ($1::uuid, $2, '{}', '{}')
		ON CONFLICT (user_id) DO UPDATE
		SET avatar_media_id = EXCLUDED.avatar_media_id
	`
	_, err := r.db.ExecContext(ctx, query, userID, mediaID)
	return err
}
