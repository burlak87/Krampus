package storage

import (
	"context"

	"krampus/internal/media/domain"
)

func (r *Repository) CreateJob(
	ctx context.Context,
	job *domain.MediaJob,
) error {

	query := `
        INSERT INTO media_jobs (
            media_file_id,
            job_type,
            status
        )
        VALUES ($1, $2, $3)
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		job.MediaFileID,
		job.JobType,
		job.Status,
	)

	return err
}
