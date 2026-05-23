package storage

import (
	"context"
	"database/sql"
)

type FolderService struct {
	db *sql.DB
}

func NewFolderService(
	db *sql.DB,
) *FolderService {

	return &FolderService{
		db: db,
	}
}

func (s *FolderService) CreateFolder(
	ctx context.Context,
	id string,
	userID string,
	title string,
) error {

	query := `
		INSERT INTO saved_folders(
			id,
			user_id,
			title
		)
		VALUES($1,$2,$3)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		id,
		userID,
		title,
	)

	return err
}
