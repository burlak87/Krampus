package service

import (
	"context"
	"database/sql"

	syncDomain "krampus/internal/sync/domain"
)

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
) ([]syncDomain.Event, error) {

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

	var events []syncDomain.Event

	for rows.Next() {

		var e syncDomain.Event

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
