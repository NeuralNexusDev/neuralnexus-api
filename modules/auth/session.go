package auth

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

// IsValid checks if a session is expired
func (s *Session) IsValid() bool {
	if s.ExpiresAt == 0 {
		return true
	}
	return time.Now().Unix() < s.ExpiresAt
}

// ----------------- Service -----------------

// SessionService interface
type SessionService interface {
	GetSession(id string) (*Session, error)
	UpdateSession(session *Session) error
	DeleteSession(id string) error
}

// sessionService - SessionService implementation
type sessionService struct {
	store SessionStore
}

// NewSessionService - Create a new session service
func NewSessionService(store Store) SessionService {
	return &sessionService{
		store: store.Session(),
	}
}

// GetSession gets a session by ID
func (s *sessionService) GetSession(id string) (*Session, error) {
	session, err := s.store.GetSessionFromCache(id)
	if err != nil {
		session, err = s.store.GetSessionFromDB(id)
		if err != nil {
			return nil, err
		}
		s.store.AddSessionToCache(session)
	}
	return session, nil
}

// UpdateSession updates a session
func (s *sessionService) UpdateSession(session *Session) error {
	err := s.store.UpdateSessionInDB(session)
	if err != nil {
		return err
	}
	s.store.AddSessionToCache(session)
	return nil
}

// DeleteSession deletes a session by ID
func (s *sessionService) DeleteSession(id string) error {
	err := s.store.DeleteSessionInDB(id)
	if err != nil {
		return err
	}
	s.store.DeleteSessionFromCache(id)
	return nil
}
