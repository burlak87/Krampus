package audit

import (
	"context"
	"database/sql"

	"krampus/internal/events"
)

type Consumer struct {
	db *sql.DB
}

func NewConsumer(
	db *sql.DB,
) *Consumer {

	return &Consumer{
		db: db,
	}
}

func (c *Consumer) Handle(
	ctx context.Context,
	event events.Event,
) error {

	query := `
		INSERT INTO audit_logs(
			actor_id,
			action,
			entity_type,
			entity_id,
			payload
		)
		VALUES($1,$2,$3,$4,$5)
	`

	_, err := c.db.ExecContext(
		ctx,
		query,
		"system",
		event.EventType,
		event.AggregateType,
		event.AggregateID,
		event.Payload,
	)

	return err
}
