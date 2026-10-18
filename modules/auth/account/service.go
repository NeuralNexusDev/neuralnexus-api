package account

import (
	"context"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

// RoleStore - The source of the permissions that roles grant
type RoleStore interface {
	GetPermissionsForRoles(ctx context.Context, roleIDs []string) ([]string, error)
}

// Service - The account service interface
type Service interface {
	AddAccount(account *Account) error
	GetAccountByID(userID string) (*Account, error)
	GetAccountByUsername(username string) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
	UpdateAccount(account *Account) error
	DeleteAccount(userID string) error
	IsPasswordAuthEnabled(userID string) (bool, error)
	NewSession(ctx context.Context, account *Account, expiresAt int64) (*session.Session, error)
}

// service - The account service struct
type service struct {
	as  Store
	ass SettingsStore
	rs  RoleStore
}

// NewService - Create a new account service
func NewService(as Store, ass SettingsStore, rs RoleStore) Service {
	return &service{as, ass, rs}
}

// GetAccountByID - Get an account by its ID
func (s *service) GetAccountByID(userID string) (*Account, error) {
	return s.as.GetAccountByID(userID)
}

// GetAccountByUsername -  Get an account by its username
func (s *service) GetAccountByUsername(username string) (*Account, error) {
	return s.as.GetAccountByUsername(username)
}

// GetAccountByEmail - Get an account by its email
func (s *service) GetAccountByEmail(email string) (*Account, error) {
	return s.as.GetAccountByEmail(email)
}

// AddAccount - Add an account to the database
func (s *service) AddAccount(account *Account) error {
	return s.as.AddAccountToDB(account)
}

// UpdateAccount - Update an account in the database
func (s *service) UpdateAccount(account *Account) error {
	return s.as.UpdateAccountInDB(account)
}

// DeleteAccount - Delete an account from the database
func (s *service) DeleteAccount(userID string) error {
	return s.as.DeleteAccountFromDB(userID)
}

// IsPasswordAuthEnabled - Check whether userID's password can be used to log in
func (s *service) IsPasswordAuthEnabled(userID string) (bool, error) {
	settings, err := s.ass.GetAccountSettings(userID)
	if err != nil {
		return false, err
	}
	return settings.PasswordAuthEnabled, nil
}

// NewSession - Create a session for an account with the permissions of its roles
func (s *service) NewSession(ctx context.Context, account *Account, expiresAt int64) (*session.Session, error) {
	permissions := []string{}
	if len(account.Roles) > 0 {
		var err error
		if permissions, err = s.rs.GetPermissionsForRoles(ctx, account.Roles); err != nil {
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
	return &session.Session{
		ID:          id,
		UserID:      account.UserID,
		Permissions: permissions,
		IssuedAt:    now,
		LastUsedAt:  now,
		ExpiresAt:   expiresAt,
	}, nil
}
