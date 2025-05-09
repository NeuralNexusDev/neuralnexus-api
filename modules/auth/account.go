package auth

// AccountService - The userService interface
type AccountService interface {
	GetAccountByID(userID string) (*Account, error)
	GetAccountByUsername(username string) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
}

// userService - The userService struct
type accountService struct {
	as AccountStore
}

// NewAccountService - Create a new userService
func NewAccountService(store Store) AccountService {
	return &accountService{store.Account()}
}

// GetAccountByID - Get a user by their ID
func (s *accountService) GetAccountByID(userID string) (*Account, error) {
	return s.as.GetAccountByID(userID)
}

// GetAccountByUsername - Get a user by their username
func (s *accountService) GetAccountByUsername(username string) (*Account, error) {
	return s.as.GetAccountByUsername(username)
}

// GetAccountByEmail - Get a user by their email
func (s *accountService) GetAccountByEmail(email string) (*Account, error) {
	return s.as.GetAccountByEmail(email)
}
