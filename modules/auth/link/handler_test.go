package link

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

type stubAccountService struct {
	account              *account.Account
	lookupErr            error
	passwordAuthDisabled bool
	passwordAuthErr      error
	newSessionErr        error
}

var _ account.AccountService = (*stubAccountService)(nil)

func (s *stubAccountService) AddAccount(*account.Account) error { return nil }

func (s *stubAccountService) GetAccountByID(string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

func (s *stubAccountService) GetAccountByUsername(string) (*account.Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, account.ErrNotFound
}

func (s *stubAccountService) GetAccountByEmail(string) (*account.Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, account.ErrNotFound
}

func (s *stubAccountService) UpdateAccount(*account.Account) error { return nil }

func (s *stubAccountService) DeleteAccount(string) error { return nil }

func (s *stubAccountService) IsPasswordAuthEnabled(string) (bool, error) {
	return !s.passwordAuthDisabled, s.passwordAuthErr
}

const stubSessionPermission = "stub:perm"

func (s *stubAccountService) NewSession(a *account.Account, expiresAt int64) (*session.Session, error) {
	if s.newSessionErr != nil {
		return nil, s.newSessionErr
	}
	return &session.Session{ID: "stub-session", UserID: a.UserID, Permissions: []string{stubSessionPermission}, ExpiresAt: expiresAt}, nil
}

type stubLinkAccountStore struct {
	existing   *LinkedAccount
	byPlatform map[Platform]*LinkedAccount
	addErr     error
}

var _ LinkAccountStore = (*stubLinkAccountStore)(nil)

func (s *stubLinkAccountStore) AddLinkedAccountToDB(*LinkedAccount) error { return s.addErr }

func (s *stubLinkAccountStore) UpdateLinkedAccount(*LinkedAccount) error { return nil }

func (s *stubLinkAccountStore) GetLinkedAccountByPlatformID(platform Platform, _ string) (*LinkedAccount, error) {
	if la, ok := s.byPlatform[platform]; ok {
		return la, nil
	}
	if s.existing != nil {
		return s.existing, nil
	}
	return nil, account.ErrNotFound
}

func (s *stubLinkAccountStore) GetLinkedAccountByPlatformName(Platform, string) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (s *stubLinkAccountStore) GetLinkedAccountByUserID(string, Platform) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (s *stubLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*LinkedAccount, error) {
	return nil, nil
}

func (s *stubLinkAccountStore) DeleteLinkedAccount(string, Platform) error { return nil }

func (s *stubLinkAccountStore) SetLinkedAccountLoginEnabled(string, Platform, bool) error {
	return nil
}

type stubSessionService struct {
	createJWT        func(*session.Session) (string, error)
	addSessionErr    error
	deleteSessionErr error
	deletedIDs       []string
	added            []*session.Session
}

var _ session.SessionService = (*stubSessionService)(nil)

func (s *stubSessionService) AddSession(session *session.Session) error {
	s.added = append(s.added, session)
	return s.addSessionErr
}

func (s *stubSessionService) GetSession(string) (*session.Session, error) {
	return nil, account.ErrNotFound
}

func (s *stubSessionService) UpdateSession(*session.Session) error { return nil }

func (s *stubSessionService) DeleteSession(id string) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return s.deleteSessionErr
}

func (s *stubSessionService) CreateJWT(session *session.Session) (string, error) {
	if s.createJWT != nil {
		return s.createJWT(session)
	}
	return "", nil
}

func (s *stubSessionService) ReadJWT(string) (*session.Session, error) {
	return nil, account.ErrNotFound
}

func encodeState(t *testing.T, state OAuthState) string {
	t.Helper()
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	return base64.URLEncoding.EncodeToString(stateJSON)
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

func requireProblemRedirect(t *testing.T, w *httptest.ResponseRecorder, wantTarget string, wantStatus int, wantDetail string) {
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
	var p responses.Problem
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("failed to unmarshal problem JSON %q: %v", raw, err)
	}
	if int(p.Status) != wantStatus {
		t.Errorf("expected problem status %d, got %d", wantStatus, p.Status)
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

func TestAU13OAuthHandlerMissingCode(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-13_OAuthMissingCode", func(t *testing.T) {
		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidRequest)
	})
}

func TestAU14OAuthHandlerMissingState(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode", nil)
	w := httptest.NewRecorder()

	t.Run("AU-14_OAuthMissingState", func(t *testing.T) {
		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidRequest)
	})
}

