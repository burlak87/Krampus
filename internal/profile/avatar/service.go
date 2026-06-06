package avatar

import (
	"context"

	"krampus/internal/media/service"
)

type MediaService interface {
	ProcessMedia(ctx context.Context, mediaFileID string) error
}

type Repository interface {
	SetAvatar(ctx context.Context, userID string, mediaID string) error
	GetAvatarMediaID(ctx context.Context, userID string) (string, error)
}

type MediaFileInput struct {
	ID        string
	MediaType string
}

type Profile struct {
	UserID      string `json:"user_id"`
	AvatarMedia string `json:"avatar_media_id,omitempty"`
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

func (s *Service) UploadAvatar(ctx context.Context, userID string, media MediaFileInput) error {
	if err := s.media.ProcessMedia(ctx, media.ID); err != nil {
		return err
	}
	return s.repo.SetAvatar(ctx, userID, media.ID)
}

func (s *Service) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	mediaID, err := s.repo.GetAvatarMediaID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Profile{UserID: userID, AvatarMedia: mediaID}, nil
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
