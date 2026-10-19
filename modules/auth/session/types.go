package session

import (
	"slices"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/sessionpb"
	"github.com/golang-jwt/jwt/v5"
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

// HasPermission checks if a session holds a node, bare or with any value
func (s *Session) HasPermission(permission perms.Scope) bool {
	for _, p := range s.Permissions {
		if p == permission.Node || strings.HasPrefix(p, permission.Node+":") {
			return true
		}
	}
	return false
}

// HasPermissionValue checks if a session holds a node with a given value
func (s *Session) HasPermissionValue(permission perms.Scope, value string) bool {
	return slices.Contains(s.Permissions, permission.Node+":"+value)
}

// IsValid checks if a session is expired
func (s *Session) IsValid() bool {
	if s.ExpiresAt == 0 {
		return true
	}
	return time.Now().Unix() < s.ExpiresAt
}

// SessionClaims custom JWT claims for session
type SessionClaims struct {
	Scope []string `json:"scope"`
	jwt.RegisteredClaims
}

// AsJWT creates a JWT for a session
func (s *Session) AsJWT() (string, error) {
	// ExpiresAt == 0 means the session never expires (see Session.IsValid and
	// store.AddSessionToCache). Encoding that literally would set the JWT's
	// exp claim to 1970-01-01, which ReadJWT would immediately reject as
	// expired, so omit the exp claim entirely in that case instead.
	var expiresAt *jwt.NumericDate
	if s.ExpiresAt != 0 {
		expiresAt = jwt.NewNumericDate(time.Unix(s.ExpiresAt, 0))
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, SessionClaims{
		s.Permissions,
		jwt.RegisteredClaims{
			ExpiresAt: expiresAt,
			IssuedAt:  jwt.NewNumericDate(time.Unix(s.IssuedAt, 0)),
			Issuer:    NN_API_URL,
			Subject:   s.UserID,
			Audience:  validAudiences,
			ID:        s.ID,
		},
	}).SignedString(JWT_SECRET)
}
