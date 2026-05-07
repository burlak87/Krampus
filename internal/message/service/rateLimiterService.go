package service

import (
	"context"
	"krampus/internal/message/domain"
	"krampus/pkg/apperror"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastUsed map[string]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		lastUsed: make(map[string]time.Time),
	}
}

func (rl *RateLimiter) Check(ctx context.Context, userID string, msgType domain.MessageType) error {
	key := userID + ":" + string(msgType)

	rl.mu.Lock()
	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rl.createLimiter(msgType)
		rl.limiters[key] = limiter
	}
	rl.lastUsed[key] = time.Now()
	rl.mu.Unlock()

	if !limiter.Allow() {
		return apperror.New(apperror.ErrRateLimit, "rate exceeded")
	}

	return nil
}

func (rl *RateLimiter) createLimiter(msgType domain.MessageType) *rate.Limiter {
	switch msgType {
	case domain.TypeText:
		return rate.NewLimiter(rate.Limit(10), 10)
	case domain.TypeCommand:
		return rate.NewLimiter(rate.Limit(2), 2)
	case domain.TypeTyping:
		return rate.NewLimiter(rate.Limit(5), 5)
	case domain.TypeFile:
		return rate.NewLimiter(rate.Limit(1), 3)
	default:
		return rate.NewLimiter(rate.Limit(50), 50)
	}
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key := range rl.limiters {
		if time.Since(rl.lastUsed[key]) > 10*time.Minute {
			delete(rl.limiters, key)
			delete(rl.lastUsed, key)
		}
	}
}
