package integrity

import (
	"context"

	"krampus/internal/files/upload"
)

type Repository interface {
	GetMediaHash(
		ctx context.Context,
		mediaID string,
	) (string, error)
}

type Verifier struct {
	repo Repository
}

func NewVerifier(
	repo Repository,
) *Verifier {

	return &Verifier{
		repo: repo,
	}
}

func (v *Verifier) Verify(
	ctx context.Context,
	mediaID string,
	data []byte,
) error {

	expected, err := v.repo.GetMediaHash(
		ctx,
		mediaID,
	)

	if err != nil {
		return err
	}

	actual := upload.SHA256(
		data,
	)

	return upload.VerifyUploadHash(
		actual,
		expected,
	)
}
