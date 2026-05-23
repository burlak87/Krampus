package storage

import "context"

func (r *Repository) CompleteJob(
	ctx context.Context,
	jobID int64,
) error {

	query := `
        UPDATE media_jobs
        SET
            status = 'completed',
            completed_at = NOW()
        WHERE id = $1
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		jobID,
	)

	return err
}
