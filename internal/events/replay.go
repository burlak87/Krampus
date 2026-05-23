package events

import (
	"context"
	"database/sql"
)

type ReplayEngine struct {
	db *sql.DB

	projector *Projector
}

func NewReplayEngine(
	db *sql.DB,
	projector *Projector,
) *ReplayEngine {

	return &ReplayEngine{
		db:        db,
		projector: projector,
	}
}

func (r *ReplayEngine) Replay(
	ctx context.Context,
	fromSequence int64,
	batchSize int,
) error {

	for {

		query := `
			SELECT
				id,
				aggregate_id,
				aggregate_type,
				event_type,
				payload,
				event_version,
				sequence,
				created_at
			FROM events
			WHERE sequence > $1
			ORDER BY sequence ASC
			LIMIT $2
		`

		rows, err := r.db.QueryContext(
			ctx,
			query,
			fromSequence,
			batchSize,
		)

		if err != nil {
			return err
		}

		var count int

		for rows.Next() {

			var event Event

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
				rows.Close()
				return err
			}

			err = r.projector.Dispatch(
				ctx,
				event,
			)

			if err != nil {
				return err
			}

			fromSequence = event.Sequence

			count++
		}

		rows.Close()

		if count == 0 {
			break
		}
	}

	return nil
}
