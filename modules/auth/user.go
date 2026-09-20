package auth

import (
	"errors"
	"fmt"
	"log"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

// UserService - The userService interface
// TODO: Convert to a user struct that cannot modify sensitive data
type UserService interface {
	GetUser(userID string) (*Account, error)
	GetUserFromPlatform(platform Platform, platformID string) (*Account, error)
	GetUserPermissions(userID string) ([]string, error)
	UpdateUser(user *Account) error
	UpdateUserFromPlatform(platform Platform, platformID string, data PlatformData) (*Account, error)
	DeleteUser(userID string) error
	GetUserLinkedAccounts(userID string) ([]*LinkedAccount, error)
	UnlinkPlatform(userID string, platform Platform) error
	SetPlatformLoginEnabled(userID string, platform Platform, enabled bool) error
}

// userService - The userService struct
type userService struct {
	as  AccountStore
	als LinkAccountStore
}

// NewUserService - Create a new userService
func NewUserService(store Store) UserService {
	return &userService{store.Account(), store.LinkAccount()}
}

// GetUser - Get a user by their ID
func (s *userService) GetUser(userID string) (*Account, error) {
	return s.as.GetAccountByID(userID)
}

// GetUserFromPlatform - Get a user by their platform ID
func (s *userService) GetUserFromPlatform(platform Platform, platformID string) (*Account, error) {
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		return nil, err
	}
	return s.as.GetAccountByID(la.UserID)
}

// GetUserPermissions - Get a user's permissions
func (s *userService) GetUserPermissions(userID string) ([]string, error) {
	a, err := s.as.GetAccountByID(userID)
	if err != nil {
		return nil, err
	}
	var permissions []string
	for _, r := range a.Roles {
		role, err := perms.GetRoleByName(r)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, p := range role.Permissions {
			permissions = append(permissions, p.Name+"|"+p.Value)
		}
	}
	return permissions, nil
}

// UpdateUser - Update a user
func (s *userService) UpdateUser(user *Account) error {
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
func (s *userService) UpdateUserFromPlatform(platform Platform, platformID string, data PlatformData) (*Account, error) {
	// If the user doesn't exist, create a new account
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		a, err := NewIDOnlyAccount()
		if err != nil {
			return nil, err
		}
		err = s.as.AddAccountToDB(a)
		if err != nil {
			return nil, err
		}
		var username string
		if data != nil {
			username = data.GetUsername()
		}
		la = NewLinkedAccount(a.UserID, platform, username, platformID, data)
		err = s.als.AddLinkedAccountToDB(la)
		if err != nil {
			// Whatever went wrong, the placeholder account created above is
			// now orphaned - clean it up before deciding how to handle err.
			if delErr := s.as.DeleteAccountFromDB(a.UserID); delErr != nil {
				return nil, fmt.Errorf("failed to link account (%w) and failed to clean up the orphaned placeholder account: %w", err, delErr)
			}
			if !errors.Is(err, ErrAlreadyLinked) {
				return nil, err
			}
			// Lost the race to link this platform account: another
			// request's insert won between our lookup and our own insert.
			// Use the winner's linked account instead.
			la, err = s.als.GetLinkedAccountByPlatformID(platform, platformID)
			if err != nil {
				return nil, err
			}
		}
	}

	// Update the linked account
	la.Data = data
	err = s.als.UpdateLinkedAccount(la)
	if err != nil {
		return nil, err
	}

	a, err := s.as.GetAccountByID(la.UserID)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// DeleteUser - Delete a user
func (s *userService) DeleteUser(userID string) error {
	return s.as.DeleteAccountFromDB(userID)
}

// GetUserLinkedAccounts - List every platform linked to a user
func (s *userService) GetUserLinkedAccounts(userID string) ([]*LinkedAccount, error) {
	return s.als.GetLinkedAccountsByUserID(userID)
}

// UnlinkPlatform - Unlink a platform from a user
func (s *userService) UnlinkPlatform(userID string, platform Platform) error {
	return s.als.DeleteLinkedAccount(userID, platform)
}

// SetPlatformLoginEnabled - Toggle whether a linked platform can log in
func (s *userService) SetPlatformLoginEnabled(userID string, platform Platform, enabled bool) error {
	return s.als.SetLinkedAccountLoginEnabled(userID, platform, enabled)
}
