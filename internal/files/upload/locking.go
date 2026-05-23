package upload

import (
	"context"
	"database/sql"
)

func LockSession(
	ctx context.Context,
	tx *sql.Tx,
	sessionID string,
) error {

	query := `
		SELECT id
		FROM upload_sessions
		WHERE id = $1
		FOR UPDATE
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		sessionID,
	)

	return err
}
