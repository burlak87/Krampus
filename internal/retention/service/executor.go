package service

import (
	"context"
	"database/sql"
	"fmt"

	retentionDomain "krampus/internal/retention/domain"
)

type Executor struct {
	db *sql.DB
}

func NewExecutor(db *sql.DB) *Executor {
	return &Executor{db: db}
}

func (e *Executor) Execute(ctx context.Context, policy retentionDomain.Policy) error {
	if !policy.AutoDelete {
		return nil
	}

	query := fmt.Sprintf(`
		DELETE FROM media_files
		WHERE media_type = $1
		AND created_at < NOW() - INTERVAL '%d days'
	`, policy.RetentionDays)

	_, err := e.db.ExecContext(ctx, query, policy.MediaType)
	return err
}
