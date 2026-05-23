package upload

import (
	"crypto/sha256"
	"encoding/hex"
)

func VerifyChecksum(
	data []byte,
	expected string,
) bool {

	hash := sha256.Sum256(data)

	actual := hex.EncodeToString(
		hash[:],
	)

	return actual == expected
}
