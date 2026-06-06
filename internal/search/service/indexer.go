package service

import (
	"context"
	"database/sql"
)

type Indexer struct {
	db *sql.DB
}

func NewIndexer(
	db *sql.DB,
) *Indexer {

	return &Indexer{
		db: db,
	}
}

func (i *Indexer) IndexMessage(
	ctx context.Context,
	messageID string,
	roomID string,
	userID string,
	content string,
) error {

	query := `
		INSERT INTO message_search_projection(
			message_id,
			room_id,
			user_id,
			content,
			search_vector,
			created_at
		)

		VALUES(
			$1,
			$2,
			$3,
			$4,
			to_tsvector('simple', $4),
			NOW()
		)

		ON CONFLICT(message_id)

		DO UPDATE SET
			content = EXCLUDED.content,
			search_vector = EXCLUDED.search_vector
	`

	_, err := i.db.ExecContext(
		ctx,
		query,
		messageID,
		roomID,
		userID,
		content,
	)

	return err
}

func (i *Indexer) DeleteMessage(
	ctx context.Context,
	messageID string,
) error {

	query := `
		DELETE FROM message_search_projection
		WHERE message_id = $1
	`

	_, err := i.db.ExecContext(
		ctx,
		query,
		messageID,
	)

	return err
}

func (i *Indexer) ReindexEntity(
	ctx context.Context,
	entityType string,
	entityID string,
) error {

	switch entityType {

	case "message":

		return i.reindexMessage(
			ctx,
			entityID,
		)
	}

	return nil
}

func (i *Indexer) reindexMessage(
	ctx context.Context,
	messageID string,
) error {

	query := `
		INSERT INTO message_search_projection(
			message_id,
			content
		)

		SELECT
			id,
			payload->>'content'
		FROM messages
		WHERE id = $1

		ON CONFLICT(message_id)
		DO UPDATE SET
			content = EXCLUDED.content
	`

	_, err := i.db.ExecContext(
		ctx,
		query,
		messageID,
	)

	return err
}

type SearchResult struct {
	MessageID string
	RoomID    string
	UserID    string
	Content   string
}

func (i *Indexer) Search(ctx context.Context, roomID, query string, limit int) ([]SearchResult, error) {
	q := `
		SELECT message_id, room_id, user_id, content
		FROM message_search_projection
		WHERE room_id = $1
		  AND search_vector @@ plainto_tsquery('simple', $2)
		ORDER BY created_at DESC
		LIMIT $3
	`
	rows, err := i.db.QueryContext(ctx, q, roomID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.MessageID, &r.RoomID, &r.UserID, &r.Content); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (i *Indexer) BuildFilters(
	f Filters,
	builder *QueryBuilder,
) {

	arg := 1

	if f.RoomID != nil {

		builder.Add(
			"room_id = $1",
			*f.RoomID,
		)

		arg++
	}

	if f.UserID != nil {

		builder.Add(
			"user_id = $"+itoa(arg),
			*f.UserID,
		)

		arg++
	}

	if f.TopicID != nil {

		builder.Add(
			"topic_id = $"+itoa(arg),
			*f.TopicID,
		)

		arg++
	}

	if f.MessageType != nil {

		builder.Add(
			"type = $"+itoa(arg),
			*f.MessageType,
		)

		arg++
	}

	if f.HasAttachments != nil {

		builder.Add(
			"has_attachments = $"+itoa(arg),
			*f.HasAttachments,
		)

		arg++
	}

	if f.HasReactions != nil {

		builder.Add(
			"reaction_count > 0",
			true,
		)
	}

	if f.HasPoll != nil {

		builder.Add(
			"has_poll = $"+itoa(arg),
			*f.HasPoll,
		)

		arg++
	}

	if f.FromTimestamp != nil {

		builder.Add(
			"timestamp >= $"+itoa(arg),
			*f.FromTimestamp,
		)

		arg++
	}

	if f.ToTimestamp != nil {

		builder.Add(
			"timestamp <= $"+itoa(arg),
			*f.ToTimestamp,
		)

		arg++
	}
}
