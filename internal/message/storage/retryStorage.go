package storage

import (
	"context"
	"encoding/json"

	message "krampus/internal/message/domain"
	database "krampus/internal/sqlc"
	"krampus/pkg/types"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type PSQLRetryRepository struct {
	queries *database.Queries
}

func NewPSQLRetryRepository(queries *database.Queries) *PSQLRetryRepository {
	return &PSQLRetryRepository{
		queries: queries,
	}
}

func (r *PSQLRetryRepository) Enqueue(ctx context.Context, job *message.DeliveryJob) error {
	payload, err := json.Marshal(job.Message)
	if err != nil {
		return err
	}

	return r.queries.CreateRetryJob(ctx, database.CreateRetryJobParams{
		ID: pgtype.UUID{
			Bytes: uuid.New(),
			Valid: true,
		},
		MessageID: job.Message.ID.String(),
		UserID:    job.UserID.String(),
		RoomID:    job.RoomID.String(),
		Payload:   payload,
		Attempt:   int32(job.Attempt),
		NextRetryAt: pgtype.Timestamp{
			Time:  job.NextRetryAt,
			Valid: true,
		},
	},
	)
}

func (r *PSQLRetryRepository) GetReadyJobs(ctx context.Context, limit int) ([]*message.DeliveryJob, error) {
	rows, err := r.queries.GetReadyRetryJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}

	jobs := make([]*message.DeliveryJob, 0, len(rows))
	for _, row := range rows {
		var msg message.BaseMessage
		if err := json.Unmarshal(row.Payload, &msg); err != nil {
			continue
		}

		jobs = append(jobs, &message.DeliveryJob{
			Message:     &msg,
			UserID:      types.UserID(row.UserID),
			RoomID:      types.RoomID(row.RoomID),
			Attempt:     int(row.Attempt),
			NextRetryAt: row.NextRetryAt.Time,
		},
		)
	}

	return jobs, nil
}

func (r *PSQLRetryRepository) Delete(ctx context.Context, messageID types.MessageID, userID types.UserID) error {
	return r.queries.DeleteRetryJob(ctx, database.DeleteRetryJobParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
	},
	)
}
