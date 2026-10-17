package auth

import (
	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
)

// UserService - The userService interface
// TODO: Convert to a user struct that cannot modify sensitive data
type UserService interface {
	GetUser(userID string) (*authaccount.Account, error)
	GetUserFromPlatform(platform linking.Platform, platformID string) (*authaccount.Account, error)
	ListUsers(limit, offset int) ([]*authaccount.Account, error)
	GetUserPermissions(userID string) ([]string, error)
	UpdateUser(user *authaccount.Account) error
	UpdateUserFromPlatform(platform linking.Platform, platformID string, data linking.PlatformData) (*authaccount.Account, error)
	DeleteUser(userID string) error
	GetAccountSettings(userID string) (*authaccount.AccountSettings, error)
	SetPasswordAuthEnabled(userID string, enabled bool) error
}

// userService - The userService struct
type userService struct {
	as    authaccount.AccountStore
	als   linking.LinkAccountStore
	links linking.Service
	ass   authaccount.AccountSettingsStore
	rs    authaccount.RoleStore
}

// NewUserService - Create a new userService
func NewUserService(as authaccount.AccountStore, als linking.LinkAccountStore, links linking.Service, ass authaccount.AccountSettingsStore, rs authaccount.RoleStore) UserService {
	return &userService{as, als, links, ass, rs}
}

// GetUser - Get a user by their ID
func (s *userService) GetUser(userID string) (*authaccount.Account, error) {
	return s.as.GetAccountByID(userID)
}

// GetUserFromPlatform - Get a user by their platform ID
func (s *userService) GetUserFromPlatform(platform linking.Platform, platformID string) (*authaccount.Account, error) {
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		return nil, err
	}
	return s.as.GetAccountByID(la.UserID)
}

// ListUsers - List a page of users ordered by ID
func (s *userService) ListUsers(limit, offset int) ([]*authaccount.Account, error) {
	return s.as.ListAccounts(limit, offset)
}

// GetUserPermissions - Get a user's permissions
func (s *userService) GetUserPermissions(userID string) ([]string, error) {
	a, err := s.as.GetAccountByID(userID)
	if err != nil {
		return nil, err
	}
	if len(a.Roles) == 0 {
		return []string{}, nil
	}
	permissions, err := s.rs.GetPermissionsForRoles(a.Roles)
	if err != nil {
		return nil, err
	}
	if permissions == nil {
		permissions = []string{}
	}
	return permissions, nil
}

// UpdateUser - Update a user
func (s *userService) UpdateUser(user *authaccount.Account) error {
	account, err := s.as.GetAccountByID(user.UserID)
	if err != nil {
		return err
	}
	if user.Username != "" {
		account.Username = user.Username
	}
	if user.Email != nil {
		account.Email = user.Email
	}
	if user.Roles != nil {
		account.Roles = user.Roles
	}
	return s.as.UpdateAccountInDB(account)
}

// UpdateUserFromPlatform - Update a user from a platform
func (s *userService) UpdateUserFromPlatform(platform linking.Platform, platformID string, data linking.PlatformData) (*authaccount.Account, error) {
	userID, err := s.links.UpdateFromPlatform(platform, platformID, data)
	if err != nil {
		return nil, err
	}
	return s.as.GetAccountByID(userID)
}

// DeleteUser - Delete a user
func (s *userService) DeleteUser(userID string) error {
	return s.as.DeleteAccountFromDB(userID)
}

// GetAccountSettings - Get a user's account settings
func (s *userService) GetAccountSettings(userID string) (*authaccount.AccountSettings, error) {
	return s.ass.GetAccountSettings(userID)
}

// SetPasswordAuthEnabled - Toggle whether a user's password can log in
func (s *userService) SetPasswordAuthEnabled(userID string, enabled bool) error {
	return s.ass.SetPasswordAuthEnabled(userID, enabled)
}
