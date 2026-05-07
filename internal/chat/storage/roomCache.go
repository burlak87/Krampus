// internal/chat/storage/roomCache.go - Заполнен пустой
package storage

import (
	"context"
	"encoding/json"
	"time"

	"krampus/internal/chat/domain"
	myredis "krampus/pkg/client-database/redis"

	"github.com/redis/go-redis/v9"
)

type RoomCache struct {
	client *myredis.Client
}

func NewRoomCache(client *myredis.Client) *RoomCache {
	return &RoomCache{client: client}
}

func (c *RoomCache) GetRoom(ctx context.Context, id string) (*domain.Room, error) {
	data, err := c.client.RDB().Get(ctx, "room:"+id).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var room domain.Room
	if err := json.Unmarshal([]byte(data), &room); err != nil {
		return nil, err
	}
	return &room, nil
}

func (c *RoomCache) SetRoom(ctx context.Context, id string, room *domain.Room) error {
	data, err := json.Marshal(room)
	if err != nil {
		return err
	}
	return c.client.RDB().Set(ctx, "room:"+id, data, 10*time.Minute).Err()
}

func (c *RoomCache) DeleteRoom(ctx context.Context, id string) error {
	return c.client.RDB().Del(ctx, "room:"+id).Err()
}
