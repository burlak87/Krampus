package scheduler

import (
	"context"
	"database/sql"
	"time"
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

type ScheduledMessage struct {
	MessageID   string
	ScheduledAt time.Time
}

func (r *Repository) LockPendingMessages(
	ctx context.Context,
	limit int,
) ([]ScheduledMessage, error) {

	tx, err := r.db.BeginTx(
		ctx,
		nil,
	)

	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	query := `
		UPDATE scheduled_messages
		SET published = TRUE
		WHERE message_id IN (
			SELECT message_id
			FROM scheduled_messages
			WHERE published = FALSE
			AND scheduled_at <= NOW()
			ORDER BY scheduled_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING
			message_id,
			scheduled_at
	`

	rows, err := tx.QueryContext(
		ctx,
		query,
		limit,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []ScheduledMessage

	for rows.Next() {

		var msg ScheduledMessage

		err := rows.Scan(
			&msg.MessageID,
			&msg.ScheduledAt,
		)

		if err != nil {
			return nil, err
		}

		result = append(
			result,
			msg,
		)
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return result, nil
}
