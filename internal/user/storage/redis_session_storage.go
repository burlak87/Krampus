package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"krampus/internal/user/domain"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSessionStorage struct {
	client *redis.Client
}

func NewRedisSessionStorage(client *redis.Client) *RedisSessionStorage {
	return &RedisSessionStorage{
		client: client,
	}
}

func (s *RedisSessionStorage) SetAccessToken(token string, session domain.CachedSession) error {
	key := fmt.Sprintf("access: %s", token)

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *RedisSessionStorage) GetAccessToken(token string) (domain.CachedSession, error) {
	key := fmt.Sprintf("access: %s", token)

	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedSession{}, fmt.Errorf("token not found")
	}
	if err != nil {
		return domain.CachedSession{}, fmt.Errorf("redis get: %w", err)
	}

	var session domain.CachedSession
	if err = json.Unmarshal([]byte(data), &session); err != nil {
		return domain.CachedSession{}, fmt.Errorf("json unmarshal: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		s.client.Del(context.Background(), key)
		return domain.CachedSession{}, fmt.Errorf("token expired")
	}

	return session, nil
}

func (s *RedisSessionStorage) DeleteAccessToken(token string) error {
	key := fmt.Sprintf("access: %s", token)
	return s.client.Del(context.Background(), key).Err()
}

func (s *RedisSessionStorage) GetSessionByUserID(userID int64) (domain.CachedTempToken, error) {
	key := fmt.Sprintf("session:%d", userID)
	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedTempToken{}, fmt.Errorf("temp token not found")
	}
	if err != nil {
		return domain.CachedTempToken{}, err
	}

	var tempToken domain.CachedTempToken
	json.Unmarshal([]byte(data), &tempToken)
	return tempToken, nil
}

func (s *RedisSessionStorage) DeleteSessionByUserID(userID int64) error {
	key := fmt.Sprintf("session:%d", userID)
	return s.client.Del(context.Background(), key).Err()
}

func (s *RedisSessionStorage) SetSessionByUserID(userID int64, session domain.CachedSession) error {
	key := fmt.Sprintf("session:%d", userID)
	data, _ := json.Marshal(session)
	return s.client.Set(context.Background(), key, data, 7*24*time.Hour).Err()
}

func (s *RedisSessionStorage) SetTempToken(token string, temp domain.CachedTempToken) error {
	key := fmt.Sprintf("temp:%s", token)
	data, _ := json.Marshal(temp)
	ttl := time.Until(temp.ExpiresAt)
	return s.client.Set(context.Background(), key, data, ttl).Err()
}

func (s *RedisSessionStorage) GetTempToken(token string) (domain.CachedTempToken, error) {
	key := fmt.Sprintf("temp:%s", token)
	data, err := s.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return domain.CachedTempToken{}, fmt.Errorf("temp token not found")
	}
	if err != nil {
		return domain.CachedTempToken{}, fmt.Errorf("redis get temp: %w", err)
	}

	var tempToken domain.CachedTempToken
	if err := json.Unmarshal([]byte(data), &tempToken); err != nil {
		return domain.CachedTempToken{}, fmt.Errorf("json unmarshal temp: %w", err)
	}

	return tempToken, nil
}

func (r *RedisSessionStorage) ValidateSession(ctx context.Context, sessionID string, userID string) error {
	session, err := r.GetAccessToken(sessionID)

	if err != nil {
		return err
	}

	if session.UserID == 0 {
		return errors.New("session not found")
	}

	if strconv.FormatInt(session.UserID, 10) != userID {
		return errors.New("invalid session owner")
	}

	return nil
}

func (s *RedisSessionStorage) RedisSessionStorage() {
	// Оставляем пустым, если реальная логика не требуется,
	// но метод нужен для реализации интерфейса
}
