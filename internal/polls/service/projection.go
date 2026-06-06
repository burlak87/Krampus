package service

import (
	"context"
	"database/sql"
)

type Projection struct {
	db *sql.DB
}

func NewProjection(
	db *sql.DB,
) *Projection {

	return &Projection{
		db: db,
	}
}

func (p *Projection) IncrementVote(
	ctx context.Context,
	pollID string,
	optionID string,
) error {

	query := `
		INSERT INTO poll_projection(
			poll_id,
			option_id,
			vote_count
		)
		VALUES($1,$2,1)

		ON CONFLICT(
			poll_id,
			option_id
		)

		DO UPDATE SET
			vote_count =
				poll_projection.vote_count + 1
	`

	_, err := p.db.ExecContext(
		ctx,
		query,
		pollID,
		optionID,
	)

	return err
}
