package upload

import "errors"

var (
	ErrMissingChunks = errors.New(
		"missing chunks",
	)
)

func VerifyChunkCount(
	expected int,
	actual int,
) error {

	if expected != actual {
		return ErrMissingChunks
	}

	return nil
}