func TestAU15OAuthHandlerLinkModeNoSession(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLink}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-15_OAuthLinkModeNoSession", func(t *testing.T) {
		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
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

// swapDefaultTransport swaps the package-level http.DefaultTransport for the
// duration of a subtest, restoring it after. Discord's token exchange (falls
// back to http.DefaultClient via context.Background()) and user fetch, and
// Steam's check_authentication/GetPlayerSummaries client, all build clients
// with no Transport set, so they resolve to http.DefaultTransport at call
// time - the same seam modules/projects/projects_test.go's swapTransport uses.
func swapDefaultTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() { http.DefaultTransport = orig })
}

func swapSteamAPIKey(t *testing.T, value string) {
	t.Helper()
	original := STEAM_API_KEY
	STEAM_API_KEY = value
	t.Cleanup(func() { STEAM_API_KEY = original })
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

	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=disc-code&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()
	as := &stubAccountService{}
	las := &stubLinkAccountStore{}
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "test-jwt", nil }}

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

	state := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
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
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "test-jwt", nil }}

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

func auDiscordTransport(t *testing.T) {
	t.Helper()
	swapDefaultTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Host == "discord.com" && req.URL.Path == "/api/oauth2/token":
			return auJSONResponse(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600,"scope":"identify"}`), nil
		case req.Method == http.MethodGet && req.URL.Host == "discord.com" && strings.HasSuffix(req.URL.Path, "/users/@me"):
			return auJSONResponse(http.StatusOK, `{"id":"d1","username":"alice","email":"a@b.com"}`), nil
		default:
			return nil, fmt.Errorf("unexpected outbound request %s %s", req.Method, req.URL.String())
		}
	}))
}

func auOAuthRequest(t *testing.T, state OAuthState) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?code=disc-code&state="+encodeState(t, state), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: state.Nonce})
	return r
}

const auSteamPlayersBody = `{"response":{"players":[{"steamid":"76561198000000000","personaname":"steamplayer","profileurl":"https://steamcommunity.com/id/steamplayer","avatarfull":"https://avatar.example/a.jpg"}]}}`

func auSteamTransport(t *testing.T, checkStatus, summaryStatus int) {
	t.Helper()
	auSteamTransportBody(t, checkStatus, summaryStatus, auSteamPlayersBody)
}

func auSteamTransportBody(t *testing.T, checkStatus, summaryStatus int, summaryBody string) {
	t.Helper()
	swapSteamAPIKey(t, "test-steam-api-key")
	swapDefaultTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Host == "steamcommunity.com" && req.URL.Path == "/openid/login":
			return auTextResponse(checkStatus, "ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"), nil
		case req.Method == http.MethodGet && req.URL.Host == "api.steampowered.com" && req.URL.Path == "/ISteamUser/GetPlayerSummaries/v2/":
			return auJSONResponse(summaryStatus, summaryBody), nil
		default:
			return nil, fmt.Errorf("unexpected outbound request %s %s", req.Method, req.URL.String())
		}
	}))
}

func auOpenIDRequest(t *testing.T, state OAuthState) *http.Request {
	t.Helper()
	q := url.Values{
		"state":               {encodeState(t, state)},
		"openid.mode":         {"id_res"},
		"openid.claimed_id":   {"https://steamcommunity.com/openid/id/76561198000000000"},
		"openid.identity":     {"https://steamcommunity.com/openid/id/76561198000000000"},
		"openid.signed":       {"op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle"},
		"openid.sig":          {"deadbeef=="},
		"openid.return_to":    {"https://neuralnexus.test/api/openid"},
		"openid.assoc_handle": {"handle1"},
	}
	r := httptest.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: state.Nonce})
	return r
}

func TestAU52to57OAuthAndOpenIDFailuresHideCause(t *testing.T) {
	oauthState := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	steamState := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	okJWT := func(*session.Session) (string, error) { return "test-jwt", nil }
	failJWT := func(*session.Session) (string, error) { return "", testerrors.ErrSigningFailed }

	t.Run("AU-52_OAuthProcessFails", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: okJWT, addSessionErr: testerrors.ErrDBDown}

		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, ss)(w, auOAuthRequest(t, oauthState))

		requireProblemRedirect(t, w, oauthState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-53_OAuthJWTFails", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: failJWT}

		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, ss)(w, auOAuthRequest(t, oauthState))

		requireProblemRedirect(t, w, oauthState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-54_OpenIDVerifyFailsWithoutRejectingAssertion", func(t *testing.T) {
		auSteamTransport(t, http.StatusInternalServerError, http.StatusOK)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: okJWT}

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, ss)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-55_OpenIDGetSteamUserFails", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusInternalServerError)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: okJWT}

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, ss)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-56_OpenIDProcessFails", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: okJWT, addSessionErr: testerrors.ErrDBDown}

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, ss)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-57_OpenIDJWTFails", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: failJWT}

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, ss)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})
}

func TestAU17OpenIDHandlerMissingState(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/openid", nil)
	w := httptest.NewRecorder()

	t.Run("AU-17_OpenIDMissingState", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidRequest)
	})
}

func TestAU18OpenIDHandlerLinkModeNoSession(t *testing.T) {
	state := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLink}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/openid?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-18_OpenIDLinkModeNoSession", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
	})
}

func TestAU19OpenIDHandlerBadOpenIDMode(t *testing.T) {
	state := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	q := url.Values{"state": {stateB64}, "openid.mode": {"cancel"}}
	r := httptest.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-19_OpenIDBadOpenIDMode", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU20OpenIDHandlerBadClaimedID(t *testing.T) {
	state := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	q := url.Values{"state": {stateB64}, "openid.mode": {"id_res"}, "openid.claimed_id": {"not-a-steam-id-url"}}
	r := httptest.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-20_OpenIDBadClaimedID", func(t *testing.T) {
		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{})(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU22DecodeAndValidateStateHappyPath(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
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
		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidRequest)
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
		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidState)
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
		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU26DecodeAndValidateStateMissingRequiredField(t *testing.T) {
	state := OAuthState{Platform: "", Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-26_DecodeAndValidateStateMissingRequiredField", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU27DecodeAndValidateStateDisallowedRedirect(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://evil.example.com/phish", Mode: ModeLogin}
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
		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU28DecodeAndValidateStateMissingNonceCookie(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	w := httptest.NewRecorder()

	t.Run("AU-28_DecodeAndValidateStateMissingNonceCookie", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU29DecodeAndValidateStateNonceMismatch(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "wrong-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-29_DecodeAndValidateStateNonceMismatch", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, session.NN_SITE_URL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU30RequireValidModeAndSessionLoginMode(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-30_RequireValidModeAndSessionLoginMode", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, ModeLogin, "https://neuralnexus.test/done"); !ok {
			t.Fatal("expected true for mode=login")
		}
		if loc := w.Header().Get("Location"); loc != "" {
			t.Errorf("expected no response written, got a redirect to %q", loc)
		}
	})
}

func TestAU31RequireValidModeAndSessionLinkModeValidSession(t *testing.T) {
	session := &session.Session{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	t.Run("AU-31_RequireValidModeAndSessionLinkModeValidSession", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, ModeLink, "https://neuralnexus.test/done"); !ok {
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
		if ok := requireValidModeAndSession(w, r, ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false with no session in context")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
	})
}

func TestAU33RequireValidModeAndSessionLinkModeNilSession(t *testing.T) {
	ctx := context.WithValue(context.Background(), mw.SessionKey, (*session.Session)(nil))
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	t.Run("AU-33_RequireValidModeAndSessionLinkModeNilSession", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for a nil session")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
	})
}

func TestAU34RequireValidModeAndSessionLinkModeExpiredSession(t *testing.T) {
	session := &session.Session{UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	t.Run("AU-34_RequireValidModeAndSessionLinkModeExpiredSession", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for an expired session")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
	})
}

func TestAU35RequireValidModeAndSessionUnrecognizedMode(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-35_RequireValidModeAndSessionUnrecognizedMode", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, Mode("bogus-mode"), "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for an unrecognized mode")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU38RedirectWithErrorHappyPath(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-38_RedirectWithErrorHappyPath", func(t *testing.T) {
		redirectWithError(w, r, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "something broke")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "something broke")
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
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "bad input")
	})
}

func TestAU42RedirectUnauthorized(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-42_RedirectUnauthorized", func(t *testing.T) {
		redirectUnauthorized(w, r, "https://neuralnexus.test/done", "no session")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, "no session")
	})
}

func TestAU43RedirectInternalServerError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-43_RedirectInternalServerError", func(t *testing.T) {
		redirectInternalServerError(w, r, "https://neuralnexus.test/done", "boom")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusInternalServerError, "boom")
	})
}

func TestAU44IsAllowedRedirectSameOrigin(t *testing.T) {
	t.Run("AU-44_IsAllowedRedirectSameOrigin", func(t *testing.T) {
		if !isAllowedRedirect(session.NN_SITE_URL + "/some/path") {
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
	siteURL, err := url.Parse(session.NN_SITE_URL)
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
	original := session.NN_SITE_URL
	session.NN_SITE_URL = "http://example.com/%zz"
	defer func() { session.NN_SITE_URL = original }()

	t.Run("AU-48_IsAllowedRedirectUnparseableSiteURL", func(t *testing.T) {
		if isAllowedRedirect("https://neuralnexus.test/done") {
			t.Error("expected false when NN_SITE_URL itself fails to parse")
		}
	})
}

func auWithSession(r *http.Request, userID string) *http.Request {
	session := &session.Session{ID: "link-session", UserID: userID}
	return r.WithContext(context.WithValue(r.Context(), mw.SessionKey, session))
}

func auDiscordTransportWithoutScope(t *testing.T) {
	t.Helper()
	swapDefaultTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost && req.URL.Host == "discord.com" && req.URL.Path == "/api/oauth2/token" {
			return auJSONResponse(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600}`), nil
		}
		return nil, fmt.Errorf("unexpected outbound request %s %s", req.Method, req.URL.String())
	}))
}

