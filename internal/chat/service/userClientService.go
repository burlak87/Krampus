package service

import (
	"context"
	"fmt"
	"krampus/internal/chat/storage"
	userDomain "krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"log"
	"os/user"
	"time"
)

type UserClientStorage interface {
	SaveUserClient(ctx context.Context, user *user.User) error
	GetUserClient(ctx context.Context, id string) (*user.User, error)
	UpdateUserClient(ctx context.Context, user *user.User) error
	UpdateLastActivity(ctx context.Context, userID string, ts int64) error
}

type UserClientCache interface {
	GetUserClient(ctx context.Context, id string) (*user.User, error)
	SetUserClient(ctx context.Context, id string, user *user.User) error
	DeleteUserClient(ctx context.Context, id string) error
}

type UserClientService struct {
	storage *storage.UserClientPGStorage
	cache   *storage.UserClientCache
}

func NewUserClientService(s *storage.UserClientPGStorage, c *storage.UserClientCache) *UserClientService {
	return &UserClientService{
		storage: s,
		cache:   c,
	}
}

func (ucs *UserClientService) GetUser(ctx context.Context, id string) (*userDomain.User, error) {
	if user, err := ucs.cache.GetUserClient(ctx, id); err == nil && user != nil {
		return user, nil
	}

	user, err := ucs.storage.GetUserClient(ctx, id)
	if err != nil {
		return nil, apperror.New(apperror.ErrUserNotFound, "user not found")
	}

	go ucs.cache.SetUserClient(ctx, id, user)
	return user, nil
}

func (ucs *UserClientService) UpdateLastActivity(ctx context.Context, userID string) {
	now := time.Now().UnixNano()

	if err := ucs.storage.UpdateLastActivity(ctx, userID, now); err != nil {
		log.Printf("Failed to update activity for %s: %v", userID, err)
		return
	}

	if user, err := ucs.cache.GetUserClient(ctx, userID); err == nil {
		user.LastActive = now
		go ucs.cache.SetUserClient(ctx, userID, user)
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

func (ucs *UserClientService) GetUserStatus(userID string) userDomain.ChatUserStatus {
	user, err := ucs.GetUser(context.Background(), userID)
	if err != nil {
		return userDomain.StatusOffline
	}

	inactiveDuration := time.Since(time.Unix(0, user.LastActive))

	switch {
	case inactiveDuration < 5*time.Minute:
		return userDomain.StatusOnline
	case inactiveDuration < 30*time.Minute:
		return userDomain.StatusAway
	default:
		return userDomain.StatusOffline
	}
}

func (ucs *UserClientService) SaveUser(ctx context.Context, user *userDomain.User) error {
	if err := ucs.storage.SaveUserClient(ctx, user); err != nil {
		return err
	}
	idStr := string(user.ID)
	return ucs.cache.SetUserClient(ctx, idStr, user)
}

func (ucs *UserClientService) UpdateUser(ctx context.Context, user *userDomain.User) error {
	if err := ucs.storage.UpdateUserClient(ctx, user); err != nil {
		return err
	}
	idStr := string(user.ID)
	return ucs.cache.DeleteUserClient(ctx, idStr)
}
