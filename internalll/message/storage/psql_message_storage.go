package storage

import (
	"database/sql"
	"fmt"
	"time"
)

type PostgresStorage struct {
	db  *sql.DB
	dsn string
}

func NewPostgres(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	p := &PostgresStorage{db: db, dsn: dsn}
	return p, p.createTables()
}

func (p *PostgresStorage) createTables() error {
	queries := []string{
		// 👥 users
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
      		name TEXT NOT NULL,
        	email TEXT UNIQUE,
         	status TEXT DEFAULT 'offline',
          	last_active BIGINT,
           	created_at BIGINT NOT NULL,
            permissions JSONB DEFAULT '[]',
            metadata JSONB
		)`,
		// 🏠 rooms
		`CREATE TABLE IF NOT EXISTS rooms (
	      id TEXT PRIMARY KEY,
	      type TEXT NOT NULL,
	      owner_id TEXT NOT NULL,
	      name TEXT,
	      members JSONB NOT NULL,
	      settings JSONB,
	      created_at BIGINT NOT NULL,
	      updated_at BIGINT NOT NULL
	    )`,
		// 📨 messages (TTL + JSONB)
		`CREATE TABLE IF NOT EXISTS messages (
	      id TEXT PRIMARY KEY,
	      type TEXT NOT NULL,
	      user_id TEXT NOT NULL,
	      room_id TEXT NOT NULL,
	      timestamp BIGINT NOT NULL,
	      payload JSONB NOT NULL,
	      signature TEXT,
	      created_at TIMESTAMP DEFAULT NOW(),
	      expires_at TIMESTAMP
	    )`,
	}

	// 🔥 ИНДЕКСЫ ДЛЯ ПРОИЗВОДИТЕЛЬНОСТИ
	indexes := []string{
		// Быстрый поиск сообщений по комнате+времени (DESC для истории)
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_room_timestamp
	     ON messages(room_id, timestamp DESC)`,

		// Поиск сообщений пользователя
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_user_timestamp
	     ON messages(user_id, timestamp DESC)`,

		// TTL очистка (автоматическая)
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_expires
	     ON messages(expires_at) WHERE expires_at < NOW()`,

		// Комнаты по владельцу
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rooms_owner
	     ON rooms(owner_id)`,
	}

	for _, query := range append(queries, indexes...) {
		if _, err := p.db.Exec(query); err != nil {
			return fmt.Errorf("Failed to create table/index: %w", err)
		}
	}
	return nil
}

// func (p *PostgrtesStorage) SaveMessage(ctx context.Context, msg *domain.BaseMessage) error {
// 	_, err := p.db.ExecContext(ctx, `INSERT INTO messages (id, type, user_id, room_id, timestamp, payload, signature) VALUES ($, $2, $3, $4, $5, $6, $7`, msg.ID, msg.Type, msg.UserID, msg.RoomID, msg.Timestamp, msg.Payload, msg.Signature)
// 	return err
// }

// func (p *PostgresStorage) createTables() {
// 	p.db.Exec(`
// 		CREATE TABLE IF NOT EXISTS messages (
// 			id TEXT PRIMARY KEY,
// 			type TEXT,
// 			user_id TEXT,
// 			room_id TEXT,
// 			timestamp BIGINT,
// 			payload JSONB,
// 			signature TEXT,
// 			created_at TIMESTAMP DEFAULT NOW()
// 		);
// 		CREATE INDEX IF NOT EXISTS idx_room_timestamp ON messages(room_id, timestamp DESC);
// 	`)
// 	// rooms, users tables аналогично
// }

// type PostgresStorage struct {
//   db  *sql.DB
//   dsn string
// }

// func NewPostgres(dsn string) (*PostgresStorage, error) {
//   db, err := sql.Open("postgres", dsn)
//   if err != nil {
//     return nil, fmt.Errorf("failed to open postgres: %w", err)
//   }

//   // 🛠️ Pool настройки
//   db.SetMaxOpenConns(100)
//   db.SetMaxIdleConns(10)
//   db.SetConnMaxLifetime(5 * time.Minute)

//   if err := db.Ping(); err != nil {
//     return nil, fmt.Errorf("failed to ping postgres: %w", err)
//   }

//   p := &PostgresStorage{db: db, dsn: dsn}
//   return p, p.createTables()
// }

// func (p *PostgresStorage) createTables() error {
// 	queries := []string{
// 		// 👥 users
// 		`CREATE TABLE IF NOT EXISTS users (
//       id TEXT PRIMARY KEY,
//       name TEXT NOT NULL,
//       email TEXT UNIQUE,
//       status TEXT DEFAULT 'offline',
//       last_active BIGINT,
//       created_at BIGINT NOT NULL,
//       permissions JSONB DEFAULT '[]',
//       metadata JSONB
//     )`,

// 		// 🏠 rooms
// 		`CREATE TABLE IF NOT EXISTS rooms (
//       id TEXT PRIMARY KEY,
//       type TEXT NOT NULL,
//       owner_id TEXT NOT NULL,
//       name TEXT,
//       members JSONB NOT NULL,
//       settings JSONB,
//       created_at BIGINT NOT NULL,
//       updated_at BIGINT NOT NULL
//     )`,

// 		// 📨 messages (TTL + JSONB)
// 		`CREATE TABLE IF NOT EXISTS messages (
//       id TEXT PRIMARY KEY,
//       type TEXT NOT NULL,
//       user_id TEXT NOT NULL,
//       room_id TEXT NOT NULL,
//       timestamp BIGINT NOT NULL,
//       payload JSONB NOT NULL,
//       signature TEXT,
//       created_at TIMESTAMP DEFAULT NOW(),
//       expires_at TIMESTAMP
//     )`,
// 	}

// 	// 🔥 ИНДЕКСЫ ДЛЯ ПРОИЗВОДИТЕЛЬНОСТИ
// 	indexes := []string{
// 		// Быстрый поиск сообщений по комнате+времени (DESC для истории)
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_room_timestamp
//      ON messages(room_id, timestamp DESC)`,

// 		// Поиск сообщений пользователя
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_user_timestamp
//      ON messages(user_id, timestamp DESC)`,

// 		// TTL очистка (автоматическая)
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_expires
//      ON messages(expires_at) WHERE expires_at < NOW()`,

// 		// Комнаты по владельцу
// 		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_rooms_owner
//      ON rooms(owner_id)`,
// 	}

// 	for _, query := range append(queries, indexes...) {
// 		if _, err := p.db.Exec(query); err != nil {
// 			return fmt.Errorf("failed to create table/index: %w", err)
// 		}
// 	}
// 	return nil
// }

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
