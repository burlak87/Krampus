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

func (s *Service) AddReaction(
	ctx context.Context,
	messageID string,
	userID string,
	reaction string,
) error {

	query := `
		INSERT INTO reactions(
			message_id,
			user_id,
			reaction
		)
		VALUES($1,$2,$3)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		messageID,
		userID,
		reaction,
	)

	return err
}
