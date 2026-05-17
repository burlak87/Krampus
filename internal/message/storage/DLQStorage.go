package storage

import (
	"context"
	"encoding/json"

	message "krampus/internal/message/domain"
	sqlc "krampus/internal/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type PSQLDLQRepository struct {
	queries *sqlc.Queries
}

func NewPSQLDLQRepository(queries *sqlc.Queries) *PSQLDLQRepository {
	return &PSQLDLQRepository{
		queries: queries,
	}
}

func (r *PSQLDLQRepository) Store(ctx context.Context, job *message.DeliveryJob, reason string) error {
	payload, err := json.Marshal(job.Message)
	if err != nil {
		return err
	}

	return r.queries.CreateDLQMessage(ctx, sqlc.CreateDLQMessageParams{
		ID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		MessageID: job.Message.ID.String(),
		UserID:    job.UserID.String(),
		RoomID:    job.RoomID.String(),
		Payload:   payload,
		Reason:    reason,
	},
	)
}
