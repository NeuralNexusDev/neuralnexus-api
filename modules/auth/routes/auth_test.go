package authroutes

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
)

// mockAccountService implements auth.AccountService for unit testing
// OAuthHandler. None of its methods should ever be called for an invalid
// state.Mode, since that's rejected before any account/session work happens.
type mockAccountService struct{}

var _ auth.AccountService = (*mockAccountService)(nil)

func (m *mockAccountService) AddAccount(*auth.Account) error { return nil }
func (m *mockAccountService) GetAccountByID(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *mockAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *mockAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *mockAccountService) UpdateAccount(*auth.Account) error { return nil }
func (m *mockAccountService) DeleteAccount(string) error        { return nil }

// mockLinkAccountStore implements auth.LinkAccountStore for unit testing
// OAuthHandler, for the same reason as mockAccountService above.
type mockLinkAccountStore struct{}

var _ auth.LinkAccountStore = (*mockLinkAccountStore)(nil)

func (m *mockLinkAccountStore) AddLinkedAccountToDB(*auth.LinkedAccount) error { return nil }
func (m *mockLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error  { return nil }
func (m *mockLinkAccountStore) GetLinkedAccountByPlatformID(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *mockLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *mockLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}

// mockSessionService implements auth.SessionService for unit testing
// sessionFromCookie.
type mockSessionService struct {
	readJWTFunc func(token string) (*auth.Session, error)
}

var _ auth.SessionService = (*mockSessionService)(nil)

func (m *mockSessionService) AddSession(*auth.Session) error           { return nil }
func (m *mockSessionService) GetSession(string) (*auth.Session, error) { return nil, auth.ErrNotFound }
func (m *mockSessionService) UpdateSession(*auth.Session) error        { return nil }
func (m *mockSessionService) DeleteSession(string) error               { return nil }
func (m *mockSessionService) CreateJWT(*auth.Session) (string, error)  { return "", nil }
func (m *mockSessionService) ReadJWT(token string) (*auth.Session, error) {
	return m.readJWTFunc(token)
}

func TestSessionFromCookieNoCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	ss := &mockSessionService{}

	_, err := sessionFromCookie(r, ss)
	if err == nil {
		t.Fatal("expected an error when the session cookie is missing")
	}
	if err.Error() != "not logged in" {
		t.Errorf("expected \"not logged in\", got: %v", err)
	}
}

func TestSessionFromCookieReadJWTErrorPropagates(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "not-a-real-jwt"})
	wantErr := errors.New("malformed token")
	ss := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return nil, wantErr
		},
	}

	_, err := sessionFromCookie(r, ss)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the ReadJWT error to propagate, got: %v", err)
	}
}

func TestSessionFromCookieExpiredSessionRejected(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "expired-jwt"})
	ss := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}, nil
		},
	}

	_, err := sessionFromCookie(r, ss)
	if err == nil {
		t.Fatal("expected an error for an expired session")
	}
	if err.Error() != "session expired" {
		t.Errorf("expected \"session expired\", got: %v", err)
	}
}

func TestSessionFromCookieValidSessionReturned(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "valid-jwt"})
	want := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	ss := &mockSessionService{
		readJWTFunc: func(token string) (*auth.Session, error) {
			if token != "valid-jwt" {
				t.Errorf("expected ReadJWT to be called with the cookie's value, got %q", token)
			}
			return want, nil
		},
	}

	got, err := sessionFromCookie(r, ss)
	if err != nil {
		t.Fatalf("sessionFromCookie returned error: %v", err)
	}
	if got != want {
		t.Error("expected the session returned by ReadJWT to be passed through")
	}
}

// Regression test: an invalid/unrecognized state.Mode used to fall through
// the mode switch's default case (which wrote a 400 response but never
// returned) straight into ss.CreateJWT(session) and session.ExpiresAt with
// session still nil - a guaranteed nil pointer dereference reachable by any
// client that sends a state blob with a Mode other than "login"/"link"
// (state.Mode is entirely client-controlled JSON, so this was a one-request
// crash, not a hypothetical). The fix adds the missing return.
func TestOAuthHandlerInvalidModeRejectedWithoutPanic(t *testing.T) {
	state := linking.OAuthState{
		Platform:    auth.PlatformDiscord,
		Nonce:       "test-nonce",
		RedirectURI: "https://example.com/done",
		Mode:        linking.Mode("bogus-mode"),
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	stateB64 := base64.URLEncoding.EncodeToString(stateJSON)

	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	handler := OAuthHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("OAuthHandler panicked on an invalid mode instead of returning an error response: %v", p)
		}
	}()
	handler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for an invalid mode, got %d", w.Code)
	}
}
