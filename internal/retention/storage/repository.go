package storage

import (
	"context"
	"database/sql"

	retentionDomain "krampus/internal/retention/domain"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetPolicies(ctx context.Context) ([]retentionDomain.Policy, error) {
	query := `
		SELECT id, media_type, retention_days, auto_delete, created_at
		FROM media_retention_policies
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []retentionDomain.Policy

	for rows.Next() {
		var p retentionDomain.Policy

		if err := rows.Scan(&p.ID, &p.MediaType, &p.RetentionDays, &p.AutoDelete, &p.CreatedAt); err != nil {
			return nil, err
		}

		result = append(result, p)
	}

	return result, nil
}
