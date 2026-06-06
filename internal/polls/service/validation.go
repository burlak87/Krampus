package service

import "errors"

var (
	ErrTooManyChoices = errors.New(
		"too many choices",
	)
)

func ValidateVote(
	maxChoices int,
	selected []string,
) error {

	if len(selected) > maxChoices {
		return ErrTooManyChoices
	}

	return nil
}
