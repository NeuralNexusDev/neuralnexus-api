package authroutes

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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

type stubAccountService struct {
	account              *auth.Account
	lookupErr            error
	passwordAuthDisabled bool
	passwordAuthErr      error
}

var _ auth.AccountService = (*stubAccountService)(nil)

func (s *stubAccountService) AddAccount(*auth.Account) error { return nil }
func (s *stubAccountService) GetAccountByID(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (s *stubAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, auth.ErrNotFound
}
func (s *stubAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, auth.ErrNotFound
}
func (s *stubAccountService) UpdateAccount(*auth.Account) error { return nil }
func (s *stubAccountService) DeleteAccount(string) error        { return nil }
func (s *stubAccountService) IsPasswordAuthEnabled(string) (bool, error) {
	return !s.passwordAuthDisabled, s.passwordAuthErr
}

type stubLinkAccountStore struct{}

var _ auth.LinkAccountStore = (*stubLinkAccountStore)(nil)

func (s *stubLinkAccountStore) AddLinkedAccountToDB(*auth.LinkedAccount) error { return nil }
func (s *stubLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error  { return nil }
func (s *stubLinkAccountStore) GetLinkedAccountByPlatformID(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (s *stubLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (s *stubLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (s *stubLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*auth.LinkedAccount, error) {
	return nil, nil
}
func (s *stubLinkAccountStore) DeleteLinkedAccount(string, auth.Platform) error { return nil }
func (s *stubLinkAccountStore) SetLinkedAccountLoginEnabled(string, auth.Platform, bool) error {
	return nil
}

type stubSessionService struct {
	createJWT        func(*auth.Session) (string, error)
	addSessionErr    error
	deleteSessionErr error
	deletedIDs       []string
}

var _ auth.SessionService = (*stubSessionService)(nil)

func (s *stubSessionService) AddSession(*auth.Session) error           { return s.addSessionErr }
func (s *stubSessionService) GetSession(string) (*auth.Session, error) { return nil, auth.ErrNotFound }
func (s *stubSessionService) UpdateSession(*auth.Session) error        { return nil }
func (s *stubSessionService) DeleteSession(id string) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return s.deleteSessionErr
}
func (s *stubSessionService) CreateJWT(session *auth.Session) (string, error) {
	if s.createJWT != nil {
		return s.createJWT(session)
	}
	return "", nil
}
func (s *stubSessionService) ReadJWT(string) (*auth.Session, error) { return nil, auth.ErrNotFound }

// encodeState base64url-encodes an OAuthState as OAuthHandler/OpenIDHandler
// expect it in the "state" query param.
func encodeState(t *testing.T, state linking.OAuthState) string {
	t.Helper()
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	return base64.URLEncoding.EncodeToString(stateJSON)
}

// problemBody mirrors problempb.Problem's JSON shape, enough to decode what
// redirectWithError embeds in the "problem" query param.
type problemBody struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

func requireRedirect(t *testing.T, w *httptest.ResponseRecorder, wantTarget string) *url.URL {
	t.Helper()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse Location %q: %v", location, err)
	}
	target := (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
	if target != wantTarget {
		t.Errorf("expected redirect target %q, got %q (full Location %q)", wantTarget, target, location)
	}
	return u
}

func requireProblemRedirect(t *testing.T, w *httptest.ResponseRecorder, wantTarget string, wantStatus int, wantTitle, wantDetail string) {
	t.Helper()
	u := requireRedirect(t, w, wantTarget)
	problemB64 := u.Query().Get("problem")
	if problemB64 == "" {
		t.Fatalf("expected a problem query param, got none")
	}
	raw, err := base64.URLEncoding.DecodeString(problemB64)
	if err != nil {
		t.Fatalf("failed to base64-decode problem param: %v", err)
	}
	var p problemBody
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("failed to unmarshal problem JSON %q: %v", raw, err)
	}
	if p.Status != wantStatus {
		t.Errorf("expected problem status %d, got %d", wantStatus, p.Status)
	}
	if p.Title != wantTitle {
		t.Errorf("expected problem title %q, got %q", wantTitle, p.Title)
	}
	if p.Detail != wantDetail {
		t.Errorf("expected problem detail %q, got %q", wantDetail, p.Detail)
	}
}

func findCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAU01LoginHandlerUsernameHappyPath(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{createJWT: func(*auth.Session) (string, error) { return "test-jwt", nil }}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("AU-01_LoginUsernameHappyPath", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		cookie := findCookie(w, mw.SessionCookieName)
		if cookie == nil || cookie.Value != "test-jwt" {
			t.Errorf("expected session cookie with value %q, got %+v", "test-jwt", cookie)
		}
	})
}

func TestAU02LoginHandlerEmailHappyPath(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{createJWT: func(*auth.Session) (string, error) { return "test-jwt", nil }}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"test@example.com","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("AU-02_LoginEmailHappyPath", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) == nil {
			t.Error("expected a session cookie to be set")
		}
	})
}

