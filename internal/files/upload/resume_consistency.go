package upload

import "context"

type ResumeVerifier struct {
	objects ObjectStorage
}

func NewResumeVerifier(
	objects ObjectStorage,
) *ResumeVerifier {

	return &ResumeVerifier{
		objects: objects,
	}
}

func (v *ResumeVerifier) VerifyChunkExists(
	ctx context.Context,
	chunk Chunk,
) bool {

	key := ChunkObjectKey(
		chunk.SessionID,
		chunk.ChunkIndex,
	)

	_, err := v.objects.GetObject(
		ctx,
		key,
	)

	return err == nil
}
