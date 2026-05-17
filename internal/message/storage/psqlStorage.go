package storage

import (
	"context"
	"fmt"

	"krampus/internal/message/domain"
	database "krampus/internal/sqlc"
	"krampus/pkg/types"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessagePGStorage struct {
	pool    *pgxpool.Pool
	queries *database.Queries
}

func NewMessagePGStorage(pool *pgxpool.Pool, queries *database.Queries) *MessagePGStorage {
	return &MessagePGStorage{
		pool:    pool,
		queries: queries,
	}
}

// SaveMessage — сохранение одного сообщения через sqlc
func (p *MessagePGStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
	return p.queries.SaveMessage(ctx, database.SaveMessageParams{
		ID:        msg.ID.String(),
		Type:      string(msg.Type),
		UserID:    msg.UserID.String(),
		RoomID:    msg.RoomID.String(),
		Timestamp: msg.Timestamp,
		Payload:   msg.Payload,
		Signature: pgtype.Text{String: msg.Signature, Valid: msg.Signature != ""},
	})
}

// SaveMessageBatch — пакетное сохранение через транзакцию sqlc
func (p *MessagePGStorage) SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error {
	// Создаем транзакцию, используя переданный пул
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// "Привязываем" запросы к этой транзакции
	qtx := p.queries.WithTx(tx)

	for _, msg := range msgs {
		err := qtx.SaveMessage(ctx, database.SaveMessageParams{
			ID:        msg.ID.String(),
			Type:      string(msg.Type),
			UserID:    msg.UserID.String(),
			RoomID:    msg.RoomID.String(),
			Timestamp: msg.Timestamp,
			Payload:   msg.Payload,
			Signature: pgtype.Text{String: msg.Signature, Valid: msg.Signature != ""},
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetRoomMessages — история чата через sqlc
func (p *MessagePGStorage) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
	rows, err := p.queries.GetRoomMessages(ctx, database.GetRoomMessagesParams{
		RoomID: roomID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get room messages: %w", err)
	}

	messages := make([]*domain.BaseMessage, 0, len(rows))
	for _, row := range rows {
		var ts int64
		if row.CreatedAt.Valid {
			ts = row.CreatedAt.Time.UnixNano()
		}

		messages = append(messages, &domain.BaseMessage{
			ID:        types.MessageID(row.ID),
			Type:      domain.MessageType(row.Type),
			UserID:    types.UserID(row.UserID),
			RoomID:    types.RoomID(row.RoomID),
			Timestamp: ts,
			Payload:   row.Payload,
			Signature: row.Signature.String,
		})
	}

	// Реверс для хронологического порядка
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetMessage — получение одного сообщения по ID
func (p *MessagePGStorage) GetMessage(ctx context.Context, id string) (*domain.BaseMessage, error) {
	row, err := p.queries.GetMessage(ctx, id)
	if err != nil {
		return nil, err
	}

	var ts int64
	if row.CreatedAt.Valid {
		ts = row.CreatedAt.Time.UnixNano()
	}

	return &domain.BaseMessage{
		ID:        types.MessageID(row.ID),
		Type:      domain.MessageType(row.Type),
		UserID:    types.UserID(row.UserID),
		RoomID:    types.RoomID(row.RoomID),
		Timestamp: ts,
		Payload:   row.Payload,
		Signature: row.Signature.String,
	}, nil
}

// CleanupOldMessages — удаление старых сообщений каждые 24ч
func (p *MessagePGStorage) CleanupOldMessages(ctx context.Context) error {
	return p.queries.CleanupOldMessages(ctx)
}

func (p *MessagePGStorage) Close() {
	p.pool.Close()
}
