package storage

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

func (r *Repository) AddScore(
	ctx context.Context,
	userID string,
	score float64,
) error {

	query := `
		INSERT INTO abuse_scores(
			user_id,
			score
		)
		VALUES($1,$2)

		ON CONFLICT(user_id)

		DO UPDATE SET
			score =
				abuse_scores.score + EXCLUDED.score,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		userID,
		score,
	)

	return err
}
