package storage

import (
	"context"

	message "krampus/internal/message/domain"
	database "krampus/internal/sqlc"
	"krampus/pkg/types"
)

type PSQLReplayRepository struct {
	queries *database.Queries
}

func NewPSQLReplayRepository(
	queries *database.Queries,
) *PSQLReplayRepository {

	return &PSQLReplayRepository{
		queries: queries,
	}
}

func (r *PSQLReplayRepository) GetMessagesAfter(
	ctx context.Context,
	roomID types.RoomID,
	after int64,
	limit int,
) ([]*message.BaseMessage, error) {

	rows, err := r.queries.GetMessagesAfter(
		ctx,
		database.GetMessagesAfterParams{
			RoomID:     roomID.String(),
			AfterTs:    after,
			LimitCount: int32(limit),
		},
	)

	if err != nil {
		return nil, err
	}

	result := make([]*message.BaseMessage, 0, len(rows))

	for _, row := range rows {

		result = append(result, &message.BaseMessage{
			ID:        types.MessageID(row.ID),
			Type:      message.MessageType(row.Type),
			UserID:    types.UserID(row.UserID),
			RoomID:    types.RoomID(row.RoomID),
			Timestamp: row.Timestamp,
			Payload:   row.Payload,
			Signature: func() string {
				if row.Signature.Valid {
					return row.Signature.String
				}
				return ""
			}(),
		})
	}

	return result, nil
}
