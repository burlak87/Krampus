package invites

import (
	"errors"
	"strings"
)

var (
	ErrInvalidDeepLink = errors.New(
		"invalid deep link",
	)
)

func ValidateDeepLink(
	link string,
) error {

	if !strings.HasPrefix(
		link,
		"krampus://join/",
	) {

		return ErrInvalidDeepLink
	}

	return nil
}

func ExtractJoinToken(
	link string,
) (string, error) {

	err := ValidateDeepLink(
		link,
	)

	if err != nil {
		return "", err
	}

	parts := strings.Split(
		link,
		"/",
	)

	return parts[len(parts)-1], nil
}
