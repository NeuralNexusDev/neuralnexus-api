package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const rateLimitKeyPrefix = "rl:"

// Store - Rate limit store interface
type Store interface {
	GetRateLimit(key string) (int, error)
	SetRateLimit(key string, val int) error
	IncrementRateLimit(key string) error
}

type store struct {
	rdb *redis.Client
}

// NewStore - Create a new rate limit store
func NewStore(rdb *redis.Client) Store {
	return &store{
		rdb: rdb,
	}
}

// GetRateLimit gets the rate limit for a key
func (s *store) GetRateLimit(key string) (int, error) {
	val, err := s.rdb.Get(context.Background(), rateLimitKeyPrefix+key).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = s.SetRateLimit(key, 1)
			if err != nil {
				return 0, err
			}
			return 1, nil
		}
		return 0, err
	}
	return val, nil
}

// SetRateLimit sets the rate limit for a key
func (s *store) SetRateLimit(key string, val int) error {
	_, err := s.rdb.Set(context.Background(), rateLimitKeyPrefix+key, val, time.Minute).Result()
	if err != nil {
		return err
	}
	return nil
}

// IncrementRateLimit increments the rate limit for a key
func (s *store) IncrementRateLimit(key string) error {
	rediskey := rateLimitKeyPrefix + key
	_, err := s.rdb.Incr(context.Background(), rediskey).Result()
	if err != nil {
		return err
	}
	// Only set the TTL if this increment created the key, so an existing
	// key's window doesn't reset on every request.
	_, err = s.rdb.ExpireNX(context.Background(), rediskey, time.Minute).Result()
	if err != nil {
		return err
	}
	return nil
}
