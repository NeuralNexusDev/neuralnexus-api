package authroutes

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goccy/go-json"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
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
func (m *mockLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*auth.LinkedAccount, error) {
	return nil, nil
}
func (m *mockLinkAccountStore) DeleteLinkedAccount(string, auth.Platform) error { return nil }
func (m *mockLinkAccountStore) SetLinkedAccountLoginEnabled(string, auth.Platform, bool) error {
	return nil
}

// mockSessionService implements auth.SessionService for unit testing
// OAuthHandler's ModeLink cookie handling.
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

// newModeLinkRequest builds a request that passes OAuthHandler's state/nonce
// checks and reaches the ModeLink branch, so tests can focus on its session
// cookie handling.
func newModeLinkRequest(t *testing.T) *http.Request {
	t.Helper()
	state := linking.OAuthState{
		Platform:    auth.PlatformDiscord,
		Nonce:       "test-nonce",
		RedirectURI: "https://example.com/done",
		Mode:        linking.ModeLink,
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	stateB64 := base64.URLEncoding.EncodeToString(stateJSON)

	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	return r
}

// TestOAuthHandlerLinkModeNoSessionRejected covers OAuthHandler's own,
// remaining responsibility for ModeLink: reject with a specific message when
// there's no session in the request context at all. Reading the session
// cookie and validating the JWT (a malformed token, an expired session) is
// now SessionMiddleware's job - see middleware_test.go's
// TestSessionMiddlewareInvalidCookieRejected and
// TestSessionMiddlewareExpiredSessionRejectedAndDeleted - since a session
// this handler receives via context is already known-valid, or absent.
func TestOAuthHandlerLinkModeNoSessionRejected(t *testing.T) {
	r := newModeLinkRequest(t)
	w := httptest.NewRecorder()

	OAuthHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when there's no session in context, got %d", w.Code)
	}
}

// TestOAuthHandlerLinkModeWithSessionProceedsToProcessOAuthLink confirms a
// session already in context (as SessionMiddleware would have put it there)
// clears OAuthHandler's own gate and reaches linking.ProcessOAuthLink,
// instead of being rejected at the handler level. An unsupported platform
// makes ProcessOAuthLink fail immediately on its own platform switch,
// without a real network call to any OAuth provider - the point here is
// only to observe that we got past the session check (a 401 would mean we
// didn't), not to exercise the OAuth exchange itself.
func TestOAuthHandlerLinkModeWithSessionProceedsToProcessOAuthLink(t *testing.T) {
	state := linking.OAuthState{
		Platform:    auth.Platform("unsupported-platform"),
		Nonce:       "test-nonce",
		RedirectURI: "https://example.com/done",
		Mode:        linking.ModeLink,
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	stateB64 := base64.URLEncoding.EncodeToString(stateJSON)

	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	ctx := context.WithValue(r.Context(), mw.SessionKey, session)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	OAuthHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code == http.StatusUnauthorized {
		t.Fatal("expected the session check to pass and reach ProcessOAuthLink, got 401")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 from ProcessOAuthLink's own unsupported-platform error, got %d", w.Code)
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
