package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	chatDomain "krampus/internal/chat/domain"
	"krampus/internal/message/domain"
	userDomain "krampus/internal/user/domain"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(addr, password string, db int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{client: client}, nil
}

// CacheRecentMessage — LRU 100 последних (pipeline)
func (r *RedisStorage) CacheRecentMessage(ctx context.Context, roomID string, msg *domain.BaseMessage) error {
	key := "recent_messages:" + roomID
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.LPush(ctx, key, msgJSON)
	pipe.LTrim(ctx, key, 0, 99) // топ 100
	pipe.Expire(ctx, key, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

// GetRecentMessages — быстрый кэш для новых клиентов
func (r *RedisStorage) GetRecentMessages(ctx context.Context, roomID string) ([]*domain.BaseMessage, error) {
	key := "recent_messages:" + roomID
	msgsJSON, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cached messages: %w", err)
	}

	var messages []*domain.BaseMessage
	for _, msgJSON := range msgsJSON {
		var msg domain.BaseMessage
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			continue // пропускаем битые
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

// 👥 User Connections (активные сессии)
func (r *RedisStorage) SetUserConnection(ctx context.Context, userID string, conn *chatDomain.UserConnection) error {
	key := "user_connections:" + userID
	connJSON, err := json.Marshal(conn)
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}

	if err := r.client.HSet(ctx, key, conn.ConnID, connJSON).Err(); err != nil {
		return fmt.Errorf("failed to set user connection: %w", err)
	}
	return r.client.Expire(ctx, key, 30*time.Minute).Err()
}

func (r *RedisStorage) RemoveUserConnection(ctx context.Context, userID, connID string) error {
	key := "user_connections:" + userID
	return r.client.HDel(ctx, key, connID).Err()
}

// Room/User Cache
func (r *RedisStorage) GetRoom(ctx context.Context, id string) (*chatDomain.Room, error) {
	key := "room:" + id
	roomJSON, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room from cache: %w", err)
	}

	var room chatDomain.Room
	if err := json.Unmarshal([]byte(roomJSON), &room); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room: %w", err)
	}
	return &room, nil
}

func (r *RedisStorage) SetRoom(ctx context.Context, id string, room *chatDomain.Room) error {
	key := "room:" + id
	roomJSON, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("failed to marshal room: %w", err)
	}
	return r.client.Set(ctx, key, roomJSON, 10*time.Minute).Err()
}

func (r *RedisStorage) GetAccessToken(token string) (*userDomain.CachedSession, error) {
	key := "access_token:" + token
	data, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}

	var session userDomain.CachedSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *RedisStorage) SetAccessToken(token string, session userDomain.CachedSession) error {
	key := "access_token:" + token
	data, _ := json.Marshal(session)
	expiration := time.Until(session.ExpiresAt)
	return r.client.Set(context.Background(), key, data, expiration).Err()
}
