package search

import (
	"context"
	"database/sql"
)

type Queue struct {
	db *sql.DB
}

func NewQueue(
	db *sql.DB,
) *Queue {

	return &Queue{
		db: db,
	}
}

func (q *Queue) Enqueue(
	ctx context.Context,
	entityType string,
	entityID string,
	operation string,
) error {

	query := `
		INSERT INTO search_index_queue(
			entity_type,
			entity_id,
			operation
		)
		VALUES($1,$2,$3)
	`

	_, err := q.db.ExecContext(
		ctx,
		query,
		entityType,
		entityID,
		operation,
	)

	return err
}
