package user

import (
	"context"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/link"
)

// Service is the user layer over the account, link and settings services and the role lookup.
type Service interface {
	GetUser(userID string) (*User, error)
	GetUserFromPlatform(platform link.Platform, platformID string) (*User, error)
	ListUsers(limit, offset int) ([]*User, error)
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
	UpdateUser(userID string, update *UserUpdate) error
	UpdateUserFromPlatform(platform link.Platform, platformID string, data link.PlatformData) (*User, error)
	DeleteUser(userID string) error
	GetAccountSettings(userID string) (*account.AccountSettings, error)
	SetPasswordAuthEnabled(userID string, enabled bool) error
}

type service struct {
	as    account.Store
	als   link.Store
	links link.Service
	ass   account.SettingsStore
	rs    account.RoleStore
}

// NewService - Create a new user Service
func NewService(as account.Store, als link.Store, links link.Service, ass account.SettingsStore, rs account.RoleStore) Service {
	return &service{as, als, links, ass, rs}
}

func userFromAccount(a *account.Account) *User {
	if a == nil {
		return nil
	}
	return &User{UserID: a.UserID, Username: a.Username, Roles: a.Roles, UpdatedAt: a.UpdatedAt}
}

// GetUser - Get a user by their ID
func (s *service) GetUser(userID string) (*User, error) {
	a, err := s.as.GetAccountByID(context.TODO(), userID)
	if err != nil {
		return nil, err
	}
	return userFromAccount(a), nil
}

// GetUserFromPlatform - Get a user by their platform ID
func (s *service) GetUserFromPlatform(platform link.Platform, platformID string) (*User, error) {
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		return nil, err
	}
	a, err := s.as.GetAccountByID(context.TODO(), la.UserID)
	if err != nil {
		return nil, err
	}
	return userFromAccount(a), nil
}

// ListUsers - List a page of users ordered by ID
func (s *service) ListUsers(limit, offset int) ([]*User, error) {
	accounts, err := s.as.ListAccounts(context.TODO(), limit, offset)
	if err != nil {
		return nil, err
	}
	var users []*User
	for _, a := range accounts {
		users = append(users, userFromAccount(a))
	}
	return users, nil
}

// GetUserPermissions - Get a user's permissions
func (s *service) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	a, err := s.as.GetAccountByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(a.Roles) == 0 {
		return []string{}, nil
	}
	permissions, err := s.rs.GetPermissionsForRoles(ctx, a.Roles)
	if err != nil {
		return nil, err
	}
	if permissions == nil {
		permissions = []string{}
	}
	return permissions, nil
}

// UpdateUser - Update a user's username and roles
func (s *service) UpdateUser(userID string, update *UserUpdate) error {
	account, err := s.as.GetAccountByID(context.TODO(), userID)
	if err != nil {
		return err
	}
	if update.Username != "" {
		account.Username = update.Username
	}
	if update.Roles != nil {
		account.Roles = update.Roles
	}
	return s.as.UpdateAccountInDB(context.TODO(), account)
}

// UpdateUserFromPlatform - Update a user from a platform
func (s *service) UpdateUserFromPlatform(platform link.Platform, platformID string, data link.PlatformData) (*User, error) {
	userID, err := s.links.UpdateFromPlatform(platform, platformID, data)
	if err != nil {
		return nil, err
	}
	a, err := s.as.GetAccountByID(context.TODO(), userID)
	if err != nil {
		return nil, err
	}
	return userFromAccount(a), nil
}

// DeleteUser - Delete a user
func (s *service) DeleteUser(userID string) error {
	return s.as.DeleteAccountFromDB(context.TODO(), userID)
}

// GetAccountSettings - Get a user's account settings
func (s *service) GetAccountSettings(userID string) (*account.AccountSettings, error) {
	return s.ass.GetAccountSettings(context.TODO(), userID)
}

// SetPasswordAuthEnabled - Toggle whether a user's password can log in
func (s *service) SetPasswordAuthEnabled(userID string, enabled bool) error {
	return s.ass.SetPasswordAuthEnabled(context.TODO(), userID, enabled)
}
