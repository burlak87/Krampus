package moderation

import (
	"context"
	"database/sql"
)

type ShadowRepository struct {
	db *sql.DB
}

func NewShadowRepository(
	db *sql.DB,
) *ShadowRepository {

	return &ShadowRepository{
		db: db,
	}
}

func (r *ShadowRepository) IsShadowBanned(
	ctx context.Context,
	userID string,
) (bool, error) {

	query := `
		SELECT EXISTS(
			SELECT 1
			FROM shadow_bans
			WHERE user_id = $1
			AND (
				expires_at IS NULL
				OR expires_at > NOW()
			)
		)
	`

	var exists bool

	err := r.db.QueryRowContext(
		ctx,
		query,
		userID,
	).Scan(&exists)

	return exists, err
}