func auMicrosoftTransport(t *testing.T) {
	t.Helper()
	swapDefaultTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Host == "login.microsoftonline.com":
			return auJSONResponse(http.StatusOK, `{"access_token":"ms-at","token_type":"Bearer","expires_in":3600,"scope":"XboxLive.signin offline_access"}`), nil
		case req.Method == http.MethodPost && req.URL.Host == "user.auth.xboxlive.com":
			return auJSONResponse(http.StatusOK, `{"Token":"xbl-t"}`), nil
		case req.Method == http.MethodPost && req.URL.Host == "xsts.auth.xboxlive.com":
			return auJSONResponse(http.StatusOK, `{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1","xid":"xid1","gtg":"Tag"}]}}`), nil
		case req.Method == http.MethodPost && req.URL.Host == "api.minecraftservices.com" && req.URL.Path == "/authentication/login_with_xbox":
			return auJSONResponse(http.StatusOK, `{"access_token":"mc-at"}`), nil
		case req.Method == http.MethodGet && req.URL.Host == "api.minecraftservices.com" && req.URL.Path == "/minecraft/profile":
			return auJSONResponse(http.StatusOK, `{"id":"069a79f4-44e9-4726-a5be-fca90e38aaf5","name":"Steve"}`), nil
		default:
			return nil, fmt.Errorf("unexpected outbound request %s %s", req.Method, req.URL.String())
		}
	}))
}

