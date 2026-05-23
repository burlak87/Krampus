package storage

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

func (p *Projection) Save(
	ctx context.Context,
	userID string,
	messageID string,
	folderID *string,
) error {

	query := `
		INSERT INTO saved_message_projection(
			user_id,
			message_id,
			folder_id
		)
		VALUES($1,$2,$3)
	`

	_, err := p.db.ExecContext(
		ctx,
		query,
		userID,
		messageID,
		folderID,
	)

	return err
}
