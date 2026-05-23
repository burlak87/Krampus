package permissions

import (
	"context"
	"database/sql"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(
	db *sql.DB,
) *PostgresRepository {

	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) GetUserPermissions(
	ctx context.Context,
	userID string,
	roomID string,
) ([]string, error) {

	query := `
		SELECT DISTINCT p.name
		FROM chat_memberships cm
		JOIN roles r
			ON r.id = cm.role_id
		JOIN role_permissions rp
			ON rp.role_id = r.id
		JOIN permissions p
			ON p.id = rp.permission_id
		WHERE cm.user_id = $1
		AND cm.room_id = $2
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		userID,
		roomID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []string

	for rows.Next() {

		var permission string

		err := rows.Scan(
			&permission,
		)

		if err != nil {
			return nil, err
		}

		result = append(
			result,
			permission,
		)
	}

	return result, nil
}
