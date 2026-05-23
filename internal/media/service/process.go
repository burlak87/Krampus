package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	mediaimage "krampus/internal/media/image"
	"krampus/internal/media/metadata"
	"krampus/internal/media/thumbnails"
	mediavideo "krampus/internal/media/video"

	"krampus/pkg/compression"
	cryptoutil "krampus/pkg/crypto"
)

func (s *Service) ProcessMedia(
	ctx context.Context,
	mediaFileID string,
) error {

	media, err := s.repo.GetMediaByID(
		ctx,
		mediaFileID,
	)

	if err != nil {
		return err
	}

	rawData, err := s.storage.Download(
		ctx,
		media.StorageKey,
	)

	if err != nil {
		return err
	}

	var processed []byte

	var thumbnailBytes []byte

	var width *int
	var height *int

	switch media.MediaType {

	case "image":

		tmpFile := filepath.Join(
			os.TempDir(),
			media.ID,
		)

		err = os.WriteFile(
			tmpFile,
			rawData,
			0644,
		)

		if err != nil {
			return err
		}

		meta, err := metadata.ExtractImageMetadata(
			tmpFile,
		)

		if err != nil {
			return err
		}

		width = &meta.Width
		height = &meta.Height

		img, err := metadata.DecodeImage(
			tmpFile,
		)

		if err != nil {
			return err
		}

		webpBytes, err := mediaimage.EncodeWebP(
			img,
		)

		if err != nil {
			return err
		}

		thumb := thumbnails.GenerateThumbnail(
			img,
		)

		thumbnailBytes, err = mediaimage.EncodeWebP(
			thumb,
		)

		if err != nil {
			return err
		}

		processed, err = compression.CompressBrotli(
			webpBytes,
		)

		if err != nil {
			return err
		}

	case "video":

		input := filepath.Join(
			os.TempDir(),
			media.ID+"_input",
		)

		output := filepath.Join(
			os.TempDir(),
			media.ID+"_output.mp4",
		)

		err = os.WriteFile(
			input,
			rawData,
			0644,
		)

		if err != nil {
			return err
		}

		err = mediavideo.CompressH265(
			input,
			output,
		)

		if err != nil {
			return err
		}

		videoBytes, err := os.ReadFile(
			output,
		)

		if err != nil {
			return err
		}

		processed, err = compression.CompressBrotli(
			videoBytes,
		)

		if err != nil {
			return err
		}

	default:
		return errors.New(
			"unsupported media type",
		)
	}

	encrypted, _, err := cryptoutil.EncryptAESGCM(
		processed,
		s.encryptionKey,
	)

	if err != nil {
		return err
	}

	finalKey := fmt.Sprintf(
		"media/%s/final",
		media.ID,
	)

	err = s.storage.Upload(
		ctx,
		finalKey,
		encrypted,
	)

	if err != nil {
		return err
	}

	var thumbnailKey *string

	if thumbnailBytes != nil {

		encryptedThumb, _, err := cryptoutil.EncryptAESGCM(
			thumbnailBytes,
			s.encryptionKey,
		)

		if err != nil {
			return err
		}

		key := fmt.Sprintf(
			"media/%s/thumb",
			media.ID,
		)

		err = s.storage.Upload(
			ctx,
			key,
			encryptedThumb,
		)

		if err != nil {
			return err
		}

		thumbnailKey = &key
	}

	err = s.repo.UpdateProcessedMedia(
		ctx,
		media.ID,
		int64(len(encrypted)),
		finalKey,
		thumbnailKey,
		width,
		height,
		"processed",
	)

	if err != nil {
		return err
	}

	return s.events.Publish(
		ctx,
		"media_processed",
		map[string]any{
			"media_id": media.ID,
		},
	)
}
