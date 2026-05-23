package sync

import (
	"context"
	"database/sql"
	"time"
)

type Event struct {
	ID int64

	AggregateID string

	AggregateType string

	EventType string

	Payload []byte

	CreatedAt time.Time
}

type Service struct {
	db *sql.DB
}

func NewService(
	db *sql.DB,
) *Service {

	return &Service{
		db: db,
	}
}

func (s *Service) GetEventsAfter(
	ctx context.Context,
	lastEventID int64,
	limit int,
) ([]Event, error) {

	query := `
		SELECT
			id,
			aggregate_id,
			aggregate_type,
			event_type,
			payload,
			created_at
		FROM events
		WHERE id > $1
		ORDER BY id ASC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		lastEventID,
		limit,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var events []Event

	for rows.Next() {

		var e Event

		err := rows.Scan(
			&e.ID,
			&e.AggregateID,
			&e.AggregateType,
			&e.EventType,
			&e.Payload,
			&e.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		events = append(
			events,
			e,
		)
	}

	return events, nil
}
