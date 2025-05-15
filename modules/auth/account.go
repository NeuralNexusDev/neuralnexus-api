package auth

// AccountService - The userService interface
type AccountService interface {
	AddAccount(account *Account) error
	GetAccountByID(userID string) (*Account, error)
	GetAccountByUsername(username string) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
	UpdateAccount(account *Account) error
	DeleteAccount(userID string) error
}

// userService - The userService struct
type accountService struct {
	as AccountStore
}

// NewAccountService - Create a new userService
func NewAccountService(store Store) AccountService {
	return &accountService{store.Account()}
}

// GetAccountByID - Get an account by its ID
func (s *accountService) GetAccountByID(userID string) (*Account, error) {
	return s.as.GetAccountByID(userID)
}

// GetAccountByUsername -  Get an account by its username
func (s *accountService) GetAccountByUsername(username string) (*Account, error) {
	return s.as.GetAccountByUsername(username)
}

// GetAccountByEmail - Get an account by its email
func (s *accountService) GetAccountByEmail(email string) (*Account, error) {
	return s.as.GetAccountByEmail(email)
}

// AddAccount - Add an account to the database
func (s *accountService) AddAccount(account *Account) error {
	return s.as.AddAccountToDB(account)
}

// UpdateAccount - Update an account in the database
func (s *accountService) UpdateAccount(account *Account) error {
	return s.as.UpdateAccountInDB(account)
}

// DeleteAccount - Delete an account from the database
func (s *accountService) DeleteAccount(userID string) error {
	return s.as.DeleteAccountFromDB(userID)
}
