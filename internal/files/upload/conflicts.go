package upload

import "errors"

var (
	ErrChunkAlreadyUploaded = errors.New(
		"chunk already uploaded",
	)
	ErrResumeConflict = errors.New(
		"resume conflict",
	)
)

func ValidateResumeGeneration(
	current int64,
	incoming int64,
) error {

	if incoming < current {
		return ErrResumeConflict
	}

	return nil
}
