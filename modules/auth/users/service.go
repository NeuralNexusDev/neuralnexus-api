package users

import (
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	accountlinking "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
)

// Service - The service interface
type Service interface {
	GetUser(userID string) (*auth.Account, error)
	GetUserFromPlatform(platform accountlinking.Platform, platformID string) (*auth.Account, error)
	UpdateUser(user *auth.Account) (*auth.Account, error)
	UpdateUserFromPlatform(platform accountlinking.Platform, platformID string, data accountlinking.Data) (*auth.Account, error)
	DeleteUser(userID string) error
}

// service - The service struct
type service struct {
	as  auth.AccountStore
	als accountlinking.LinkAccountStore
}

// NewService - Create a new service
func NewService(as auth.AccountStore, als accountlinking.LinkAccountStore) Service {
	return &service{as, als}
}

// GetUser - Get a user by their ID
func (s *service) GetUser(userID string) (*auth.Account, error) {
	return s.as.GetAccountByID(userID)
}

// GetUserFromPlatform - Get a user by their platform ID
func (s *service) GetUserFromPlatform(platform accountlinking.Platform, platformID string) (*auth.Account, error) {
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		return nil, err
	}
	return s.as.GetAccountByID(la.UserID)
}

// UpdateUser - Update a user
func (s *service) UpdateUser(user *auth.Account) (*auth.Account, error) {
	return s.as.UpdateAccount(user)
}

// UpdateUserFromPlatform - Update a user from a platform
func (s *service) UpdateUserFromPlatform(platform accountlinking.Platform, platformID string, data accountlinking.Data) (*auth.Account, error) {
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
		la = &accountlinking.LinkedAccount{
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
