package storage

import (
	"context"
	"encoding/json"
	"time"

	message "krampus/internal/message/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepositoryPsql struct {
	db *pgxpool.Pool
}

func NewOutboxRepositoryPsql(
	db *pgxpool.Pool,
) *OutboxRepositoryPsql {

	return &OutboxRepositoryPsql{
		db: db,
	}
}

func (r *OutboxRepositoryPsql) SaveEvent(
	ctx context.Context,
	event *message.OutboxEvent,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		INSERT INTO outbox_events (
			id,
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6)
		`,
		uuid.MustParse(event.ID),
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		time.Now(),
	)

	return err
}

func (r *OutboxRepositoryPsql) GetUnpublishedEvents(
	ctx context.Context,
	limit int,
) ([]*message.OutboxEvent, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			created_at,
			retry_count,
			COALESCE(last_error, '')
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
		`,
		limit,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []*message.OutboxEvent

	for rows.Next() {

		var event message.OutboxEvent

		var payload json.RawMessage

		err := rows.Scan(
			&event.ID,
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&payload,
			&event.CreatedAt,
			&event.RetryCount,
			&event.LastError,
		)

		if err != nil {
			return nil, err
		}

		event.Payload = payload

		result = append(result, &event)
	}

	return result, nil
}

func (r *OutboxRepositoryPsql) MarkPublished(
	ctx context.Context,
	eventID string,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET published_at = NOW()
		WHERE id = $1
		`,
		uuid.MustParse(eventID),
	)

	return err
}

func (r *OutboxRepositoryPsql) IncrementRetry(
	ctx context.Context,
	eventID string,
	erMsg string,
) error {

	_, err := r.db.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET
			retry_count = retry_count + 1,
			last_error = $2
		WHERE id = $1
		`,
		uuid.MustParse(eventID),
		erMsg,
	)

	return err
}
