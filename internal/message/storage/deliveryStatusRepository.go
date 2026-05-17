package storage

import (
	"context"
	"time"

	message "krampus/internal/message/domain"
	sqlc "krampus/internal/sqlc"
	"krampus/pkg/types"

	"github.com/jackc/pgx/v5/pgtype"
)

type PSQLDeliveryStatusRepository struct {
	queries *sqlc.Queries
}

func NewPSQLDeliveryStatusRepository(queries *sqlc.Queries) *PSQLDeliveryStatusRepository {
	return &PSQLDeliveryStatusRepository{
		queries: queries,
	}
}

func (r *PSQLDeliveryStatusRepository) UpdateStatus(ctx context.Context, messageID types.MessageID, userID types.UserID, status message.DeliveryStatus) error {
	return r.queries.UpsertDeliveryStatus(ctx, sqlc.UpsertDeliveryStatusParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
		Status:    string(status),
	},
	)
}

func (r *PSQLDeliveryStatusRepository) MarkDelivered(ctx context.Context, messageID types.MessageID, userID types.UserID, at time.Time) error {
	return r.queries.UpsertDeliveryStatus(ctx, sqlc.UpsertDeliveryStatusParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
		Status: string(
			message.DeliveryDelivered,
		),
		DeliveredAt: pgtype.Timestamp{
			Time:  at,
			Valid: true,
		},
	},
	)
}

func (r *PSQLDeliveryStatusRepository) MarkRead(ctx context.Context, messageID types.MessageID, userID types.UserID, at time.Time) error {
	return r.queries.UpsertDeliveryStatus(ctx, sqlc.UpsertDeliveryStatusParams{
		MessageID: messageID.String(),
		UserID:    userID.String(),
		Status:    string(message.DeliveryRead),
		ReadAt: pgtype.Timestamp{
			Time:  at,
			Valid: true,
		},
	},
	)
}
