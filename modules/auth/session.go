package auth

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
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
	s.store.AddSessionToCache(session)
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

	if claims, ok := token.Claims.(*SessionClaims); ok {
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

		// Populate session
		session := &Session{
			ID:          claims.ID,
			UserID:      claims.Subject,
			Permissions: claims.Scope,
			IssuedAt:    claims.IssuedAt.Unix(),
			LastUsedAt:  time.Now().Unix(),
			ExpiresAt:   claims.ExpiresAt.Unix(),
		}

		err = s.UpdateSession(session)
		if err != nil {
			return nil, err
		}

		return session, nil
	} else {
		return nil, errors.New("invalid token claims")
	}
}
