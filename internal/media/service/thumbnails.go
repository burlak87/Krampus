package service

import (
	"image"

	"github.com/nfnt/resize"
)

func GenerateThumbnail(
	img image.Image,
) image.Image {

	return resize.Thumbnail(
		320,
		320,
		img,
		resize.Lanczos3,
	)
}
