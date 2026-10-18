package ratelimit

import "context"

// Service - Rate limit service interface
// If there is no session fall back to the request's IP
type Service interface {
	GetRateLimit(ctx context.Context, key string) (int, error)
	SetRateLimit(ctx context.Context, key string, limit int) error
	IncrRateLimit(ctx context.Context, key string) error
}

type service struct {
	store Store
}

// NewService - Create a new rate limit service
func NewService(store Store) Service {
	return &service{
		store: store,
	}
}

// GetRateLimit Get the rate limit by key
func (s *service) GetRateLimit(ctx context.Context, key string) (int, error) {
	return s.store.GetRateLimit(ctx, key)
}

// SetRateLimit Set the rate limit for a key
func (s *service) SetRateLimit(ctx context.Context, key string, limit int) error {
	return s.store.SetRateLimit(ctx, key, limit)
}

// IncrRateLimit Increment the rate limit for a key
func (s *service) IncrRateLimit(ctx context.Context, key string) error {
	return s.store.IncrementRateLimit(context.WithoutCancel(ctx), key)
}
