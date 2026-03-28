package message

import (
	"context"
	"fmt"
	"krampus/internal/domain"
	"log"
	"time"
)

// SaveMessage — ОДНО сообщение
func (p *PostgresStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
	query := `
    INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
  `

	createdAt := time.Unix(0, msg.Timestamp)
	_, err := p.db.ExecContext(ctx, query,
		msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
		msg.Payload, msg.Signature, createdAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	return nil
}

// SaveMessageBatch — 1000+ msg/сек транзакцией
func (p *PostgresStorage) SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
    INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
  `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, msg := range msgs {
		createdAt := time.Unix(0, msg.Timestamp)
		_, err := stmt.ExecContext(ctx,
			msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
			msg.Payload, msg.Signature, createdAt,
		)
		if err != nil {
			return fmt.Errorf("failed to execute batch insert: %w", err)
		}
	}

	return tx.Commit()
}

// GetRoomMessages — история чата (последние N)
func (p *PostgresStorage) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
	query := `
    SELECT id, type, user_id, room_id, timestamp, payload, signature
    FROM messages
    WHERE room_id = $1
    ORDER BY timestamp DESC
    LIMIT $2
  `

	rows, err := p.db.QueryContext(ctx, query, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.BaseMessage
	for rows.Next() {
		msg := &domain.BaseMessage{}
		var createdAt time.Time
		if err := rows.Scan(&msg.ID, &msg.Type, &msg.UserID, &msg.RoomID,
			&msg.Timestamp, &msg.Payload, &msg.Signature, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		msg.Timestamp = createdAt.UnixNano() // Postgres → UnixNano
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	// 🔄 Реверс: старые→новые (для UI)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// CleanupOldMessages — cron каждые 24ч
func (p *PostgresStorage) CleanupOldMessages(ctx context.Context) error {
	query := `DELETE FROM messages WHERE expires_at < NOW()`
	result, err := p.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup old messages: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Cleaned up %d old messages", rowsAffected)
	return nil
}

// // SaveMessage — ОДНО сообщение
// func (p *PostgresStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
//   query := `
//     INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
//     VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
//   `

//   createdAt := time.Unix(0, msg.Timestamp)
//   _, err := p.db.ExecContext(ctx, query,
//     msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
//     msg.Payload, msg.Signature, createdAt,
//   )

//   if err != nil {
//     return fmt.Errorf("failed to save message: %w", err)
//   }
//   return nil
// }

// // SaveMessageBatch — 1000+ msg/сек транзакцией
// func (p *PostgresStorage) SaveMessageBatch(ctx context.Context, msgs []*domain.BaseMessage) error {
//   tx, err := p.db.BeginTx(ctx, nil)
//   if err != nil {
//     return fmt.Errorf("failed to begin transaction: %w", err)
//   }
//   defer tx.Rollback()

//   stmt, err := tx.PrepareContext(ctx, `
//     INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature, expires_at)
//     VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '60 days')
//   `)
//   if err != nil {
//     return fmt.Errorf("failed to prepare statement: %w", err)
//   }
//   defer stmt.Close()

//   for _, msg := range msgs {
//     createdAt := time.Unix(0, msg.Timestamp)
//     _, err := stmt.ExecContext(ctx,
//       msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp,
//       msg.Payload, msg.Signature, createdAt,
//     )
//     if err != nil {
//       return fmt.Errorf("failed to execute batch insert: %w", err)
//     }
//   }

//   return tx.Commit()
// }

// // GetRoomMessages — история чата (последние N)
// func (p *PostgresStorage) GetRoomMessages(ctx context.Context, roomID string, limit int) ([]*domain.BaseMessage, error) {
//   query := `
//     SELECT id, type, user_id, room_id, timestamp, payload, signature
//     FROM messages
//     WHERE room_id = $1
//     ORDER BY timestamp DESC
//     LIMIT $2
//   `

//   rows, err := p.db.QueryContext(ctx, query, roomID, limit)
//   if err != nil {
//     return nil, fmt.Errorf("failed to query messages: %w", err)
//   }
//   defer rows.Close()

//   var messages []*domain.BaseMessage
//   for rows.Next() {
//     msg := &domain.BaseMessage{}
//     var createdAt time.Time
//     if err := rows.Scan(&msg.ID, &msg.Type, &msg.UserID, &msg.RoomID,
//                        &msg.Timestamp, &msg.Payload, &msg.Signature, &createdAt); err != nil {
//       return nil, fmt.Errorf("failed to scan message: %w", err)
//     }
//     msg.Timestamp = createdAt.UnixNano() // Postgres → UnixNano
//     messages = append(messages, msg)
//   }

//   if err := rows.Err(); err != nil {
//     return nil, fmt.Errorf("rows iteration error: %w", err)
//   }

//   // 🔄 Реверс: старые→новые (для UI)
//   for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
//     messages[i], messages[j] = messages[j], messages[i]
//   }

//   return messages, nil
// }

// // CleanupOldMessages — cron каждые 24ч
// func (p *PostgresStorage) CleanupOldMessages(ctx context.Context) error {
//   query := `DELETE FROM messages WHERE expires_at < NOW()`
//   result, err := p.db.ExecContext(ctx, query)
//   if err != nil {
//     return fmt.Errorf("failed to cleanup old messages: %w", err)
//   }

//   rowsAffected, _ := result.RowsAffected()
//   log.Printf("Cleaned up %d old messages", rowsAffected)
//   return nil
// }
