package ratelimit

import (
	"context"
	"errors"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/redis/go-redis/v9"
)

const rateLimitKeyPrefix = "rl:"

// Store - Rate limit store interface
type Store interface {
	GetRateLimit(ctx context.Context, key string) (int, error)
	SetRateLimit(ctx context.Context, key string, val int) error
	IncrementRateLimit(ctx context.Context, key string) error
}

type store struct {
	cfg config.RateLimit
	rdb *redis.Client
}

// NewStore - Create a new rate limit store
func NewStore(cfg config.RateLimit, rdb *redis.Client) Store {
	return &store{
		cfg: cfg,
		rdb: rdb,
	}
}

// GetRateLimit gets the rate limit for a key
func (s *store) GetRateLimit(ctx context.Context, key string) (int, error) {
	val, err := s.rdb.Get(ctx, rateLimitKeyPrefix+key).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = s.SetRateLimit(ctx, key, 1)
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
func (s *store) SetRateLimit(ctx context.Context, key string, val int) error {
	_, err := s.rdb.Set(ctx, rateLimitKeyPrefix+key, val, s.cfg.Window).Result()
	if err != nil {
		return err
	}
	return nil
}

// IncrementRateLimit increments the rate limit for a key
func (s *store) IncrementRateLimit(ctx context.Context, key string) error {
	rediskey := rateLimitKeyPrefix + key
	_, err := s.rdb.Incr(ctx, rediskey).Result()
	if err != nil {
		return err
	}
	// Only set the TTL if this increment created the key, so an existing
	// key's window doesn't reset on every request.
	_, err = s.rdb.ExpireNX(ctx, rediskey, s.cfg.Window).Result()
	if err != nil {
		return err
	}
	return nil
}
