package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyRepositoryPsql struct {
	db *pgxpool.Pool
}

func NewIdempotencyRepositoryPsql(
	db *pgxpool.Pool,
) *IdempotencyRepositoryPsql {

	return &IdempotencyRepositoryPsql{
		db: db,
	}
}

func (r *IdempotencyRepositoryPsql) IsDuplicate(
	ctx context.Context,
	key string,
) (bool, error) {

	var exists bool

	err := r.db.QueryRow(
		ctx,
		`
		SELECT EXISTS(
			SELECT 1
			FROM message_deduplication
			WHERE idempotency_key = $1
		)
		`,
		key,
	).Scan(&exists)

	return exists, err
}

func (r *IdempotencyRepositoryPsql) Save(
	ctx context.Context,
	key string,
	messageID string,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		INSERT INTO message_deduplication (
			idempotency_key,
			message_id
		)
		VALUES ($1,$2)
		ON CONFLICT DO NOTHING
		`,
		key,
		messageID,
	)

	return err
}
