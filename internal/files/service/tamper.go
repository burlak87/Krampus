package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

var (
	ErrTamperedUpload = errors.New(
		"upload tampered",
	)
)

func SignMetadata(
	payload string,
	secret string,
) string {

	h := hmac.New(
		sha256.New,
		[]byte(secret),
	)

	h.Write([]byte(payload))

	return hex.EncodeToString(
		h.Sum(nil),
	)
}

func VerifyMetadataSignature(
	payload string,
	signature string,
	secret string,
) bool {

	actual := SignMetadata(
		payload,
		secret,
	)

	return actual == signature
}

func VerifyUploadHash(
	actual string,
	expected string,
) error {

	if actual != expected {
		return ErrTamperedUpload
	}

	return nil
}
