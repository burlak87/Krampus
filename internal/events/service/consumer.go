package service

import (
	"context"
	"database/sql"
	"log"
	"time"

	evdomain "krampus/internal/events/domain"
)

type Consumer struct {
	db          *sql.DB
	projector   *Projector
	checkpoints *CheckpointRepository
	idempotency *IdempotencyRepository
	name        string
	batchSize   int
}

func NewConsumer(
	db *sql.DB,
	projector *Projector,
	checkpoints *CheckpointRepository,
	name string,
	batchSize int,
) *Consumer {
	return &Consumer{
		db:          db,
		projector:   projector,
		checkpoints: checkpoints,
		name:        name,
		batchSize:   batchSize,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.process(ctx); err != nil {
				log.Println(err)
			}
		}
	}
}

func (c *Consumer) process(ctx context.Context) error {
	lastSequence, err := c.checkpoints.Get(ctx, c.name)
	if err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	query := `
		SELECT id, aggregate_id, aggregate_type, event_type,
		       payload, event_version, sequence, created_at
		FROM events
		WHERE sequence > $1
		ORDER BY sequence ASC
		LIMIT $2
	`

	rows, err := tx.QueryContext(ctx, query, lastSequence, c.batchSize)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer rows.Close()

	var processed int64

	for rows.Next() {
		var event evdomain.Event

		err := rows.Scan(
			&event.ID,
			&event.AggregateID,
			&event.AggregateType,
			&event.EventType,
			&event.Payload,
			&event.EventVersion,
			&event.Sequence,
			&event.CreatedAt,
		)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err = c.projector.Dispatch(ctx, event); err != nil {
			c.moveToDLQ(ctx, event, err)
			continue
		}

		processed = event.Sequence
	}

	if processed > 0 {
		upsert := `
			INSERT INTO projection_checkpoints(consumer_name, last_sequence)
			VALUES($1,$2)
			ON CONFLICT(consumer_name)
			DO UPDATE SET last_sequence = EXCLUDED.last_sequence, updated_at = NOW()
		`
		if _, err = tx.ExecContext(ctx, upsert, c.name, processed); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (c *Consumer) moveToDLQ(ctx context.Context, event evdomain.Event, err error) {
	query := `
		INSERT INTO event_dlq(event_id, consumer_name, error, payload)
		VALUES($1,$2,$3,$4)
	`
	_, _ = c.db.ExecContext(ctx, query, event.ID, c.name, err.Error(), event.Payload)
}
