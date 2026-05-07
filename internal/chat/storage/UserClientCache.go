package storage

import (
	"context"
	"encoding/json"
	"time"

	"krampus/internal/user/domain"
	myredis "krampus/pkg/client-database/redis"

	"github.com/redis/go-redis/v9"
)

type UserClientCache struct {
	client *myredis.Client
}

func NewUserClientCache(client *myredis.Client) *UserClientCache {
	return &UserClientCache{client: client}
}

func (c *UserClientCache) GetUserClient(ctx context.Context, id string) (*domain.User, error) {
	data, err := c.client.RDB().Get(ctx, "user_client:"+id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var user domain.User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *UserClientCache) SetUserClient(ctx context.Context, id string, user *domain.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return nil
	}
	return c.client.RDB().Set(ctx, "user_client:"+id, data, 5*time.Minute).Err()
}

func (c *UserClientCache) DeleteUserClient(ctx context.Context, id string) error {
	return c.client.RDB().Del(ctx, "user_client:"+id).Err()
}
