package avatar

import (
	"context"

	"krampus/internal/media/domain"
	"krampus/internal/media/service"
)

type MediaService interface {
	ProcessMedia(ctx context.Context, mediaFileID string) error
}

type Repository interface {
	SetAvatar(ctx context.Context, userID string, mediaID string) error
}

type Service struct {
	media MediaService
	repo  Repository
}

func New(
	media *service.Service,
	repo Repository) *Service {
	return &Service{
		media: media,
		repo:  repo,
	}
}

func (s *Service) UploadAvatar(ctx context.Context, userID string, media domain.MediaFile) error {
	err := s.media.ProcessMedia(ctx, media.ID)
	if err != nil {
		return err
	}

	return s.repo.SetAvatar(ctx, userID, media.ID)
}

func (s *Service) ProcessAvatar(
	ctx context.Context,
	mediaID string,
) error {

	return s.media.ProcessMedia(
		ctx,
		mediaID,
	)
}