func TestAU03LoginHandlerMalformedBody(t *testing.T) {
	as := &stubAccountService{}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`not json`))
	w := httptest.NewRecorder()

	t.Run("AU-03_LoginMalformedBody", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestAU04LoginHandlerAccountLookupFails(t *testing.T) {
	as := &stubAccountService{}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"no-such-user","password":"whatever"}`))
	w := httptest.NewRecorder()

	t.Run("AU-04_LoginAccountLookupFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie on a failed lookup")
		}
	})
}

func TestAU05LoginHandlerWrongPassword(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"wrong-password"}`))
	w := httptest.NewRecorder()

	t.Run("AU-05_LoginWrongPassword", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie on a wrong password")
		}
	})
}

func TestAU06LoginHandlerPasswordAuthCheckErrors(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account, passwordAuthErr: errors.New("db exploded")}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("AU-06_LoginPasswordAuthCheckErrors", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestAU07LoginHandlerPasswordAuthDisabled(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account, passwordAuthDisabled: true}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("AU-07_LoginPasswordAuthDisabled", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie when password auth is disabled")
		}
	})
}

func TestAU08LoginHandlerAddSessionFails(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{addSessionErr: errors.New("db exploded")}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("AU-08_LoginAddSessionFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie when AddSession fails")
		}
	})
}

func TestAU09LoginHandlerCreateJWTFails(t *testing.T) {
	account, err := auth.NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{createJWT: func(*auth.Session) (string, error) { return "", errors.New("signing failed") }}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("AU-09_LoginCreateJWTFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie when CreateJWT fails")
		}
	})
}

func TestAU10LogoutHandlerHappyPath(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &stubSessionService{}

	t.Run("AU-10_LogoutHappyPath", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		if len(ss.deletedIDs) != 1 || ss.deletedIDs[0] != "s1" {
			t.Errorf("expected DeleteSession(\"s1\"), got %v", ss.deletedIDs)
		}
		cookie := findCookie(w, mw.SessionCookieName)
		if cookie == nil {
			t.Fatal("expected the session cookie to be cleared")
		}
		if cookie.Value != "" {
			t.Errorf("expected an empty cleared cookie value, got %q", cookie.Value)
		}
		if !cookie.Expires.Before(time.Now()) {
			t.Errorf("expected the cleared cookie's Expires to be in the past, got %v", cookie.Expires)
		}
	})
}

func TestAU11LogoutHandlerNilSessionInContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), mw.SessionKey, (*auth.Session)(nil))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &stubSessionService{}

	t.Run("AU-11_LogoutNilSessionInContext", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		if len(ss.deletedIDs) != 0 {
			t.Error("expected DeleteSession to never be called for a nil session")
		}
	})
}

func TestAU12LogoutHandlerDeleteSessionFails(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &stubSessionService{deleteSessionErr: errors.New("db exploded")}

	t.Run("AU-12_LogoutDeleteSessionFails", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no cleared cookie when DeleteSession fails")
		}
	})
}

func TestAU13OAuthHandlerMissingCode(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-13_OAuthMissingCode", func(t *testing.T) {
		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid request")
	})
}

func TestAU14OAuthHandlerMissingState(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode", nil)
	w := httptest.NewRecorder()

	t.Run("AU-14_OAuthMissingState", func(t *testing.T) {
		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid request")
	})
}

