package upload

import "fmt"

func ChunkObjectKey(
	sessionID string,
	chunkIndex int64,
) string {

	return fmt.Sprintf(
		"uploads/%s/chunks/%d",
		sessionID,
		chunkIndex,
	)
}
