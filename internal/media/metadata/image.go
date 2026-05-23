package metadata

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

type ImageMetadata struct {
	Width  int
	Height int
}

func ExtractImageMetadata(
	path string,
) (*ImageMetadata, error) {

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	config, _, err := image.DecodeConfig(
		file,
	)

	if err != nil {
		return nil, err
	}

	return &ImageMetadata{
		Width:  config.Width,
		Height: config.Height,
	}, nil
}
