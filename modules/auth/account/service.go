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
	AddAccount(ctx context.Context, account *Account) error
	GetAccountByID(ctx context.Context, userID string) (*Account, error)
	GetAccountByUsername(ctx context.Context, username string) (*Account, error)
	GetAccountByEmail(ctx context.Context, email string) (*Account, error)
	UpdateAccount(ctx context.Context, account *Account) error
	DeleteAccount(ctx context.Context, userID string) error
	IsPasswordAuthEnabled(ctx context.Context, userID string) (bool, error)
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
func (s *service) GetAccountByID(ctx context.Context, userID string) (*Account, error) {
	return s.as.GetAccountByID(ctx, userID)
}

// GetAccountByUsername -  Get an account by its username
func (s *service) GetAccountByUsername(ctx context.Context, username string) (*Account, error) {
	return s.as.GetAccountByUsername(ctx, username)
}

// GetAccountByEmail - Get an account by its email
func (s *service) GetAccountByEmail(ctx context.Context, email string) (*Account, error) {
	return s.as.GetAccountByEmail(ctx, email)
}

// AddAccount - Add an account to the database
func (s *service) AddAccount(ctx context.Context, account *Account) error {
	return s.as.AddAccountToDB(context.WithoutCancel(ctx), account)
}

// UpdateAccount - Update an account in the database
func (s *service) UpdateAccount(ctx context.Context, account *Account) error {
	return s.as.UpdateAccountInDB(ctx, account)
}

// DeleteAccount - Delete an account from the database
func (s *service) DeleteAccount(ctx context.Context, userID string) error {
	return s.as.DeleteAccountFromDB(ctx, userID)
}

// IsPasswordAuthEnabled - Check whether userID's password can be used to log in
func (s *service) IsPasswordAuthEnabled(ctx context.Context, userID string) (bool, error) {
	settings, err := s.ass.GetAccountSettings(ctx, userID)
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
		panic("snowflake generation failed although config.FromEnv limits SNOWFLAKE_NODE_ID and SNOWFLAKE_WORKER_ID to 31: " + err.Error())
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
