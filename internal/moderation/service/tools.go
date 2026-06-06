package service

import (
	"context"
	"database/sql"
)

type Tools struct {
	db *sql.DB
}

func NewTools(
	db *sql.DB,
) *Tools {

	return &Tools{
		db: db,
	}
}

func (t *Tools) BanUser(
	ctx context.Context,
	userID string,
	reason string,
) error {

	query := `
		INSERT INTO bans(
			user_id,
			reason,
			created_at
		)
		VALUES($1,$2,NOW())
	`

	_, err := t.db.ExecContext(
		ctx,
		query,
		userID,
		reason,
	)

	return err
}

func (t *Tools) MuteUser(
	ctx context.Context,
	userID string,
	until string,
) error {

	query := `
		INSERT INTO mutes(
			user_id,
			muted_until
		)
		VALUES($1,$2)
	`

	_, err := t.db.ExecContext(
		ctx,
		query,
		userID,
		until,
	)

	return err
}
