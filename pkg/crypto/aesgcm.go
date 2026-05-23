package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

func EncryptAESGCM(data []byte, key []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, gcm.NonceSize())

	_, err = rand.Read(nonce)
	if err != nil {
		return nil, nil, err
	}

	encrypted := gcm.Seal(nil, nonce, data, nil)

	return encrypted, nonce, nil
}
