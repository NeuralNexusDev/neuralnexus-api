package account

import (
	"time"

	authsession "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

// RoleStore - The source of the permissions that roles grant
type RoleStore interface {
	GetPermissionsForRoles(roleIDs []string) ([]string, error)
}

// AccountService - The account service interface
type AccountService interface {
	AddAccount(account *Account) error
	GetAccountByID(userID string) (*Account, error)
	GetAccountByUsername(username string) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
	UpdateAccount(account *Account) error
	DeleteAccount(userID string) error
	IsPasswordAuthEnabled(userID string) (bool, error)
	NewSession(account *Account, expiresAt int64) (*authsession.Session, error)
}

// accountService - The accountService struct
type accountService struct {
	as  AccountStore
	ass AccountSettingsStore
	rs  RoleStore
}

// NewAccountService - Create a new AccountService
func NewAccountService(as AccountStore, ass AccountSettingsStore, rs RoleStore) AccountService {
	return &accountService{as, ass, rs}
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

// IsPasswordAuthEnabled - Check whether userID's password can be used to log in
func (s *accountService) IsPasswordAuthEnabled(userID string) (bool, error) {
	settings, err := s.ass.GetAccountSettings(userID)
	if err != nil {
		return false, err
	}
	return settings.PasswordAuthEnabled, nil
}

// NewSession - Create a session for an account with the permissions of its roles
func (s *accountService) NewSession(account *Account, expiresAt int64) (*authsession.Session, error) {
	permissions := []string{}
	if len(account.Roles) > 0 {
		var err error
		if permissions, err = s.rs.GetPermissionsForRoles(account.Roles); err != nil {
			return nil, err
		}
		if permissions == nil {
			permissions = []string{}
		}
	}
	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	return &authsession.Session{
		ID:          id,
		UserID:      account.UserID,
		Permissions: permissions,
		IssuedAt:    now,
		LastUsedAt:  now,
		ExpiresAt:   expiresAt,
	}, nil
}
