package metadata

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

func DecodeImage(
	path string,
) (image.Image, error) {

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	img, _, err := image.Decode(file)

	if err != nil {
		return nil, err
	}

	return img, nil
}
