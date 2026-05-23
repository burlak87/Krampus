package storage

import "context"

func (r *Repository) FailJob(
	ctx context.Context,
	jobID int64,
	failure string,
) error {

	query := `
        UPDATE media_jobs
        SET
            status = 'failed',
            error = $2
        WHERE id = $1
    `

	_, err := r.db.ExecContext(
		ctx,
		query,
		jobID,
		failure,
	)

	return err
}