// auSessionChangesContext serves first for the first read of the session and
// second for every read after it, standing in for a session that disappears or
// expires between the handler's check and the linking call.
type auSessionChangesContext struct {
	context.Context
	reads  int
	first  any
	second any
}

func (c *auSessionChangesContext) Value(key any) any {
	if key != mw.SessionKey {
		return c.Context.Value(key)
	}
	c.reads++
	if c.reads == 1 {
		return c.first
	}
	return c.second
}

func auWithSessionChange(r *http.Request, second any) *http.Request {
	first := &session.Session{ID: "link-session", UserID: "u1"}
	return r.WithContext(&auSessionChangesContext{Context: r.Context(), first: first, second: second})
}

func TestAU60to71OAuthAndOpenIDFailureMapping(t *testing.T) {
	const redirect = "https://neuralnexus.test/done"
	discordLogin := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: redirect, Mode: ModeLogin}
	discordLink := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: redirect, Mode: ModeLink}
	steamLogin := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: redirect, Mode: ModeLogin}
	steamLink := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: redirect, Mode: ModeLink}
	okJWT := func(*session.Session) (string, error) { return "test-jwt", nil }
	linkedElsewhere := &stubLinkAccountStore{existing: &LinkedAccount{UserID: "u2", Verified: true, LoginEnabled: true}}
	addFails := func(err error) *stubLinkAccountStore { return &stubLinkAccountStore{addErr: err} }

	t.Run("AU-60_OAuthInvalidPlatform", func(t *testing.T) {
		w := httptest.NewRecorder()
		state := OAuthState{Platform: "nonexistent", Nonce: "test-nonce", RedirectURI: redirect, Mode: ModeLogin}

		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{createJWT: okJWT})(w, auOAuthRequest(t, state))

		requireProblemRedirect(t, w, redirect, http.StatusBadRequest, msgInvalidPlatform)
	})

	t.Run("AU-61_OAuthTokenWithoutScope", func(t *testing.T) {
		auDiscordTransportWithoutScope(t)
		w := httptest.NewRecorder()

		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{createJWT: okJWT})(w, auOAuthRequest(t, discordLogin))

		requireProblemRedirect(t, w, redirect, http.StatusBadRequest, msgMissingOAuthScope)
	})

	t.Run("AU-62_OpenIDSteamIDMismatch", func(t *testing.T) {
		auSteamTransportBody(t, http.StatusOK, http.StatusOK, `{"response":{"players":[{"steamid":"76561198999999999","personaname":"someone"}]}}`)
		w := httptest.NewRecorder()

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{createJWT: okJWT})(w, auOpenIDRequest(t, steamLogin))

		requireProblemRedirect(t, w, redirect, http.StatusBadRequest, msgSteamAccountMismatch)
	})

	t.Run("AU-63_OAuthLinkedToDifferentAccount", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()

		OAuthHandler(&stubAccountService{}, linkedElsewhere, &stubSessionService{createJWT: okJWT})(w, auWithSession(auOAuthRequest(t, discordLink), "u1"))

		requireProblemRedirect(t, w, redirect, http.StatusConflict, msgPlatformAlreadyLinked)
	})

	t.Run("AU-64_OAuthConflictingMicrosoftIdentities", func(t *testing.T) {
		auMicrosoftTransport(t)
		w := httptest.NewRecorder()
		las := &stubLinkAccountStore{byPlatform: map[Platform]*LinkedAccount{
			PlatformXboxLive:  {UserID: "u1", Verified: true, LoginEnabled: true},
			PlatformMinecraft: {UserID: "u2", Verified: true, LoginEnabled: true},
		}}
		state := OAuthState{Platform: PlatformMinecraft, Nonce: "test-nonce", RedirectURI: redirect, Mode: ModeLogin}

		OAuthHandler(&stubAccountService{}, las, &stubSessionService{createJWT: okJWT})(w, auOAuthRequest(t, state))

		requireProblemRedirect(t, w, redirect, http.StatusConflict, msgConflictingMicrosoftIdentities)
	})

	t.Run("AU-65_OAuthLinkFails", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()

		OAuthHandler(&stubAccountService{}, addFails(testerrors.ErrDBDown), &stubSessionService{createJWT: okJWT})(w, auWithSession(auOAuthRequest(t, discordLink), "u1"))

		requireProblemRedirect(t, w, redirect, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-67_OpenIDLinkedToDifferentAccount", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(&stubAccountService{}, linkedElsewhere, &stubSessionService{createJWT: okJWT})(w, auWithSession(auOpenIDRequest(t, steamLink), "u1"))

		requireProblemRedirect(t, w, redirect, http.StatusConflict, msgPlatformAlreadyLinked)
	})

	t.Run("AU-68_OpenIDLinkFails", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(&stubAccountService{}, addFails(testerrors.ErrDBDown), &stubSessionService{createJWT: okJWT})(w, auWithSession(auOpenIDRequest(t, steamLink), "u1"))

		requireProblemRedirect(t, w, redirect, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	expired := &session.Session{ID: "link-session", UserID: "u1", ExpiresAt: 1}

	t.Run("AU-66_OAuthSessionGoneBeforeLinking", func(t *testing.T) {
		w := httptest.NewRecorder()

		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{createJWT: okJWT})(w, auWithSessionChange(auOAuthRequest(t, discordLink), nil))

		requireProblemRedirect(t, w, redirect, http.StatusUnauthorized, msgLoginRequiredToLink)
	})

	t.Run("AU-69_OAuthSessionExpiredBeforeLinking", func(t *testing.T) {
		w := httptest.NewRecorder()

		OAuthHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{createJWT: okJWT})(w, auWithSessionChange(auOAuthRequest(t, discordLink), expired))

		requireProblemRedirect(t, w, redirect, http.StatusUnauthorized, msgSessionExpired)
	})

	t.Run("AU-70_OpenIDSessionGoneBeforeLinking", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{createJWT: okJWT})(w, auWithSessionChange(auOpenIDRequest(t, steamLink), nil))

		requireProblemRedirect(t, w, redirect, http.StatusUnauthorized, msgLoginRequiredToLink)
	})

	t.Run("AU-71_OpenIDSessionExpiredBeforeLinking", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, &stubSessionService{createJWT: okJWT})(w, auWithSessionChange(auOpenIDRequest(t, steamLink), expired))

		requireProblemRedirect(t, w, redirect, http.StatusUnauthorized, msgSessionExpired)
	})
}

