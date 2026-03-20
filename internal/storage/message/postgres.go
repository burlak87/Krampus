package message

import (
	"context"
	"krampus/internal/domain"
)

func (p *PostgrtesStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
	_, err := p.db.ExecContext(ctx, `INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature) VALUES ($, $2, $3, $4, $5, $6, $7`, msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp, msg.Payload, msg.Signature)
	return err
}

func (p *PostgresStorage) createTables() {
	p.db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			type TEXT,
			user_id TEXT,
			room_id TEXT,
			timestamp BIGINT,
			payload JSONB,
			signature TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_room_timestamp ON messages(room_id, timestamp DESC);
	`)
	// rooms, users tables аналогично
}
