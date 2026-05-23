package stickers

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

func (s *Service) AddSticker(
	ctx context.Context,
	id string,
	packID string,
	emoji string,
	mediaFileID string,
) error {

	query := `
		INSERT INTO stickers(
			id,
			pack_id,
			emoji,
			media_file_id
		)
		VALUES($1,$2,$3,$4)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		id,
		packID,
		emoji,
		mediaFileID,
	)

	return err
}