func TestAU72to74OAuthAndOpenIDFailuresUseMapping(t *testing.T) {
	oauthState := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	steamState := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	okJWT := func(*session.Session) (string, error) { return "test-jwt", nil }
	disabledLink := &stubLinkAccountStore{existing: &LinkedAccount{UserID: "u1", Verified: true, LoginEnabled: false}}

	t.Run("AU-72_OAuthProcessMapsLoginDisabled", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: okJWT}

		OAuthHandler(&stubAccountService{}, disabledLink, ss)(w, auOAuthRequest(t, oauthState))

		requireProblemRedirect(t, w, oauthState.RedirectURI, http.StatusForbidden, msgPlatformLoginDisabled)
	})

	t.Run("AU-73_OpenIDSteamUserMapsNoPlayers", func(t *testing.T) {
		auSteamTransportBody(t, http.StatusOK, http.StatusOK, `{"response":{"players":[]}}`)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: okJWT}

		OpenIDHandler(&stubAccountService{}, &stubLinkAccountStore{}, ss)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusNotFound, msgSteamAccountNotFound)
	})

	t.Run("AU-74_OpenIDProcessMapsLoginDisabled", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()
		ss := &stubSessionService{createJWT: okJWT}

		OpenIDHandler(&stubAccountService{}, disabledLink, ss)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusForbidden, msgPlatformLoginDisabled)
	})
}

