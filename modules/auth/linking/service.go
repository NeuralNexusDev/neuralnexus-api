package linking

import (
	"errors"
	"log"

	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
)

const logLinkCleanupFailed = "Failed to link account, and failed to clean up the orphaned placeholder account:\n\t"

// PlaceholderAccountStore - the account writes a new link needs when it creates a placeholder account
type PlaceholderAccountStore interface {
	AddAccountToDB(a *authaccount.Account) error
	DeleteAccountFromDB(userID string) error
}

// Service - The linking service
type Service interface {
	GetUserLinkedAccounts(userID string) ([]*LinkedAccount, error)
	UnlinkPlatform(userID string, platform Platform) error
	SetPlatformLoginEnabled(userID string, platform Platform, enabled bool) error
	UpdateFromPlatform(platform Platform, platformID string, data PlatformData) (string, error)
}

// service - The linking service struct
type service struct {
	als LinkAccountStore
	as  PlaceholderAccountStore
}

// NewService - Create a new linking service
func NewService(als LinkAccountStore, as PlaceholderAccountStore) Service {
	return &service{als, as}
}

// GetUserLinkedAccounts - List every platform linked to a user
func (s *service) GetUserLinkedAccounts(userID string) ([]*LinkedAccount, error) {
	return s.als.GetLinkedAccountsByUserID(userID)
}

// UnlinkPlatform - Unlink a platform from a user
func (s *service) UnlinkPlatform(userID string, platform Platform) error {
	return s.als.DeleteLinkedAccount(userID, platform)
}

// SetPlatformLoginEnabled - Toggle whether a linked platform can log in
func (s *service) SetPlatformLoginEnabled(userID string, platform Platform, enabled bool) error {
	return s.als.SetLinkedAccountLoginEnabled(userID, platform, enabled)
}

// UpdateFromPlatform - Update the linked account for a platform identity, creating a placeholder account when none exists, and return the ID of the user it belongs to
func (s *service) UpdateFromPlatform(platform Platform, platformID string, data PlatformData) (string, error) {
	// If the user doesn't exist, create a new account
	la, err := s.als.GetLinkedAccountByPlatformID(platform, platformID)
	if err != nil {
		if !errors.Is(err, authaccount.ErrNotFound) {
			return "", err
		}
		a, err := authaccount.NewIDOnlyAccount()
		if err != nil {
			return "", err
		}
		err = s.as.AddAccountToDB(a)
		if err != nil {
			return "", err
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
				log.Println(logLinkCleanupFailed, err)
				return "", delErr
			}
			if !errors.Is(err, ErrAlreadyLinked) {
				return "", err
			}
			// Lost the race to link this platform account: another
			// request's insert won between our lookup and our own insert.
			// Use the winner's linked account instead.
			la, err = s.als.GetLinkedAccountByPlatformID(platform, platformID)
			if err != nil {
				return "", err
			}
		}
	}

	// Update the linked account
	la.Data = data
	err = s.als.UpdateLinkedAccount(la)
	if err != nil {
		return "", err
	}
	return la.UserID, nil
}
