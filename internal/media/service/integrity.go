package service

import (
	"context"

	filesService "krampus/internal/files/service"
)

type HashRepository interface {
	GetMediaHash(
		ctx context.Context,
		mediaID string,
	) (string, error)
}

type Verifier struct {
	repo HashRepository
}

func NewVerifier(
	repo HashRepository,
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

	actual := filesService.SHA256(
		data,
	)

	return filesService.VerifyUploadHash(
		actual,
		expected,
	)
}
