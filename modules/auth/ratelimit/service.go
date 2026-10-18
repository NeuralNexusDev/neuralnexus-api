package ratelimit

// Service - Rate limit service interface
// If there is no session fall back to the request's IP
type Service interface {
	GetRateLimit(key string) (int, error)
	SetRateLimit(key string, limit int) error
	IncrRateLimit(key string) error
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
func (s *service) GetRateLimit(key string) (int, error) {
	return s.store.GetRateLimit(key)
}

// SetRateLimit Set the rate limit for a key
func (s *service) SetRateLimit(key string, limit int) error {
	return s.store.SetRateLimit(key, limit)
}

// IncrRateLimit Increment the rate limit for a key
func (s *service) IncrRateLimit(key string) error {
	return s.store.IncrementRateLimit(key)
}
