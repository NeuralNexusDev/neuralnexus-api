package sess

import (
	"time"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/sessionpb"
	"google.golang.org/protobuf/proto"
)

// Session struct
type Session struct {
	ID          string   `json:"session_id" xml:"session_id" db:"session_id"`
	UserID      string   `json:"user_id" xml:"user_id" db:"user_id"`
	Permissions []string `json:"permissions" xml:"permissions" db:"permissions"`
	IssuedAt    int64    `json:"iat" xml:"iat" db:"iat"`
	LastUsedAt  int64    `json:"lua" xml:"lua" db:"lua"`
	ExpiresAt   int64    `json:"exp" xml:"exp" db:"exp"`
}

// ToProto converts a session to a protobuf message
func (s *Session) ToProto() proto.Message {
	return &sessionpb.Session{
		Id:          s.ID,
		UserId:      s.UserID,
		Permissions: s.Permissions,
		IssuedAt:    s.IssuedAt,
		LastUsedAt:  s.LastUsedAt,
		ExpiresAt:   s.ExpiresAt,
	}
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
