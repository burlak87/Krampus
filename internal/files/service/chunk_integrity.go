package service

import "context"

type IntegrityVerifier struct {
	objects ObjectStorage
}

func NewIntegrityVerifier(
	objects ObjectStorage,
) *IntegrityVerifier {

	return &IntegrityVerifier{
		objects: objects,
	}
}

func (v *IntegrityVerifier) VerifyChunk(
	ctx context.Context,
	chunk Chunk,
) error {

	key := ChunkObjectKey(
		chunk.SessionID,
		chunk.ChunkIndex,
	)

	data, err := v.objects.GetObject(
		ctx,
		key,
	)

	if err != nil {
		return err
	}

	actual := SHA256(
		data,
	)

	return VerifyUploadHash(
		actual,
		chunk.Checksum,
	)
}
