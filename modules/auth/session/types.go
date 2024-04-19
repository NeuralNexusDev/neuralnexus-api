package sess

import (
	"time"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/google/uuid"
)

// Session struct
type Session struct {
	ID          uuid.UUID `json:"session_id" xml:"session_id" db:"session_id"`
	UserID      uuid.UUID `json:"user_id" xml:"user_id" db:"user_id"`
	Permissions []string  `json:"permissions" xml:"permissions" db:"permissions"`
	IssuedAt    int64     `json:"iat" xml:"iat" db:"iat"`
	LastUsedAt  int64     `json:"lua" xml:"lua" db:"lua"`
	ExpiresAt   int64     `json:"exp" xml:"exp" db:"exp"`
}

// HasPermission checks if a session has a permission
func (s *Session) HasPermission(permission perms.Scope) bool {
	for _, p := range s.Permissions {
		if p == permission.Name+"|"+permission.Value {
			return true
		}
	}
	return false
}

// IsExpired checks if a session is expired
func (s *Session) IsValid() bool {
	if s.ExpiresAt == 0 {
		return true
	}
	return time.Now().Unix() < s.ExpiresAt
}
