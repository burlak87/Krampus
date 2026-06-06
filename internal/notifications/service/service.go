package service

import (
	"context"

	"krampus/internal/notifications/providers"
)

type Service struct {
	provider providers.Provider
}

func New(
	provider providers.Provider,
) *Service {

	return &Service{
		provider: provider,
	}
}

func (s *Service) SendPush(ctx context.Context, token, title, body string) error {
	return s.provider.SendPush(ctx, token, title, body)
}
