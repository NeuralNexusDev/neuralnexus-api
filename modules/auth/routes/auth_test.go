package authroutes

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
)

// mockAccountService implements auth.AccountService for unit testing
// OAuthHandler and LoginHandler. None of its methods should ever be called
// for an invalid state.Mode, since that's rejected before any account/
// session work happens. account, when set, is returned by both
// GetAccountByUsername and GetAccountByEmail instead of auth.ErrNotFound.
type mockAccountService struct {
	account *auth.Account
}

var _ auth.AccountService = (*mockAccountService)(nil)

func (m *mockAccountService) AddAccount(*auth.Account) error { return nil }
func (m *mockAccountService) GetAccountByID(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *mockAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	if m.account != nil {
		return m.account, nil
	}
	return nil, auth.ErrNotFound
}
func (m *mockAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	if m.account != nil {
		return m.account, nil
	}
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
// OAuthHandler's ModeLink cookie handling and LogoutHandler.
type mockSessionService struct {
	readJWTFunc      func(token string) (*auth.Session, error)
	createJWTFunc    func(session *auth.Session) (string, error)
	addSessionErr    error
	deleteSessionErr error
	deletedIDs       []string
}

var _ auth.SessionService = (*mockSessionService)(nil)

func (m *mockSessionService) AddSession(*auth.Session) error           { return m.addSessionErr }
func (m *mockSessionService) GetSession(string) (*auth.Session, error) { return nil, auth.ErrNotFound }
func (m *mockSessionService) UpdateSession(*auth.Session) error        { return nil }
func (m *mockSessionService) DeleteSession(id string) error {
	m.deletedIDs = append(m.deletedIDs, id)
	return m.deleteSessionErr
}
func (m *mockSessionService) CreateJWT(session *auth.Session) (string, error) {
	if m.createJWTFunc != nil {
		return m.createJWTFunc(session)
	}
	return "", nil
}
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
		RedirectURI: "https://neuralnexus.test/done",
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
// cookie and validating the JWT is now SessionMiddleware's job - see
// middleware_test.go's TestSessionMiddlewareInvalidCookieFailsOpen and
// TestSessionMiddlewareExpiredCookieFailsOpenAndDeletesSession - since a
// session this handler receives via context is already known-valid, or
// absent (an invalid/expired cookie fails open rather than being rejected,
// so it looks the same as "absent" from here).
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
		RedirectURI: "https://neuralnexus.test/done",
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
		RedirectURI: "https://neuralnexus.test/done",
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

// TestOAuthHandlerRejectsRedirectOutsideSiteOrigin is the regression test for
// an open redirect: state.RedirectURI is attacker-controlled (client-supplied,
// base64-encoded JSON) and used to be passed straight to http.Redirect with no
// allow-list check, sending the browser - and the fresh session cookie set
// right before the redirect - to any URL an attacker chose.
func TestOAuthHandlerRejectsRedirectOutsideSiteOrigin(t *testing.T) {
	state := linking.OAuthState{
		Platform:    auth.PlatformDiscord,
		Nonce:       "test-nonce",
		RedirectURI: "https://evil.example.com/phish",
		Mode:        linking.ModeLogin,
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	stateB64 := base64.URLEncoding.EncodeToString(stateJSON)

	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	OAuthHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a redirect URI outside the site origin, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "" {
		t.Errorf("expected no redirect to be issued, got Location: %q", location)
	}
}

// TestOAuthHandlerAllowsRedirectMatchingSiteOrigin confirms a same-origin
// RedirectURI still clears the allow-list check (an unsupported platform then
// fails ProcessOAuthLogin's own switch with 500, without any network call -
// the point is only to confirm we got past the redirect check, not 400).
func TestOAuthHandlerAllowsRedirectMatchingSiteOrigin(t *testing.T) {
	state := linking.OAuthState{
		Platform:    auth.Platform("unsupported-platform"),
		Nonce:       "test-nonce",
		RedirectURI: "https://neuralnexus.test/done",
		Mode:        linking.ModeLogin,
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	stateB64 := base64.URLEncoding.EncodeToString(stateJSON)

	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	OAuthHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code == http.StatusBadRequest {
		t.Fatal("expected the redirect check to pass and reach ProcessOAuthLogin, got 400")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 from ProcessOAuthLogin's own unsupported-platform error, got %d", w.Code)
	}
}

// -------------- OpenIDHandler --------------

// newOpenIDRequest builds a Steam OpenID callback request carrying a valid,
// base64-encoded state param and a matching nonce cookie (unless nonce is
// empty, in which case no cookie is set at all), plus any extra openid.*
// query params a test wants to layer on.
func newOpenIDRequest(t *testing.T, mode linking.Mode, redirectURI, nonce string, extra url.Values) *http.Request {
	t.Helper()
	state := linking.OAuthState{
		Platform:    auth.PlatformSteam,
		Nonce:       "test-nonce",
		RedirectURI: redirectURI,
		Mode:        mode,
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	stateB64 := base64.URLEncoding.EncodeToString(stateJSON)

	q := url.Values{}
	for k, v := range extra {
		q[k] = v
	}
	q.Set("state", stateB64)

	r := httptest.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	if nonce != "" {
		r.AddCookie(&http.Cookie{Name: "nonce", Value: nonce})
	}
	return r
}

func TestOpenIDHandlerNoStateRejected(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/openid", nil)
	w := httptest.NewRecorder()

	OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when no state is provided, got %d", w.Code)
	}
}

func TestOpenIDHandlerRejectsRedirectOutsideSiteOrigin(t *testing.T) {
	r := newOpenIDRequest(t, linking.ModeLogin, "https://evil.example.com/phish", "", nil)
	w := httptest.NewRecorder()

	OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a redirect URI outside the site origin, got %d", w.Code)
	}
}

func TestOpenIDHandlerMissingNonceCookieRejected(t *testing.T) {
	r := newOpenIDRequest(t, linking.ModeLogin, "https://neuralnexus.test/done", "", nil)
	w := httptest.NewRecorder()

	OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when there's no nonce cookie, got %d", w.Code)
	}
}

func TestOpenIDHandlerNonceMismatchRejected(t *testing.T) {
	r := newOpenIDRequest(t, linking.ModeLogin, "https://neuralnexus.test/done", "wrong-nonce", nil)
	w := httptest.NewRecorder()

	OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when the nonce cookie doesn't match state.Nonce, got %d", w.Code)
	}
}

