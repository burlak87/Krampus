package storage

import (
	"context"

	"krampus/internal/media/domain"
)

func (r *Repository) LockPendingJobs(
	ctx context.Context,
	limit int,
) ([]domain.MediaJob, error) {

	query := `
        UPDATE media_jobs
        SET
            status = 'processing',
            started_at = NOW(),
            attempts = attempts + 1
        WHERE id IN (
            SELECT id
            FROM media_jobs
            WHERE status = 'pending'
            AND scheduled_at <= NOW()
            ORDER BY created_at ASC
            LIMIT $1
            FOR UPDATE SKIP LOCKED
        )
        RETURNING
            id,
            media_file_id,
            job_type,
            status,
            attempts,
            scheduled_at,
            started_at,
            completed_at,
            error,
            created_at
    `

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var jobs []domain.MediaJob

	for rows.Next() {
		var j domain.MediaJob

		err := rows.Scan(
			&j.ID,
			&j.MediaFileID,
			&j.JobType,
			&j.Status,
			&j.Attempts,
			&j.ScheduledAt,
			&j.StartedAt,
			&j.CompletedAt,
			&j.Error,
			&j.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		jobs = append(jobs, j)
	}

	return jobs, nil
}
