package session

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"os"
	"time"
)

//goland:noinspection GoSnakeCaseUsage
var (
	NN_API_URL     = os.Getenv("NN_API_URL")
	NN_SITE_URL    = os.Getenv("NN_SITE_URL")
	JWT_SECRET     = []byte(os.Getenv("JWT_SECRET"))
	validAudiences = []string{NN_SITE_URL, NN_API_URL}
)

var (
	ErrMissingAudience        = errors.New("missing audience")
	ErrEmptyAudienceEntry     = errors.New("empty audience entry")
	ErrInvalidAudience        = errors.New("invalid audience")
	ErrSessionSubjectMismatch = errors.New("session does not match token subject")
)

const (
	msgJWTSecretUnset  = "JWT_SECRET environment variable must be set"
	msgSiteAPIURLUnset = "NN_SITE_URL and NN_API_URL environment variables must be set"
)

func init() {
	if len(JWT_SECRET) == 0 {
		log.Fatal(msgJWTSecretUnset)
	}
	if NN_SITE_URL == "" || NN_API_URL == "" {
		log.Fatal(msgSiteAPIURLUnset)
	}
}

// Service - The session service interface
type Service interface {
	AddSession(session *Session) error
	GetSession(id string) (*Session, error)
	UpdateSession(session *Session) error
	DeleteSession(id string) error
	CreateJWT(*Session) (string, error)
	ReadJWT(token string) (*Session, error)
}

// service - The session service implementation
type service struct {
	store Store
}

// NewService - Create a new session service
func NewService(store Store) Service {
	return &service{
		store: store,
	}
}

// AddSession adds a session to the database and cache
func (s *service) AddSession(session *Session) error {
	err := s.store.AddSessionToDB(session)
	if err != nil {
		return err
	}
	if err := s.store.AddSessionToCache(session); err != nil {
		log.Println("failed to add session to cache:\n\t", err)
	}
	return nil
}

// GetSession gets a session by ID
func (s *service) GetSession(id string) (*Session, error) {
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
func (s *service) UpdateSession(session *Session) error {
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
func (s *service) DeleteSession(id string) error {
	err := s.store.DeleteSessionInDB(id)
	if err != nil {
		return err
	}
	// Unlike AddSessionToCache, this can't be fail-open: a stale cache entry would keep a revoked session valid until it expires.
	if err := s.store.DeleteSessionFromCache(id); err != nil {
		log.Println("Session deleted from DB but failed to evict from cache:\n\t", err)
		return err
	}
	return nil
}

// CreateJWT creates a JWT for a session
func (s *service) CreateJWT(session *Session) (string, error) {
	// ExpiresAt == 0 means the session never expires (see Session.IsValid and
	// store.AddSessionToCache). Encoding that literally would set the JWT's
	// exp claim to 1970-01-01, which ReadJWT would immediately reject as
	// expired, so omit the exp claim entirely in that case instead.
	var expiresAt *jwt.NumericDate
	if session.ExpiresAt != 0 {
		expiresAt = jwt.NewNumericDate(time.Unix(session.ExpiresAt, 0))
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, SessionClaims{
		session.Permissions,
		jwt.RegisteredClaims{
			ExpiresAt: expiresAt,
			IssuedAt:  jwt.NewNumericDate(time.Unix(session.IssuedAt, 0)),
			Issuer:    NN_API_URL,
			Subject:   session.UserID,
			Audience:  validAudiences,
			ID:        session.ID,
		},
	}).SignedString(JWT_SECRET)
}

// ReadJWT reads a JWT and returns the session
func (s *service) ReadJWT(tokenStr string) (*Session, error) {
	claims := &SessionClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return JWT_SECRET, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	if len(claims.Audience) == 0 {
		return nil, ErrMissingAudience
	}
	for _, aud := range claims.Audience {
		if aud == "" {
			return nil, ErrEmptyAudienceEntry
		}
		valid := false
		for _, validAud := range validAudiences {
			if aud == validAud {
				valid = true
				break
			}
		}
		if !valid {
			log.Println(ErrInvalidAudience, aud)
			return nil, ErrInvalidAudience
		}
	}

	// The session must still exist in the store; a deleted/logged-out
	// session must not be revivable just because its JWT hasn't expired yet.
	session, err := s.GetSession(claims.ID)
	if err != nil {
		return nil, err
	}
	if session.UserID != claims.Subject {
		return nil, ErrSessionSubjectMismatch
	}

	session.LastUsedAt = time.Now().Unix()
	err = s.UpdateSession(session)
	if err != nil {
		return nil, err
	}

	return session, nil
}
