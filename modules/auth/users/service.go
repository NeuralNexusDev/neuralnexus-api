package users

import (
	"log"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

// Service - The service interface
type Service interface {
	GetUser(userID string) (*auth.Account, error)
	GetUserFromPlatform(platform auth.Platform, platformID string) (*auth.Account, error)
	GetUserPermissions(userID string) ([]string, error)
	UpdateUser(user *auth.Account) (*auth.Account, error)
	UpdateUserFromPlatform(platform auth.Platform, platformID string, data auth.Data) (*auth.Account, error)
	DeleteUser(userID string) error
}

// service - The service struct
type service struct {
	as  auth.AccountStore
	als auth.LinkAccountStore
}

// NewService - Create a new service
func NewService(store auth.Store) Service {
	return &service{store.Account(), store.LinkAccount()}
}

// GetUser - Get a user by their ID
func (s *service) GetUser(userID string) (*auth.Account, error) {
	return s.as.GetAccountByID(userID)
}

// GetUserFromPlatform - Get a user by their platform ID
func (s *service) GetUserFromPlatform(platform auth.Platform, platformID string) (*auth.Account, error) {
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		return nil, err
	}
	return s.as.GetAccountByID(la.UserID)
}

// GetUserPermissions - Get a user's permissions
func (s *service) GetUserPermissions(userID string) ([]string, error) {
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
func (s *service) UpdateUser(user *auth.Account) (*auth.Account, error) {
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
func (s *service) UpdateUserFromPlatform(platform auth.Platform, platformID string, data auth.Data) (*auth.Account, error) {
	// If the user doesn't exist, create a new account
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		a, err := auth.NewIDOnlyAccount()
		if err != nil {
			return nil, err
		}
		a, err = s.as.AddAccountToDB(a)
		if err != nil {
			return nil, err
		}
		la = &auth.LinkedAccount{
			UserID:        a.UserID,
			Platform:      platform,
			PlatformID:    platformID,
			Data:          data,
			DataUpdatedAt: time.Now(),
			CreatedAt:     time.Now(),
		}
		_, err = s.als.AddLinkedAccountToDB(la)
		if err != nil {
			return nil, err
		}
	}

	// Update the linked account
	la.Data = data
	_, err = s.als.UpdateLinkedAccount(la)
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
func (s *service) DeleteUser(userID string) error {
	return s.as.DeleteAccount(userID)
}
