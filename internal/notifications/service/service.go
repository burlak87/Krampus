package service

import (
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
