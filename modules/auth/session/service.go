package session

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
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
	CookieName           = "session"
	msgJWTSecretUnset    = "JWT_SECRET environment variable must be set"
	msgSiteAPIURLUnset   = "NN_SITE_URL and NN_API_URL environment variables must be set"
	logFailedToCreateJWT = "Failed to create JWT:\n\t"
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
	AddSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, id string) error
	SetSessionCookie(ctx context.Context, w http.ResponseWriter, session *Session) error
	CreateJWT(*Session) (string, error)
	ReadJWT(ctx context.Context, token string) (*Session, error)
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
func (s *service) AddSession(ctx context.Context, session *Session) error {
	ctx = context.WithoutCancel(ctx)
	err := s.store.AddSessionToDB(ctx, session)
	if err != nil {
		return err
	}
	if err := s.store.AddSessionToCache(ctx, session); err != nil {
		reqctx.Log(ctx, "failed to add session to cache:\n\t", err.Error())
	}
	return nil
}

// GetSession gets a session by ID
func (s *service) GetSession(ctx context.Context, id string) (*Session, error) {
	session, err := s.store.GetSessionFromCache(ctx, id)
	if err != nil {
		session, err = s.store.GetSessionFromDB(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := s.store.AddSessionToCache(ctx, session); err != nil {
			reqctx.Log(ctx, "failed to re-populate session cache after a cache miss:\n\t", err.Error())
		}
	}
	return session, nil
}

// UpdateSession updates a session
func (s *service) UpdateSession(ctx context.Context, session *Session) error {
	ctx = context.WithoutCancel(ctx)
	err := s.store.UpdateSessionInDB(ctx, session)
	if err != nil {
		return err
	}
	if err := s.store.AddSessionToCache(ctx, session); err != nil {
		reqctx.Log(ctx, "failed to update session in cache:\n\t", err.Error())
	}
	return nil
}

// DeleteSession deletes a session by ID
func (s *service) DeleteSession(ctx context.Context, id string) error {
	err := s.store.DeleteSessionInDB(ctx, id)
	if err != nil {
		return err
	}
	// Unlike AddSessionToCache, this can't be fail-open: a stale cache entry would keep a revoked session valid until it expires.
	if err := s.store.DeleteSessionFromCache(ctx, id); err != nil {
		reqctx.Log(ctx, "Session deleted from DB but failed to evict from cache:\n\t", err.Error())
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

// SetSessionCookie creates a JWT for an already-persisted session and sets it as the session cookie.
func (s *service) SetSessionCookie(ctx context.Context, w http.ResponseWriter, session *Session) error {
	jwtString, err := s.CreateJWT(session)
	if err != nil {
		reqctx.Log(ctx, logFailedToCreateJWT, err.Error())
		return err
	}
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    jwtString,
		Domain:   ".neuralnexus.dev",
		Path:     "/",
		Expires:  time.Unix(session.ExpiresAt, 0),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	return nil
}

// ReadJWT reads a JWT and returns the session
func (s *service) ReadJWT(ctx context.Context, tokenStr string) (*Session, error) {
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
			reqctx.Log(ctx, ErrInvalidAudience.Error(), aud)
			return nil, ErrInvalidAudience
		}
	}

	// The session must still exist in the store; a deleted/logged-out
	// session must not be revivable just because its JWT hasn't expired yet.
	session, err := s.GetSession(ctx, claims.ID)
	if err != nil {
		return nil, err
	}
	if session.UserID != claims.Subject {
		return nil, ErrSessionSubjectMismatch
	}

	session.LastUsedAt = time.Now().Unix()
	err = s.UpdateSession(ctx, session)
	if err != nil {
		return nil, err
	}

	return session, nil
}
