package user

import (
	"context"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/link"
)

// Service is the user layer over the account, link and settings services and the role lookup.
type Service interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserFromPlatform(ctx context.Context, platform link.Platform, platformID string) (*User, error)
	ListUsers(ctx context.Context, limit, offset int) ([]*User, error)
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
	UpdateUser(ctx context.Context, userID string, update *UserUpdate) error
	UpdateUserFromPlatform(ctx context.Context, platform link.Platform, platformID string, data link.PlatformData) (*User, error)
	DeleteUser(ctx context.Context, userID string) error
	GetAccountSettings(ctx context.Context, userID string) (*account.AccountSettings, error)
	SetPasswordAuthEnabled(ctx context.Context, userID string, enabled bool) error
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
func (s *service) GetUser(ctx context.Context, userID string) (*User, error) {
	a, err := s.as.GetAccountByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return userFromAccount(a), nil
}

// GetUserFromPlatform - Get a user by their platform ID
func (s *service) GetUserFromPlatform(ctx context.Context, platform link.Platform, platformID string) (*User, error) {
	la, err := s.als.GetLinkedAccountByPlatformID(ctx, platform, platformID)
	if err != nil {
		return nil, err
	}
	a, err := s.as.GetAccountByID(ctx, la.UserID)
	if err != nil {
		return nil, err
	}
	return userFromAccount(a), nil
}

// ListUsers - List a page of users ordered by ID
func (s *service) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	accounts, err := s.as.ListAccounts(ctx, limit, offset)
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
func (s *service) UpdateUser(ctx context.Context, userID string, update *UserUpdate) error {
	account, err := s.as.GetAccountByID(ctx, userID)
	if err != nil {
		return err
	}
	if update.Username != "" {
		account.Username = update.Username
	}
	if update.Roles != nil {
		account.Roles = update.Roles
	}
	return s.as.UpdateAccountInDB(ctx, account)
}

// UpdateUserFromPlatform - Update a user from a platform
func (s *service) UpdateUserFromPlatform(ctx context.Context, platform link.Platform, platformID string, data link.PlatformData) (*User, error) {
	userID, err := s.links.UpdateFromPlatform(ctx, platform, platformID, data)
	if err != nil {
		return nil, err
	}
	a, err := s.as.GetAccountByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return userFromAccount(a), nil
}

// DeleteUser - Delete a user
func (s *service) DeleteUser(ctx context.Context, userID string) error {
	return s.as.DeleteAccountFromDB(ctx, userID)
}

// GetAccountSettings - Get a user's account settings
func (s *service) GetAccountSettings(ctx context.Context, userID string) (*account.AccountSettings, error) {
	return s.ass.GetAccountSettings(ctx, userID)
}

// SetPasswordAuthEnabled - Toggle whether a user's password can log in
func (s *service) SetPasswordAuthEnabled(ctx context.Context, userID string, enabled bool) error {
	return s.ass.SetPasswordAuthEnabled(ctx, userID, enabled)
}
