package auth

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"os"
	"time"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/sessionpb"
	"google.golang.org/protobuf/proto"
)

//goland:noinspection GoSnakeCaseUsage
var (
	NN_API_URL     = os.Getenv("NN_API_URL")
	NN_SITE_URL    = os.Getenv("NN_SITE_URL")
	JWT_SECRET     = []byte(os.Getenv("JWT_SECRET"))
	validAudiences = []string{NN_SITE_URL, NN_API_URL}
)

func init() {
	if len(JWT_SECRET) == 0 {
		log.Fatal("JWT_SECRET environment variable must be set")
	}
}

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
	AddSession(session *Session) error
	GetSession(id string) (*Session, error)
	UpdateSession(session *Session) error
	DeleteSession(id string) error
	CreateJWT(*Session) (string, error)
	ReadJWT(token string) (*Session, error)
}

// sessionService - SessionService implementation
type sessionService struct {
	store SessionStore
}

// NewSessionService - Create a new session userService
func NewSessionService(store Store) SessionService {
	return &sessionService{
		store: store.Session(),
	}
}

// AddSession adds a session to the database and cache
func (s *sessionService) AddSession(session *Session) error {
	err := s.store.AddSessionToDB(session)
	if err != nil {
		return err
	}
	// Caching is a best-effort accelerator, not the source of truth (the DB
	// write above already succeeded), so a cache failure here doesn't fail
	// the request - it just costs a DB round trip on the next read. Log it
	// so a struggling/unreachable cache is still visible.
	if err := s.store.AddSessionToCache(session); err != nil {
		log.Println("failed to add session to cache:\n\t", err)
	}
	return nil
}

// GetSession gets a session by ID
func (s *sessionService) GetSession(id string) (*Session, error) {
	session, err := s.store.GetSessionFromCache(id)
	if err != nil {
		session, err = s.store.GetSessionFromDB(id)
		if err != nil {
			return nil, err
		}
		if err := s.store.AddSessionToCache(session); err != nil {
			log.Println("failed to re-populate session cache after a cache miss:\n\t", err)
		}
	}
	return session, nil
}

// UpdateSession updates a session
func (s *sessionService) UpdateSession(session *Session) error {
	err := s.store.UpdateSessionInDB(session)
	if err != nil {
		return err
	}
	if err := s.store.AddSessionToCache(session); err != nil {
		log.Println("failed to update session in cache:\n\t", err)
	}
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

// SessionClaims custom JWT claims for session
type SessionClaims struct {
	Scope []string `json:"scope"`
	jwt.RegisteredClaims
}

// CreateJWT creates a JWT for a session
func (s *sessionService) CreateJWT(session *Session) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, SessionClaims{
		session.Permissions,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Unix(session.ExpiresAt, 0)),
			IssuedAt:  jwt.NewNumericDate(time.Unix(session.IssuedAt, 0)),
			Issuer:    NN_API_URL,
			Subject:   session.UserID,
			Audience:  validAudiences,
			ID:        session.ID,
		},
	}).SignedString(JWT_SECRET)
}

// ReadJWT reads a JWT and returns the session
func (s *sessionService) ReadJWT(tokenStr string) (*Session, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		return JWT_SECRET, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*SessionClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Validate audience
	for _, aud := range claims.Audience {
		valid := false
		for _, validAud := range validAudiences {
			if aud == validAud {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid audience: %s", aud)
		}
	}

	// The session must still exist in the store; a deleted/logged-out
	// session must not be revivable just because its JWT hasn't expired yet.
	session, err := s.GetSession(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	if session.UserID != claims.Subject {
		return nil, errors.New("session does not match token subject")
	}

	session.LastUsedAt = time.Now().Unix()
	err = s.UpdateSession(session)
	if err != nil {
		return nil, err
	}

	return session, nil
}