func TestAU15OAuthHandlerLinkModeNoSession(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLink}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-15_OAuthLinkModeNoSession", func(t *testing.T) {
		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, "Unauthorized", "You must be logged in to link an account")
	})
}

type auRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f auRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// auJSONResponse builds a canned *http.Response carrying a JSON
// Content-Type - oauth2's token exchange requires that header to parse the
// body as a JSON token response rather than as form-urlencoded.
func auJSONResponse(status int, body string) *http.Response {
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: h}
}

// auTextResponse builds a canned *http.Response with a plain-text body, for
// Steam's check_authentication response format (newline-separated
// "key:value" lines, not JSON).
func auTextResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

// swapDefaultTransport replaces the package-level http.DefaultTransport for
// the duration of a subtest and restores it afterward. ProcessOAuthLogin's
// Discord token exchange (oauth2.Config.Exchange, called with
// context.Background() so it falls back to http.DefaultClient) and its
// discordgo user fetch (discordgo.New builds a *http.Client with no
// Transport set), and VerifySteamOpenIDCallback/GetSteamUser's
// steamHTTPClient (also built with no Transport set), all resolve to
// http.DefaultTransport at call time - the same seam
// modules/projects/projects_test.go's swapTransport uses, so neither
// OAuthHandler's nor OpenIDHandler's happy path needs a live network
// dependency or an env-var skip.
func swapDefaultTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// swapSteamAPIKey overrides linking.STEAM_API_KEY for the duration of a
// subtest and restores it afterward - GetSteamUser refuses to run without
// one.
func swapSteamAPIKey(t *testing.T, value string) {
	t.Helper()
	original := linking.STEAM_API_KEY
	linking.STEAM_API_KEY = value
	t.Cleanup(func() { linking.STEAM_API_KEY = original })
}

