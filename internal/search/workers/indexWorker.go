package workers

import (
	"context"
	"database/sql"
	"log"
	"time"

	searchSvc "krampus/internal/search/service"
)

type QueueItem struct {
	ID int64

	EntityType string

	EntityID string

	Operation string
}

type IndexWorker struct {
	db *sql.DB

	indexer *searchSvc.Indexer
}

func NewIndexWorker(
	db *sql.DB,
	indexer *searchSvc.Indexer,
) *IndexWorker {

	return &IndexWorker{
		db:      db,
		indexer: indexer,
	}
}

func (w *IndexWorker) Start(
	ctx context.Context,
) {

	ticker := time.NewTicker(
		5 * time.Second,
	)

	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:

			err := w.process(ctx)

			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (w *IndexWorker) process(
	ctx context.Context,
) error {

	tx, err := w.db.BeginTx(
		ctx,
		nil,
	)

	if err != nil {
		return err
	}

	defer tx.Rollback()

	query := `
		SELECT
			id,
			entity_type,
			entity_id,
			operation
		FROM search_index_queue
		WHERE processed = FALSE
		ORDER BY id ASC
		LIMIT 100
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.QueryContext(
		ctx,
		query,
	)

	if err != nil {
		return err
	}

	defer rows.Close()

	type QueueItem struct {
		ID int64

		EntityType string

		EntityID string

		Operation string
	}

	var items []QueueItem

	for rows.Next() {

		var item QueueItem

		err := rows.Scan(
			&item.ID,
			&item.EntityType,
			&item.EntityID,
			&item.Operation,
		)

		if err != nil {
			return err
		}

		items = append(
			items,
			item,
		)
	}

	for _, item := range items {

		err := w.handleItem(
			ctx,
			item.EntityType,
			item.EntityID,
			item.Operation,
		)

		if err != nil {

			_ = w.moveToDLQ(
				ctx,
				item.EntityType,
				item.EntityID,
				item.Operation,
				err,
			)

			continue
		}

		_, err = tx.ExecContext(
			ctx,
			`
			UPDATE search_index_queue
			SET processed = TRUE
			WHERE id = $1
			`,
			item.ID,
		)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (w *IndexWorker) handleItem(
	ctx context.Context,
	entityType string,
	entityID string,
	operation string,
) error {

	switch operation {

	case "delete":

		return w.indexer.DeleteMessage(
			ctx,
			entityID,
		)

	case "index":

		return w.indexer.ReindexEntity(
			ctx,
			entityType,
			entityID,
		)
	}

	return nil
}

func (w *IndexWorker) moveToDLQ(
	ctx context.Context,
	entityType string,
	entityID string,
	operation string,
	failure error,
) error {

	query := `
		INSERT INTO search_index_dlq(
			entity_type,
			entity_id,
			operation,
			error
		)
		VALUES($1,$2,$3,$4)
	`

	_, err := w.db.ExecContext(
		ctx,
		query,
		entityType,
		entityID,
		operation,
		failure.Error(),
	)

	return err
}