type stubLinkingService struct {
	links    []*LinkedAccount
	linksErr error

	unlinkErr   error
	unlinkCalls []Platform

	setEnableErr   error
	setEnableCalls []bool
}

var _ Service = (*stubLinkingService)(nil)

func (s *stubLinkingService) GetUserLinkedAccounts(string) ([]*LinkedAccount, error) {
	return s.links, s.linksErr
}
func (s *stubLinkingService) UnlinkPlatform(_ string, platform Platform) error {
	s.unlinkCalls = append(s.unlinkCalls, platform)
	return s.unlinkErr
}
func (s *stubLinkingService) SetPlatformLoginEnabled(_ string, _ Platform, enabled bool) error {
	s.setEnableCalls = append(s.setEnableCalls, enabled)
	return s.setEnableErr
}
func (s *stubLinkingService) UpdateFromPlatform(Platform, string, PlatformData) (string, error) {
	return "", nil
}

func newSessionRequest(method string, session *session.Session, userID, platform, body string) *http.Request {
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/", strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, "/", nil).WithContext(ctx)
	}
	if userID != "" {
		r.SetPathValue("user_id", userID)
	}
	if platform != "" {
		r.SetPathValue("platform", platform)
	}
	return r
}

func selfSession(userID string) *session.Session {
	return &session.Session{UserID: userID}
}

func adminUsersSession(userID string) *session.Session {
	return &session.Session{UserID: userID, Permissions: []string{perms.ScopeAdminUsers.Node}}
}

func expectStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("expected %d, got %d: %s", want, w.Code, w.Body.String())
	}
}

func expectDetail(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", w.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func TestUR28GetUserLinkedAccountsHandlerSelfHappyPath(t *testing.T) {
	svc := &stubLinkingService{links: []*LinkedAccount{{Platform: PlatformDiscord}}}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-28_GetUserLinkedAccountsSelfHappyPath", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUR29GetUserLinkedAccountsHandlerAdminHappyPath(t *testing.T) {
	svc := &stubLinkingService{links: []*LinkedAccount{{Platform: PlatformDiscord}}}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-29_GetUserLinkedAccountsAdminHappyPath", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUR30GetUserLinkedAccountsHandlerForbidden(t *testing.T) {
	svc := &stubLinkingService{}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-30_GetUserLinkedAccountsForbidden", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToViewLinkedAccounts)
	})
}

func TestUR31GetUserLinkedAccountsHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubLinkingService{linksErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-31_GetUserLinkedAccountsServiceErrorMapsTo500", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToGetLinkedAccounts)
	})
}

func TestUR32UnlinkPlatformHandlerHappyPath(t *testing.T) {
	svc := &stubLinkingService{}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("UR-32_UnlinkPlatformHappyPath", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNoContent)
		if len(svc.unlinkCalls) != 1 || svc.unlinkCalls[0] != PlatformDiscord {
			t.Errorf("expected UnlinkPlatform(discord) to be called, got %v", svc.unlinkCalls)
		}
	})
}

func TestUR33UnlinkPlatformHandlerForbidden(t *testing.T) {
	svc := &stubLinkingService{}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "someone-else", "discord", "")
	w := httptest.NewRecorder()

	t.Run("UR-33_UnlinkPlatformForbidden", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToUnlinkPlatforms)
		if len(svc.unlinkCalls) != 0 {
			t.Error("expected UnlinkPlatform to never be called for a forbidden request")
		}
	})
}

func TestUR34UnlinkPlatformHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubLinkingService{unlinkErr: account.ErrNotFound}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("UR-34_UnlinkPlatformNotFoundMapsTo404", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgPlatformNotLinked)
	})
}

func TestUR35UnlinkPlatformHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubLinkingService{unlinkErr: account.ErrWouldLockAccount}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("UR-35_UnlinkPlatformWouldLockAccountMapsTo400", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgSetPasswordOrLinkBeforeUnlinking)
	})
}

func TestUR36UnlinkPlatformHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubLinkingService{unlinkErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("UR-36_UnlinkPlatformUnclassifiedErrorMapsTo500", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToUnlinkPlatform)
	})
}

func TestUR37SetPlatformLoginEnabledHandlerHappyPath(t *testing.T) {
	svc := &stubLinkingService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-37_SetPlatformLoginEnabledHappyPath", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNoContent)
		if len(svc.setEnableCalls) != 1 || svc.setEnableCalls[0] != true {
			t.Errorf("expected SetPlatformLoginEnabled(true), got %v", svc.setEnableCalls)
		}
	})
}

func TestUR38SetPlatformLoginEnabledHandlerForbidden(t *testing.T) {
	svc := &stubLinkingService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "someone-else", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-38_SetPlatformLoginEnabledForbidden", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToUpdatePlatforms)
	})
}

func TestUR39SetPlatformLoginEnabledHandlerMalformedBody(t *testing.T) {
	svc := &stubLinkingService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `not json`)
	w := httptest.NewRecorder()

	t.Run("UR-39_SetPlatformLoginEnabledMalformedBody", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setEnableCalls) != 0 {
			t.Error("expected SetPlatformLoginEnabled to never be called for a malformed body")
		}
	})
}

func TestUR40SetPlatformLoginEnabledHandlerMissingField(t *testing.T) {
	svc := &stubLinkingService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{}`)
	w := httptest.NewRecorder()

	t.Run("UR-40_SetPlatformLoginEnabledMissingField", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setEnableCalls) != 0 {
			t.Error("expected SetPlatformLoginEnabled to never be called when login_enabled is omitted")
		}
	})
}

func TestUR41SetPlatformLoginEnabledHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubLinkingService{setEnableErr: account.ErrNotFound}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-41_SetPlatformLoginEnabledNotFoundMapsTo404", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgPlatformNotLinked)
	})
}

func TestUR42SetPlatformLoginEnabledHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubLinkingService{setEnableErr: account.ErrWouldLockAccount}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":false}`)
	w := httptest.NewRecorder()

	t.Run("UR-42_SetPlatformLoginEnabledWouldLockAccountMapsTo400", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgSetPasswordOrLinkBeforeDisabling)
	})
}

func TestUR43SetPlatformLoginEnabledHandlerUnverifiedMapsTo400(t *testing.T) {
	svc := &stubLinkingService{setEnableErr: ErrLinkedAccountUnverified}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-43_SetPlatformLoginEnabledUnverifiedMapsTo400", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgLinkedAccountUnverified)
	})
}

func TestUR44SetPlatformLoginEnabledHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubLinkingService{setEnableErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-44_SetPlatformLoginEnabledUnclassifiedErrorMapsTo500", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToUpdatePlatform)
	})
}