func TestOpenIDHandlerInvalidModeRejectedWithoutPanic(t *testing.T) {
	r := newOpenIDRequest(t, linking.Mode("bogus-mode"), "https://neuralnexus.test/done", "test-nonce", nil)
	w := httptest.NewRecorder()

	handler := OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("OpenIDHandler panicked on an invalid mode instead of returning an error response: %v", p)
		}
	}()
	handler(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for an invalid mode, got %d", w.Code)
	}
}

// TestOpenIDHandlerLinkModeNoSessionRejected is the regression test for
// checking the mode/session before doing any of the expensive Steam
// verification work: a ModeLink request with no session in context must be
// rejected without ever reaching linking.VerifySteamOpenIDCallback (which
// would otherwise burn a real network round-trip to Steam for a request
// that's going to be rejected anyway).
func TestOpenIDHandlerLinkModeNoSessionRejected(t *testing.T) {
	r := newOpenIDRequest(t, linking.ModeLink, "https://neuralnexus.test/done", "test-nonce", nil)
	w := httptest.NewRecorder()

	OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when there's no session in context, got %d", w.Code)
	}
}

// TestOpenIDHandlerLinkModeExpiredSessionRejected is the regression test for
// a session present but expired: requireValidModeAndSession must reject it
// at the gate (before any Steam network call), the same as no session at
// all - previously only presence was checked here, and an expired session
// fell through to ProcessSteamLink's own IsValid() check, by which point
// VerifySteamOpenIDCallback and GetSteamUser had already run.
func TestOpenIDHandlerLinkModeExpiredSessionRejected(t *testing.T) {
	r := newOpenIDRequest(t, linking.ModeLink, "https://neuralnexus.test/done", "test-nonce", nil)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	ctx := context.WithValue(r.Context(), mw.SessionKey, session)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for an expired session before any Steam network call, got %d", w.Code)
	}
}

