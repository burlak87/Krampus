package service

import (
	"context"
	"fmt"
	userDomain "krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/types"
	"log"
	"time"
)

type UserClientStorage interface {
	SaveUserClient(ctx context.Context, user *userDomain.User) error
	GetUserClient(ctx context.Context, id string) (*userDomain.User, error)
	UpdateUserClient(ctx context.Context, user *userDomain.User) error
	UpdateLastActivity(ctx context.Context, userID string, ts int64) error
	ListUsers(ctx context.Context, limit, offset int) ([]*userDomain.User, error)
	SetAvatar(ctx context.Context, userID string, avatar string) error
}

type UserClientCache interface {
	GetUserClient(ctx context.Context, id string) (*userDomain.User, error)
	SetUserClient(ctx context.Context, id string, user *userDomain.User) error
	DeleteUserClient(ctx context.Context, id string) error
}

type UserClientService struct {
	storage UserClientStorage
	cache   UserClientCache
}

func NewUserClientService(s UserClientStorage, c UserClientCache) *UserClientService {
	return &UserClientService{
		storage: s,
		cache:   c,
	}
}

func (ucs *UserClientService) ListUsers(ctx context.Context, limit, offset int) ([]*userDomain.User, error) {
	return ucs.storage.ListUsers(ctx, limit, offset)
}

func (ucs *UserClientService) SetAvatar(ctx context.Context, userID string, avatar string) error {
	if err := ucs.storage.SetAvatar(ctx, userID, avatar); err != nil {
		return err
	}
	// drop stale cache so the new avatar is served
	ucs.cache.DeleteUserClient(ctx, userID)
	return nil
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
	idStr := types.UserIDFromInt64(user.ID).String()
	return ucs.cache.SetUserClient(ctx, idStr, user)
}

func (ucs *UserClientService) UpdateUser(ctx context.Context, user *userDomain.User) error {
	if err := ucs.storage.UpdateUserClient(ctx, user); err != nil {
		return err
	}
	idStr := types.UserIDFromInt64(user.ID).String()
	return ucs.cache.DeleteUserClient(ctx, idStr)
}
