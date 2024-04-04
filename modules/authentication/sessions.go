package authentication

import (
	"time"

	"github.com/google/uuid"
)

// -------------- Structs --------------

// Session struct
type Session struct {
	ID          uuid.UUID `json:"id"`          // Session ID
	UserID      uuid.UUID `json:"user_id"`     // User ID
	Permissions []string  `json:"permissions"` // Permissions -- Roles squashed into an array
	IssuedAt    int64     `json:"iat"`         // Created at
	LastUsedAt  int64     `json:"lua"`         // Last used at
	ExpiresAt   int64     `json:"exp"`         // Expires at -- set to 0 for no expiration
}

// NewSession creates a new session
func (a *Account) NewSession(expiresAt int64) Session {
	permissions := []string{}
	for _, role := range a.Roles {
		permissions = append(permissions, role.Permissions...)
	}

	return Session{
		ID:          uuid.New(),
		UserID:      a.UserID,
		Permissions: permissions,
		IssuedAt:    time.Now().Unix(),
		LastUsedAt:  time.Now().Unix(),
		ExpiresAt:   expiresAt,
	}
}

// HasPermission checks if a session has a permission
func (s *Session) HasPermission(permission string) bool {
	for _, p := range s.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// IsExpired checks if a session is expired
func (s *Session) IsExpired() bool {
	if s.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > s.ExpiresAt
}