// TestOpenIDHandlerLinkModeWithSessionProceedsPastSessionCheck confirms a
// session already in context clears OpenIDHandler's own gate and reaches
// linking.VerifySteamOpenIDCallback - an openid.mode other than "id_res"
// makes that fail immediately on its own, without a real call to Steam, so
// this only observes that we got past the session check (401 would mean we
// didn't), not that the OpenID exchange itself succeeds.
func TestOpenIDHandlerLinkModeWithSessionProceedsPastSessionCheck(t *testing.T) {
	r := newOpenIDRequest(t, linking.ModeLink, "https://neuralnexus.test/done", "test-nonce", url.Values{
		"openid.mode": {"cancel"},
	})
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	ctx := context.WithValue(r.Context(), mw.SessionKey, session)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	OpenIDHandler(&mockAccountService{}, &mockLinkAccountStore{}, &mockSessionService{})(w, r)

	if w.Code == http.StatusUnauthorized {
		t.Fatal("expected the session check to pass and reach VerifySteamOpenIDCallback, got 401")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 from VerifySteamOpenIDCallback's own openid.mode check, got %d", w.Code)
	}
}

// -------------- LogoutHandler --------------

// TestLogoutHandlerClearsSessionCookie is the regression test for a real
// bug: LogoutHandler deleted the server-side session but never cleared the
// browser's session cookie, which kept sending it (SessionMiddleware now
// reads that cookie) until its own ~24h expiry - see auth.go's sessionCookie
// helper, now shared between setting and clearing it.
func TestLogoutHandlerClearsSessionCookie(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &mockSessionService{}

	LogoutHandler(ss)(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if len(ss.deletedIDs) != 1 || ss.deletedIDs[0] != "s1" {
		t.Errorf("expected DeleteSession to be called with the session ID, got: %v", ss.deletedIDs)
	}

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.SessionCookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected LogoutHandler to set a Set-Cookie clearing the session cookie, got none")
	}
	if sessionCookie.Value != "" {
		t.Errorf("expected the cleared cookie's value to be empty, got %q", sessionCookie.Value)
	}
	if !sessionCookie.Expires.Before(time.Now()) {
		t.Errorf("expected the cleared cookie's Expires to be in the past, got %v", sessionCookie.Expires)
	}
}

func TestLogoutHandlerDoesNotClearCookieOnDeleteSessionError(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &mockSessionService{deleteSessionErr: errors.New("db exploded")}

	LogoutHandler(ss)(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.SessionCookieName {
			t.Error("expected no session cookie to be set when DeleteSession fails")
		}
	}
}

// -------------- LoginHandler --------------

// TestLoginHandlerSetsSessionCookie is the regression test for a gap
// alongside the LogoutHandler fix above: LoginHandler returned the JWT in
// the response body but never set the session cookie, unlike OAuthHandler,
// leaving the browser frontend with no cookie after a plain username/
// password login.
func TestLoginHandlerSetsSessionCookie(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build test account: %v", err)
	}
	as := &mockAccountService{account: account}
	ss := &mockSessionService{
		createJWTFunc: func(*auth.Session) (string, error) { return "test-jwt", nil },
	}
	body := `{"username":"testuser","password":"correct-password"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	LoginHandler(as, ss)(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.SessionCookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected LoginHandler to set a session cookie, got none")
	}
	if sessionCookie.Value != "test-jwt" {
		t.Errorf("expected the session cookie's value to be the JWT, got %q", sessionCookie.Value)
	}
	if !sessionCookie.Expires.After(time.Now()) {
		t.Errorf("expected the cookie's Expires to be in the future, got %v", sessionCookie.Expires)
	}
}

func TestLoginHandlerRejectsBadPasswordWithoutCookie(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build test account: %v", err)
	}
	as := &mockAccountService{account: account}
	ss := &mockSessionService{}
	body := `{"username":"testuser","password":"wrong-password"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	LoginHandler(as, ss)(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.SessionCookieName {
			t.Error("expected no session cookie to be set on a failed login")
		}
	}
}

