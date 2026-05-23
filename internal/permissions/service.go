package permissions

import "context"

type Repository interface {
	GetUserPermissions(
		ctx context.Context,
		userID string,
		roomID string,
	) ([]string, error)
}

type Service struct {
	repo Repository
}

func NewService(
	repo Repository,
) *Service {

	return &Service{
		repo: repo,
	}
}

func (s *Service) HasPermission(
	ctx context.Context,
	userID string,
	roomID string,
	permission string,
) (bool, error) {

	perms, err := s.repo.GetUserPermissions(
		ctx,
		userID,
		roomID,
	)

	if err != nil {
		return false, err
	}

	for _, p := range perms {
		if p == permission {
			return true, nil
		}
	}

	return false, nil
}
