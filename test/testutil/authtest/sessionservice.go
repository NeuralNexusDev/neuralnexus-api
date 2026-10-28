package authtest

import (
	"context"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
)

// FakeSessionService is a session.Service that records AddSession and
// DeleteSession calls and returns the configured errors.
type FakeSessionService struct {
	// CreateJWTFunc, when set, is called by CreateJWT; when nil, CreateJWT returns "" and nil.
	CreateJWTFunc func(*session.Session) (string, error)
	// AddSessionErr is returned by AddSession after the session is appended to Added.
	AddSessionErr error
	// DeleteSessionErr is returned by DeleteSession after the ID is appended to DeletedIDs.
	DeleteSessionErr error
	// Added holds every session passed to AddSession, in order.
	Added []*session.Session
	// DeletedIDs holds every ID passed to DeleteSession, in order.
	DeletedIDs []string
}

var _ session.Service = (*FakeSessionService)(nil)

// AddSession appends sess to Added and returns AddSessionErr.
func (f *FakeSessionService) AddSession(_ context.Context, sess *session.Session) error {
	f.Added = append(f.Added, sess)
	return f.AddSessionErr
}

// GetSession returns session.ErrNotFound.
func (f *FakeSessionService) GetSession(context.Context, string) (*session.Session, error) {
	return nil, session.ErrNotFound
}

// UpdateSession returns nil.
func (f *FakeSessionService) UpdateSession(context.Context, *session.Session) error { return nil }

// DeleteSession appends id to DeletedIDs and returns DeleteSessionErr.
func (f *FakeSessionService) DeleteSession(_ context.Context, id string) error {
	f.DeletedIDs = append(f.DeletedIDs, id)
	return f.DeleteSessionErr
}

// CreateJWT calls CreateJWTFunc when set, and otherwise returns "" and nil.
func (f *FakeSessionService) CreateJWT(_ context.Context, sess *session.Session) (string, error) {
	if f.CreateJWTFunc != nil {
		return f.CreateJWTFunc(sess)
	}
	return "", nil
}

// SetSessionCookie sets the session cookie from CreateJWT, or returns the CreateJWT error.
func (f *FakeSessionService) SetSessionCookie(ctx context.Context, w http.ResponseWriter, sess *session.Session) error {
	jwt, err := f.CreateJWT(ctx, sess)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: session.CookieName, Value: jwt})
	return nil
}

// ReadJWT returns session.ErrNotFound.
func (f *FakeSessionService) ReadJWT(context.Context, string) (*session.Session, error) {
	return nil, session.ErrNotFound
}
