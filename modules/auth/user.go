package auth

import (
	"log"
	"time"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

// UserService - The userService interface
type UserService interface {
	GetUser(userID string) (*Account, error)
	GetUserFromPlatform(platform Platform, platformID string) (*Account, error)
	GetUserPermissions(userID string) ([]string, error)
	UpdateUser(user *Account) (*Account, error)
	UpdateUserFromPlatform(platform Platform, platformID string, data Data) (*Account, error)
	DeleteUser(userID string) error
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
// TODO: Make this return just an error
func (s *userService) UpdateUser(user *Account) (*Account, error) {
	account, err := s.as.GetAccountByID(user.UserID)
	if err != nil {
		return nil, err
	}
	if user.Username != "" {
		account.Username = user.Username
	}
	if user.Email != "" {
		account.Email = user.Email
	}
	if user.Roles != nil {
		account.Roles = user.Roles
	}
	return s.as.UpdateAccount(account)
}

// UpdateUserFromPlatform - Update a user from a platform
func (s *userService) UpdateUserFromPlatform(platform Platform, platformID string, data Data) (*Account, error) {
	// If the user doesn't exist, create a new account
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		a, err := NewIDOnlyAccount()
		if err != nil {
			return nil, err
		}
		err = s.as.AddAccountToDB(a)
		if err != nil {
			return nil, err
		}
		la = &LinkedAccount{
			UserID:        a.UserID,
			Platform:      platform,
			PlatformID:    platformID,
			Data:          data,
			DataUpdatedAt: time.Now(),
			CreatedAt:     time.Now(),
		}
		err = s.als.AddLinkedAccountToDB(la)
		if err != nil {
			return nil, err
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
	return s.as.DeleteAccount(userID)
}
