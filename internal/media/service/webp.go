package service

import (
	"bytes"
	"image"

	"github.com/chai2010/webp"
)

func EncodeWebP(img image.Image) ([]byte, error) {

	var buf bytes.Buffer

	err := webp.Encode(
		&buf,
		img,
		&webp.Options{
			Quality: 75,
		},
	)

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
