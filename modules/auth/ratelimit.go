package auth

// RateLimitService - Rate limit service interface
// If there is no session fall back to the request's IP
type RateLimitService interface {
	GetRateLimit(key string) (int, error)
	SetRateLimit(key string, limit int) error
	IncrRateLimit(key string) error
}

// rateLimitService - Rate limit service struct
type rateLimitService struct {
	store RateLimitStore
}

// NewRateLimitService - Create a new rate limit service
func NewRateLimitService(store Store) RateLimitService {
	return &rateLimitService{
		store: store.RateLimit(),
	}
}

// GetRateLimit Get the rate limit by key
func (s *rateLimitService) GetRateLimit(key string) (int, error) {
	return s.store.GetRateLimit(key)
}

// SetRateLimit Set the rate limit for a key
func (s *rateLimitService) SetRateLimit(key string, limit int) error {
	return s.store.SetRateLimit(key, limit)
}

// IncrRateLimit Increment the rate limit for a key
func (s *rateLimitService) IncrRateLimit(key string) error {
	return s.store.IncrementRateLimit(key)
}