func TestLoginHandlerNoCookieOnCreateJWTError(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build test account: %v", err)
	}
	as := &mockAccountService{account: account}
	ss := &mockSessionService{
		createJWTFunc: func(*auth.Session) (string, error) { return "", errors.New("jwt signing failed") },
	}
	body := `{"username":"testuser","password":"correct-password"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	LoginHandler(as, ss)(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.SessionCookieName {
			t.Error("expected no session cookie to be set when CreateJWT fails")
		}
	}
}

func TestLoginHandlerNoCookieOnAddSessionError(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build test account: %v", err)
	}
	as := &mockAccountService{account: account}
	ss := &mockSessionService{
		createJWTFunc: func(*auth.Session) (string, error) { return "test-jwt", nil },
		addSessionErr: errors.New("db exploded"),
	}
	body := `{"username":"testuser","password":"correct-password"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	LoginHandler(as, ss)(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == mw.SessionCookieName {
			t.Error("expected no session cookie to be set when AddSession fails")
		}
	}
}

// TestLoginHandlerPersistsSessionBeforeCreatingJWT is the regression test for
// matching OAuthHandler/OpenIDHandler's order: those persist the session
// (inside Process*Login) before ever minting a JWT via
// issueSessionAndRedirect, whereas LoginHandler used to create the JWT
// first and persist the session after - harmless today since CreateJWT has
// no store dependency, but inconsistent, and would silently paper over a
// future CreateJWT that assumes the session already exists in the store.
// TestLoginHandlerNoCookieOnAddSessionError can't tell the two orders apart
// (both produce a 500 with no cookie either way), so this asserts directly
// that CreateJWT is never reached once AddSession has already failed.
func TestLoginHandlerPersistsSessionBeforeCreatingJWT(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build test account: %v", err)
	}
	as := &mockAccountService{account: account}
	createJWTCalled := false
	ss := &mockSessionService{
		createJWTFunc: func(*auth.Session) (string, error) {
			createJWTCalled = true
			return "test-jwt", nil
		},
		addSessionErr: errors.New("db exploded"),
	}
	body := `{"username":"testuser","password":"correct-password"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	LoginHandler(as, ss)(w, r)

	if createJWTCalled {
		t.Error("expected CreateJWT to never be called once AddSession has already failed")
	}
}

// TestLoginHandlerPaysHashCostOnUnknownAccount is the regression test for a
// username/email enumeration timing side channel: a nonexistent account used
// to fail instantly, while an existing account with a wrong password paid the
// full Argon2id cost - letting an attacker infer account existence from
// response time. LoginHandler now burns the same cost via
// auth.DummyValidateUser on a lookup miss. 20ms is a wide margin below the
// real cost (~140ms measured for these Argon2id params) while comfortably
// above what an instant ErrNotFound return would take.
func TestLoginHandlerPaysHashCostOnUnknownAccount(t *testing.T) {
	as := &mockAccountService{}
	ss := &mockSessionService{}
	body := `{"username":"no-such-user","password":"whatever"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	start := time.Now()
	LoginHandler(as, ss)(w, r)
	elapsed := time.Since(start)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if elapsed < 20*time.Millisecond {
		t.Errorf("expected the lookup-miss path to pay the dummy hash cost (~140ms), took %v", elapsed)
	}
}
