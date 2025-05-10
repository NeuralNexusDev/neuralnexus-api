package auth

// RateLimitService - Rate limit service interface
// If there is no session fall back to the request's IP
type RateLimitService interface {
	GetRateLimitByID(userID string) (int, error)
	SetRateLimitByID(userID string, limit int) error
	IncrementRateLimitByID(userID string) error
	GetRateLimitIP(ip string) (int, error)
	SetRateLimitIP(ip string, limit int) error
	IncrementRateLimitIP(ip string) error
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

// GetRateLimitByID - Get the rate limit for a user by their ID
func (s *rateLimitService) GetRateLimitByID(userID string) (int, error) {
	return s.store.GetRateLimit(userID)
}

// SetRateLimitByID - Set the rate limit for a user by their ID
func (s *rateLimitService) SetRateLimitByID(userID string, limit int) error {
	return s.store.SetRateLimit(userID, limit)
}

// IncrementRateLimitByID - Increment the rate limit for a user by their ID
func (s *rateLimitService) IncrementRateLimitByID(userID string) error {
	return s.store.IncrementRateLimit(userID)
}

// GetRateLimitIP - Get the rate limit for a user by their IP
func (s *rateLimitService) GetRateLimitIP(ip string) (int, error) {
	return s.store.GetRateLimit(ip)
}

// SetRateLimitIP - Set the rate limit for a user by their IP
func (s *rateLimitService) SetRateLimitIP(ip string, limit int) error {
	return s.store.SetRateLimit(ip, limit)
}

// IncrementRateLimitIP - Increment the rate limit for a user by their IP
func (s *rateLimitService) IncrementRateLimitIP(ip string) error {
	return s.store.IncrementRateLimit(ip)
}