func TestAU50OAuthHandlerLoginHappyPath(t *testing.T) {
	swapDefaultTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Host == "discord.com" && req.URL.Path == "/api/oauth2/token":
			return auJSONResponse(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600,"scope":"identify"}`), nil
		case req.Method == http.MethodGet && req.URL.Host == "discord.com" && strings.HasSuffix(req.URL.Path, "/users/@me"):
			return auJSONResponse(http.StatusOK, `{"id":"d1","username":"alice","email":"a@b.com"}`), nil
		default:
			return nil, fmt.Errorf("AU-50: unexpected outbound request %s %s", req.Method, req.URL.String())
		}
	}))

	state := linking.OAuthState{Platform: auth.PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=disc-code&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()
	as := &stubAccountService{}
	las := &stubLinkAccountStore{}
	ss := &stubSessionService{createJWT: func(*auth.Session) (string, error) { return "test-jwt", nil }}

	t.Run("AU-50_OAuthHandlerLoginHappyPath", func(t *testing.T) {
		OAuthHandler(as, las, ss)(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
		}
		if loc := w.Header().Get("Location"); loc != state.RedirectURI {
			t.Errorf("expected redirect straight to %q with no problem param, got %q", state.RedirectURI, loc)
		}
		cookie := findCookie(w, mw.SessionCookieName)
		if cookie == nil || cookie.Value != "test-jwt" {
			t.Errorf("expected session cookie with value %q, got %+v", "test-jwt", cookie)
		}
	})
}

func TestAU51OpenIDHandlerLoginHappyPath(t *testing.T) {
	swapSteamAPIKey(t, "test-steam-api-key")
	swapDefaultTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Host == "steamcommunity.com" && req.URL.Path == "/openid/login":
			return auTextResponse(http.StatusOK, "ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"), nil
		case req.Method == http.MethodGet && req.URL.Host == "api.steampowered.com" && req.URL.Path == "/ISteamUser/GetPlayerSummaries/v2/":
			return auJSONResponse(http.StatusOK, `{"response":{"players":[{"steamid":"76561198000000000","personaname":"steamplayer","profileurl":"https://steamcommunity.com/id/steamplayer","avatarfull":"https://avatar.example/a.jpg"}]}}`), nil
		default:
			return nil, fmt.Errorf("AU-51: unexpected outbound request %s %s", req.Method, req.URL.String())
		}
	}))

	state := linking.OAuthState{Platform: auth.PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	q := url.Values{
		"state":               {stateB64},
		"openid.mode":         {"id_res"},
		"openid.claimed_id":   {"https://steamcommunity.com/openid/id/76561198000000000"},
		"openid.identity":     {"https://steamcommunity.com/openid/id/76561198000000000"},
		"openid.signed":       {"op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle"},
		"openid.sig":          {"deadbeef=="},
		"openid.return_to":    {"https://neuralnexus.test/api/openid"},
		"openid.assoc_handle": {"handle1"},
	}
	r := httptest.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()
	as := &stubAccountService{}
	las := &stubLinkAccountStore{}
	ss := &stubSessionService{createJWT: func(*auth.Session) (string, error) { return "test-jwt", nil }}

	t.Run("AU-51_OpenIDHandlerLoginHappyPath", func(t *testing.T) {
		OpenIDHandler(as, las, ss)(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
		}
		if loc := w.Header().Get("Location"); loc != state.RedirectURI {
			t.Errorf("expected redirect straight to %q with no problem param, got %q", state.RedirectURI, loc)
		}
		cookie := findCookie(w, mw.SessionCookieName)
		if cookie == nil || cookie.Value != "test-jwt" {
			t.Errorf("expected session cookie with value %q, got %+v", "test-jwt", cookie)
		}
	})
}

func TestAU17OpenIDHandlerMissingState(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/openid", nil)
	w := httptest.NewRecorder()

	t.Run("AU-17_OpenIDMissingState", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid request")
	})
}

func TestAU18OpenIDHandlerLinkModeNoSession(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLink}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/openid?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-18_OpenIDLinkModeNoSession", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, "Unauthorized", "You must be logged in to link an account")
	})
}

func TestAU19OpenIDHandlerBadOpenIDMode(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	q := url.Values{"state": {stateB64}, "openid.mode": {"cancel"}}
	r := httptest.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-19_OpenIDBadOpenIDMode", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU20OpenIDHandlerBadClaimedID(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	q := url.Values{"state": {stateB64}, "openid.mode": {"id_res"}, "openid.claimed_id": {"not-a-steam-id-url"}}
	r := httptest.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-20_OpenIDBadClaimedID", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU22DecodeAndValidateStateHappyPath(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-22_DecodeAndValidateStateHappyPath", func(t *testing.T) {
		got, ok := decodeAndValidateState(w, r)
		if !ok {
			t.Fatalf("expected ok=true, got false (body: %s)", w.Body.String())
		}
		if got != state {
			t.Errorf("expected state %+v, got %+v", state, got)
		}
	})
}

func TestAU23DecodeAndValidateStateMissingParam(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-23_DecodeAndValidateStateMissingParam", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid request")
	})
}

func TestAU24DecodeAndValidateStateInvalidBase64(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state=not-valid-base64!!!", nil)
	w := httptest.NewRecorder()

	t.Run("AU-24_DecodeAndValidateStateInvalidBase64", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU25DecodeAndValidateStateInvalidJSON(t *testing.T) {
	stateB64 := base64.URLEncoding.EncodeToString([]byte("not json"))
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	w := httptest.NewRecorder()

	t.Run("AU-25_DecodeAndValidateStateInvalidJSON", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU26DecodeAndValidateStateMissingRequiredField(t *testing.T) {
	state := linking.OAuthState{Platform: "", Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-26_DecodeAndValidateStateMissingRequiredField", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU27DecodeAndValidateStateDisallowedRedirect(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://evil.example.com/phish", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-27_DecodeAndValidateStateDisallowedRedirect", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		if loc := w.Header().Get("Location"); strings.Contains(loc, "evil.example.com") {
			t.Fatalf("expected no redirect to the attacker's URL, got Location: %q", loc)
		}
		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU28DecodeAndValidateStateMissingNonceCookie(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	w := httptest.NewRecorder()

	t.Run("AU-28_DecodeAndValidateStateMissingNonceCookie", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU29DecodeAndValidateStateNonceMismatch(t *testing.T) {
	state := linking.OAuthState{Platform: auth.PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: linking.ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "wrong-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-29_DecodeAndValidateStateNonceMismatch", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auth.NN_SITE_URL, http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU30RequireValidModeAndSessionLoginMode(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-30_RequireValidModeAndSessionLoginMode", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, linking.ModeLogin, "https://neuralnexus.test/done"); !ok {
			t.Fatal("expected true for mode=login")
		}
		if loc := w.Header().Get("Location"); loc != "" {
			t.Errorf("expected no response written, got a redirect to %q", loc)
		}
	})
}

func TestAU31RequireValidModeAndSessionLinkModeValidSession(t *testing.T) {
	session := &auth.Session{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	t.Run("AU-31_RequireValidModeAndSessionLinkModeValidSession", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, linking.ModeLink, "https://neuralnexus.test/done"); !ok {
			t.Fatal("expected true for a valid session")
		}
		if loc := w.Header().Get("Location"); loc != "" {
			t.Errorf("expected no response written, got a redirect to %q", loc)
		}
	})
}

func TestAU32RequireValidModeAndSessionLinkModeNoSessionKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-32_RequireValidModeAndSessionLinkModeNoSessionKey", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, linking.ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false with no session in context")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, "Unauthorized", "You must be logged in to link an account")
	})
}

func TestAU33RequireValidModeAndSessionLinkModeNilSession(t *testing.T) {
	ctx := context.WithValue(context.Background(), mw.SessionKey, (*auth.Session)(nil))
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	t.Run("AU-33_RequireValidModeAndSessionLinkModeNilSession", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, linking.ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for a nil session")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, "Unauthorized", "You must be logged in to link an account")
	})
}

func TestAU34RequireValidModeAndSessionLinkModeExpiredSession(t *testing.T) {
	session := &auth.Session{UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	t.Run("AU-34_RequireValidModeAndSessionLinkModeExpiredSession", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, linking.ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for an expired session")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, "Unauthorized", "You must be logged in to link an account")
	})
}

func TestAU35RequireValidModeAndSessionUnrecognizedMode(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-35_RequireValidModeAndSessionUnrecognizedMode", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, linking.Mode("bogus-mode"), "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for an unrecognized mode")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "Invalid state")
	})
}

func TestAU36CreateSessionJWTAndSetCookieHappyPath(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	ss := &stubSessionService{createJWT: func(*auth.Session) (string, error) { return "test-jwt", nil }}
	w := httptest.NewRecorder()

	t.Run("AU-36_CreateSessionJWTAndSetCookieHappyPath", func(t *testing.T) {
		if err := createSessionJWTAndSetCookie(ss, w, session); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		cookie := findCookie(w, mw.SessionCookieName)
		if cookie == nil {
			t.Fatal("expected a session cookie to be set")
		}
		if cookie.Value != "test-jwt" {
			t.Errorf("expected cookie value %q, got %q", "test-jwt", cookie.Value)
		}
		wantExpires := time.Unix(session.ExpiresAt, 0)
		if !cookie.Expires.Equal(wantExpires) {
			t.Errorf("expected cookie Expires %v, got %v", wantExpires, cookie.Expires)
		}
	})
}

func TestAU37CreateSessionJWTAndSetCookieCreateJWTFails(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	wantErr := errors.New("signing failed")
	ss := &stubSessionService{createJWT: func(*auth.Session) (string, error) { return "", wantErr }}
	w := httptest.NewRecorder()

	t.Run("AU-37_CreateSessionJWTAndSetCookieCreateJWTFails", func(t *testing.T) {
		err := createSessionJWTAndSetCookie(ss, w, session)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no cookie to be set when CreateJWT fails")
		}
	})
}

func TestAU38RedirectWithErrorHappyPath(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-38_RedirectWithErrorHappyPath", func(t *testing.T) {
		redirectWithError(w, r, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "something broke")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "something broke")
	})
}

func TestAU39RedirectWithErrorUnparseableTarget(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	target := "http://example.com/%zz"

	t.Run("AU-39_RedirectWithErrorUnparseableTarget", func(t *testing.T) {
		redirectWithError(w, r, target, http.StatusBadRequest, "Bad Request", "something broke")

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
		}
		if loc := w.Header().Get("Location"); loc != target {
			t.Errorf("expected redirect straight to %q with no problem param, got %q", target, loc)
		}
	})
}

func TestAU41RedirectBadRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-41_RedirectBadRequest", func(t *testing.T) {
		redirectBadRequest(w, r, "https://neuralnexus.test/done", "bad input")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "bad input")
	})
}

func TestAU42RedirectUnauthorized(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-42_RedirectUnauthorized", func(t *testing.T) {
		redirectUnauthorized(w, r, "https://neuralnexus.test/done", "no session")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, "Unauthorized", "no session")
	})
}

func TestAU43RedirectInternalServerError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-43_RedirectInternalServerError", func(t *testing.T) {
		redirectInternalServerError(w, r, "https://neuralnexus.test/done", "boom")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusInternalServerError, "Internal Server Error", "boom")
	})
}

func TestAU44IsAllowedRedirectSameOrigin(t *testing.T) {
	t.Run("AU-44_IsAllowedRedirectSameOrigin", func(t *testing.T) {
		if !isAllowedRedirect(auth.NN_SITE_URL + "/some/path") {
			t.Error("expected true for a same-origin redirect URI")
		}
	})
}

func TestAU45IsAllowedRedirectDifferentHost(t *testing.T) {
	t.Run("AU-45_IsAllowedRedirectDifferentHost", func(t *testing.T) {
		if isAllowedRedirect("https://evil.example.com/phish") {
			t.Error("expected false for a different host")
		}
	})
}

func TestAU46IsAllowedRedirectDifferentScheme(t *testing.T) {
	siteURL, err := url.Parse(auth.NN_SITE_URL)
	if err != nil {
		t.Fatalf("failed to parse NN_SITE_URL: %v", err)
	}

	t.Run("AU-46_IsAllowedRedirectDifferentScheme", func(t *testing.T) {
		if isAllowedRedirect("http://" + siteURL.Host + "/done") {
			t.Error("expected false for a different scheme")
		}
	})
}

func TestAU47IsAllowedRedirectUnparseableRedirectURI(t *testing.T) {
	t.Run("AU-47_IsAllowedRedirectUnparseableRedirectURI", func(t *testing.T) {
		if isAllowedRedirect("http://example.com/%zz") {
			t.Error("expected false for an unparseable redirect URI")
		}
	})
}

func TestAU48IsAllowedRedirectUnparseableSiteURL(t *testing.T) {
	original := auth.NN_SITE_URL
	auth.NN_SITE_URL = "http://example.com/%zz"
	defer func() { auth.NN_SITE_URL = original }()

	t.Run("AU-48_IsAllowedRedirectUnparseableSiteURL", func(t *testing.T) {
		if isAllowedRedirect("https://neuralnexus.test/done") {
			t.Error("expected false when NN_SITE_URL itself fails to parse")
		}
	})
}

func TestAU49SessionCookie(t *testing.T) {
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("AU-49_SessionCookie", func(t *testing.T) {
		c := sessionCookie("abc", expires)

		if c.Name != mw.SessionCookieName {
			t.Errorf("expected Name %q, got %q", mw.SessionCookieName, c.Name)
		}
		if c.Value != "abc" {
			t.Errorf("expected Value %q, got %q", "abc", c.Value)
		}
		if c.Domain != ".neuralnexus.dev" {
			t.Errorf("expected Domain %q, got %q", ".neuralnexus.dev", c.Domain)
		}
		if c.Path != "/" {
			t.Errorf("expected Path %q, got %q", "/", c.Path)
		}
		if !c.Expires.Equal(expires) {
			t.Errorf("expected Expires %v, got %v", expires, c.Expires)
		}
		if !c.Secure {
			t.Error("expected Secure=true")
		}
		if !c.HttpOnly {
			t.Error("expected HttpOnly=true")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("expected SameSite=Lax, got %v", c.SameSite)
		}
	})
}
