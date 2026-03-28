package chatuser

import (
	"context"
	"fmt"
	"krampus/internal/domain"
	"krampus/internal/storage"
	"log"
	"time"
)

type UserClientService struct {
	storage storage.UserClientStorage
	cache   storage.UserClientCache
}

func NewUserClientService(s storage.UserClientStorage, c storage.UserClientCache) *UserClientService {
	return &UserClientService{
		storage: s,
		cache:   c,
	}
}

func (ucs *UserClientService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	if user := ucs.cache.GetUser(ctx, id); user != nil {
		return user, nil
	}

	user, err := ucs.storage.GetUser(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrUserNotFound, "user not found")
	}

	go ucs.cache.SetUser(ctx, id, user)
	return user, nil
}

func (ucs *UserClientService) UpdateLastActivity(ctx context.Context, userID string) {
	now := time.Now().UnixNano()

	if err := ucs.storage.UpdateLastActivity(ctx, userID, now); err != nil {
		log.Printf("Failed to update activity for %s: %v", userID, err)
		return
	}

	if user, err := ucs.cache.GetUser(ctx, userID); err == nil {
		user.LastActive = now
		go ucs.cache.SetUser(ctx, userID, user)
	}
}

func (ucs *UserClientService) ValidateUserPermissions(ctx context.Context, userID string, required []string) error {
	user, err := ucs.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, requiredPerm := range required {
		hasPerm := false
		for _, userPerm := range user.Permissions {
			if userPerm == requiredPerm {
				hasPerm = true
				break
			}
		}
		if !hasPerm {
			return apperror.New(apperror.ErrConnection, fmt.Sprintf("missing permission: %s", requiredPerm))
		}
	}
	return nil
}

func (ucs *UserClientService) GetUserStatus(userID string) domain.UserStatus {
	user, err := ucs.GetUser(context.Background(), userID)
	if err != nil {
		return domain.StatusOffline
	}

	inactiveDuration := time.Since(time.Unix(0, user.LastActive))

	switch {
	case inactiveDuration < 5*time.Minute:
		return domain.StatusOnline
	case inactiveDuration < 30*time.Minute:
		return domain.StatusAway
	default:
		return domain.StatusOffline
	}
}
