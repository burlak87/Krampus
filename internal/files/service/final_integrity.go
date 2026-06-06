package service

import "context"

type FinalIntegrityVerifier struct {
	objects ObjectStorage
}

func NewFinalIntegrityVerifier(
	objects ObjectStorage,
) *FinalIntegrityVerifier {

	return &FinalIntegrityVerifier{
		objects: objects,
	}
}

func (v *FinalIntegrityVerifier) VerifyObject(
	ctx context.Context,
	key string,
	expectedHash string,
) error {

	data, err := v.objects.GetObject(
		ctx,
		key,
	)

	if err != nil {
		return err
	}

	actual := SHA256(data)

	return VerifyUploadHash(
		actual,
		expectedHash,
	)
}
