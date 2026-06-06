package service

import (
	"context"
	"database/sql"
)

type Aggregation struct {
	db *sql.DB
}

func NewAggregation(
	db *sql.DB,
) *Aggregation {

	return &Aggregation{
		db: db,
	}
}

func (a *Aggregation) CountVotes(
	ctx context.Context,
	pollID string,
	optionID string,
) (int64, error) {

	query := `
		SELECT COUNT(*)
		FROM poll_votes
		WHERE poll_id = $1
		AND option_id = $2
	`

	var count int64

	err := a.db.QueryRowContext(
		ctx,
		query,
		pollID,
		optionID,
	).Scan(&count)

	return count, err
}
