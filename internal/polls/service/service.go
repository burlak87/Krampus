package service

import (
	"context"
	"database/sql"
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

func (s *Service) Vote(
	ctx context.Context,
	pollID string,
	optionID string,
	userID string,
) error {

	query := `
		INSERT INTO poll_votes(
			poll_id,
			option_id,
			user_id
		)
		VALUES($1,$2,$3)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		pollID,
		optionID,
		userID,
	)

	return err
}
