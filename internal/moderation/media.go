package moderation

import "context"

type MediaScanner interface {
	Scan(
		ctx context.Context,
		path string,
	) error
}

type MediaModeration struct {
	scanner MediaScanner
}

func NewMediaModeration(
	scanner MediaScanner,
) *MediaModeration {

	return &MediaModeration{
		scanner: scanner,
	}
}

func (m *MediaModeration) Moderate(
	ctx context.Context,
	path string,
) error {

	return m.scanner.Scan(
		ctx,
		path,
	)
}
