package retention

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

func (r *Repository) GetPolicies(
	ctx context.Context,
) ([]Policy, error) {

	query := `
		SELECT
			id,
			media_type,
			retention_days,
			auto_delete,
			created_at
		FROM media_retention_policies
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []Policy

	for rows.Next() {

		var p Policy

		err := rows.Scan(
			&p.ID,
			&p.MediaType,
			&p.RetentionDays,
			&p.AutoDelete,
			&p.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		result = append(
			result,
			p,
		)
	}

	return result, nil
}
