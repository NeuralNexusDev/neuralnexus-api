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
	"golang.org/x/oauth2"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil/authtest"
)

type stubAccountService struct {
	account              *account.Account
	lookupErr            error
	passwordAuthDisabled bool
	passwordAuthErr      error
	newSessionErr        error
}

var _ account.Service = (*stubAccountService)(nil)

func (s *stubAccountService) AddAccount(context.Context, *account.Account) error { return nil }

func (s *stubAccountService) GetAccountByID(context.Context, string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

func (s *stubAccountService) GetAccountByUsername(context.Context, string) (*account.Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, account.ErrNotFound
}

func (s *stubAccountService) GetAccountByEmail(context.Context, string) (*account.Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, account.ErrNotFound
}

func (s *stubAccountService) UpdateAccount(context.Context, *account.Account) error { return nil }

func (s *stubAccountService) DeleteAccount(context.Context, string) error { return nil }

func (s *stubAccountService) IsPasswordAuthEnabled(context.Context, string) (bool, error) {
	return !s.passwordAuthDisabled, s.passwordAuthErr
}

const stubSessionPermission = "stub:perm"

func (s *stubAccountService) NewSession(_ context.Context, a *account.Account, expiresAt int64) (*session.Session, error) {
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

var _ Store = (*stubLinkAccountStore)(nil)

func (s *stubLinkAccountStore) AddLinkedAccountToDB(context.Context, *LinkedAccount) error {
	return s.addErr
}

func (s *stubLinkAccountStore) UpdateLinkedAccount(context.Context, *LinkedAccount) error { return nil }

func (s *stubLinkAccountStore) GetLinkedAccountByPlatformID(_ context.Context, platform Platform, _ string) (*LinkedAccount, error) {
	if la, ok := s.byPlatform[platform]; ok {
		return la, nil
	}
	if s.existing != nil {
		return s.existing, nil
	}
	return nil, account.ErrNotFound
}

func (s *stubLinkAccountStore) GetLinkedAccountByPlatformName(context.Context, Platform, string) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (s *stubLinkAccountStore) GetLinkedAccountByUserID(context.Context, string, Platform) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (s *stubLinkAccountStore) GetLinkedAccountsByUserID(context.Context, string) ([]*LinkedAccount, error) {
	return nil, nil
}

func (s *stubLinkAccountStore) DeleteLinkedAccount(context.Context, string, Platform) error {
	return nil
}

func (s *stubLinkAccountStore) SetLinkedAccountLoginEnabled(context.Context, string, Platform, bool) error {
	return nil
}

func encodeState(t *testing.T, state OAuthState) string {
	t.Helper()
	stateJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	return base64.URLEncoding.EncodeToString(stateJSON)
}

func requireProblemRedirect(t *testing.T, w *httptest.ResponseRecorder, wantTarget string, wantStatus int, wantDetail string) {
	t.Helper()
	u := testutil.RequireRedirect(t, w, wantTarget)
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

func TestAU13OAuthHandlerMissingCode(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-13_OAuthMissingCode", func(t *testing.T) {
		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{}, auSiteURL)(w, r)

		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidRequest)
	})
}

func TestAU14OAuthHandlerMissingState(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?code=somecode", nil)
	w := httptest.NewRecorder()

	t.Run("AU-14_OAuthMissingState", func(t *testing.T) {
		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{}, auSiteURL)(w, r)

		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidRequest)
	})
}

func TestAU15OAuthHandlerLinkModeNoSession(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLink}
	stateB64 := encodeState(t, state)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?code=somecode&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-15_OAuthLinkModeNoSession", func(t *testing.T) {
		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{}, auSiteURL)(w, r)

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

const auSiteURL = "https://neuralnexus.test"

func auConfig() *config.Config {
	return testutil.NewConfig(func(c *config.Config) {
		c.Discord.OAuth2Config = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: "https://discord.com/api/oauth2/token"}}
		c.Microsoft = config.Microsoft{
			OAuth2Config:            &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/token"}},
			XboxLiveAuthenticateURL: "https://user.auth.xboxlive.com/user/authenticate",
			XstsAuthorizeURL:        "https://xsts.auth.xboxlive.com/xsts/authorize",
			MinecraftLoginURL:       "https://api.minecraftservices.com/authentication/login_with_xbox",
			MinecraftProfileURL:     "https://api.minecraftservices.com/minecraft/profile",
		}
		c.Steam = config.Steam{
			APIKey:           "test-steam-api-key",
			OpenIDLoginURL:   "https://steamcommunity.com/openid/login",
			PlayerSummaryURL: "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/",
		}
	})
}

func TestAU50OAuthHandlerLoginHappyPath(t *testing.T) {
	testutil.SwapTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
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
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?code=disc-code&state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()
	as := &stubAccountService{}
	las := &stubLinkAccountStore{}
	ss := &authtest.FakeSessionService{CreateJWTFunc: func(*session.Session) (string, error) { return "test-jwt", nil }}

	t.Run("AU-50_OAuthHandlerLoginHappyPath", func(t *testing.T) {
		OAuthHandler(auConfig(), as, las, ss, auSiteURL)(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
		}
		if loc := w.Header().Get("Location"); loc != state.RedirectURI {
			t.Errorf("expected redirect straight to %q with no problem param, got %q", state.RedirectURI, loc)
		}
		cookie := testutil.FindCookie(w, session.CookieName)
		if cookie == nil || cookie.Value != "test-jwt" {
			t.Errorf("expected session cookie with value %q, got %+v", "test-jwt", cookie)
		}
	})
}

func TestAU51OpenIDHandlerLoginHappyPath(t *testing.T) {
	testutil.SwapTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
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
	r := testutil.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()
	as := &stubAccountService{}
	las := &stubLinkAccountStore{}
	ss := &authtest.FakeSessionService{CreateJWTFunc: func(*session.Session) (string, error) { return "test-jwt", nil }}

	t.Run("AU-51_OpenIDHandlerLoginHappyPath", func(t *testing.T) {
		OpenIDHandler(auConfig().Steam, as, las, ss, auSiteURL)(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
		}
		if loc := w.Header().Get("Location"); loc != state.RedirectURI {
			t.Errorf("expected redirect straight to %q with no problem param, got %q", state.RedirectURI, loc)
		}
		cookie := testutil.FindCookie(w, session.CookieName)
		if cookie == nil || cookie.Value != "test-jwt" {
			t.Errorf("expected session cookie with value %q, got %+v", "test-jwt", cookie)
		}
	})
}

func auDiscordTransport(t *testing.T) {
	t.Helper()
	testutil.SwapTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
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
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?code=disc-code&state="+encodeState(t, state), nil)
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
	testutil.SwapTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
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
	r := testutil.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: state.Nonce})
	return r
}

func TestAU52to56OAuthAndOpenIDFailuresHideCause(t *testing.T) {
	oauthState := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	steamState := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	okJWT := func(*session.Session) (string, error) { return "test-jwt", nil }

	t.Run("AU-52_OAuthProcessFails", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()
		ss := &authtest.FakeSessionService{CreateJWTFunc: okJWT, AddSessionErr: testutil.ErrDBDown}

		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, ss, auSiteURL)(w, auOAuthRequest(t, oauthState))

		requireProblemRedirect(t, w, oauthState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-54_OpenIDVerifyFailsWithoutRejectingAssertion", func(t *testing.T) {
		auSteamTransport(t, http.StatusInternalServerError, http.StatusOK)
		w := httptest.NewRecorder()
		ss := &authtest.FakeSessionService{CreateJWTFunc: okJWT}

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, ss, auSiteURL)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-55_OpenIDGetSteamUserFails", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusInternalServerError)
		w := httptest.NewRecorder()
		ss := &authtest.FakeSessionService{CreateJWTFunc: okJWT}

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, ss, auSiteURL)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-56_OpenIDProcessFails", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()
		ss := &authtest.FakeSessionService{CreateJWTFunc: okJWT, AddSessionErr: testutil.ErrDBDown}

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, ss, auSiteURL)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusInternalServerError, msgAuthenticationFailed)
	})
}

func TestAU17OpenIDHandlerMissingState(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/api/openid", nil)
	w := httptest.NewRecorder()

	t.Run("AU-17_OpenIDMissingState", func(t *testing.T) {
		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{}, auSiteURL)(w, r)

		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidRequest)
	})
}

func TestAU18OpenIDHandlerLinkModeNoSession(t *testing.T) {
	state := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLink}
	stateB64 := encodeState(t, state)
	r := testutil.NewRequest(http.MethodGet, "/api/openid?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-18_OpenIDLinkModeNoSession", func(t *testing.T) {
		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{}, auSiteURL)(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
	})
}

func TestAU19OpenIDHandlerBadOpenIDMode(t *testing.T) {
	state := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	q := url.Values{"state": {stateB64}, "openid.mode": {"cancel"}}
	r := testutil.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-19_OpenIDBadOpenIDMode", func(t *testing.T) {
		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{}, auSiteURL)(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU20OpenIDHandlerBadClaimedID(t *testing.T) {
	state := OAuthState{Platform: PlatformSteam, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	q := url.Values{"state": {stateB64}, "openid.mode": {"id_res"}, "openid.claimed_id": {"not-a-steam-id-url"}}
	r := testutil.NewRequest(http.MethodGet, "/api/openid?"+q.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-20_OpenIDBadClaimedID", func(t *testing.T) {
		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{}, auSiteURL)(w, r)

		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU22DecodeAndValidateStateHappyPath(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-22_DecodeAndValidateStateHappyPath", func(t *testing.T) {
		got, ok := decodeAndValidateState(w, r, auSiteURL)
		if !ok {
			t.Fatalf("expected ok=true, got false (body: %s)", w.Body.String())
		}
		if got != state {
			t.Errorf("expected state %+v, got %+v", state, got)
		}
	})
}

func TestAU23DecodeAndValidateStateMissingParam(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-23_DecodeAndValidateStateMissingParam", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r, auSiteURL)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidRequest)
	})
}

func TestAU24DecodeAndValidateStateInvalidBase64(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?state=not-valid-base64!!!", nil)
	w := httptest.NewRecorder()

	t.Run("AU-24_DecodeAndValidateStateInvalidBase64", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r, auSiteURL)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU25DecodeAndValidateStateInvalidJSON(t *testing.T) {
	stateB64 := base64.URLEncoding.EncodeToString([]byte("not json"))
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	w := httptest.NewRecorder()

	t.Run("AU-25_DecodeAndValidateStateInvalidJSON", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r, auSiteURL)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU26DecodeAndValidateStateMissingRequiredField(t *testing.T) {
	state := OAuthState{Platform: "", Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-26_DecodeAndValidateStateMissingRequiredField", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r, auSiteURL)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU27DecodeAndValidateStateDisallowedRedirect(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://evil.example.com/phish", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "test-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-27_DecodeAndValidateStateDisallowedRedirect", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r, auSiteURL)
		if ok {
			t.Fatal("expected ok=false")
		}
		if loc := w.Header().Get("Location"); strings.Contains(loc, "evil.example.com") {
			t.Fatalf("expected no redirect to the attacker's URL, got Location: %q", loc)
		}
		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU28DecodeAndValidateStateMissingNonceCookie(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	w := httptest.NewRecorder()

	t.Run("AU-28_DecodeAndValidateStateMissingNonceCookie", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r, auSiteURL)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU29DecodeAndValidateStateNonceMismatch(t *testing.T) {
	state := OAuthState{Platform: PlatformDiscord, Nonce: "test-nonce", RedirectURI: "https://neuralnexus.test/done", Mode: ModeLogin}
	stateB64 := encodeState(t, state)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth?state="+stateB64, nil)
	r.AddCookie(&http.Cookie{Name: "nonce", Value: "wrong-nonce"})
	w := httptest.NewRecorder()

	t.Run("AU-29_DecodeAndValidateStateNonceMismatch", func(t *testing.T) {
		_, ok := decodeAndValidateState(w, r, auSiteURL)
		if ok {
			t.Fatal("expected ok=false")
		}
		requireProblemRedirect(t, w, auSiteURL, http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU30RequireValidModeAndSessionLoginMode(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil)
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
	ctx := context.WithValue(context.Background(), reqctx.SessionKey, session)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
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
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-32_RequireValidModeAndSessionLinkModeNoSessionKey", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false with no session in context")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
	})
}

func TestAU33RequireValidModeAndSessionLinkModeNilSession(t *testing.T) {
	ctx := context.WithValue(context.Background(), reqctx.SessionKey, (*session.Session)(nil))
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
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
	ctx := context.WithValue(context.Background(), reqctx.SessionKey, session)
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	t.Run("AU-34_RequireValidModeAndSessionLinkModeExpiredSession", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, ModeLink, "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for an expired session")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusUnauthorized, msgLoginRequiredToLink)
	})
}

func TestAU35RequireValidModeAndSessionUnrecognizedMode(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/api/oauth", nil)
	w := httptest.NewRecorder()

	t.Run("AU-35_RequireValidModeAndSessionUnrecognizedMode", func(t *testing.T) {
		if ok := requireValidModeAndSession(w, r, Mode("bogus-mode"), "https://neuralnexus.test/done"); ok {
			t.Fatal("expected false for an unrecognized mode")
		}
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, msgInvalidState)
	})
}

func TestAU38RedirectWithErrorHappyPath(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	t.Run("AU-38_RedirectWithErrorHappyPath", func(t *testing.T) {
		redirectWithError(w, r, "https://neuralnexus.test/done", http.StatusBadRequest, "Bad Request", "something broke")
		requireProblemRedirect(t, w, "https://neuralnexus.test/done", http.StatusBadRequest, "something broke")
	})
}

func TestAU39RedirectWithErrorUnparseableTarget(t *testing.T) {
	r := testutil.NewRequest(http.MethodGet, "/", nil)
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

func TestAU44IsAllowedRedirectSameOrigin(t *testing.T) {
	t.Run("AU-44_IsAllowedRedirectSameOrigin", func(t *testing.T) {
		if !isAllowedRedirect(auSiteURL+"/some/path", auSiteURL) {
			t.Error("expected true for a same-origin redirect URI")
		}
	})
}

func TestAU45IsAllowedRedirectDifferentHost(t *testing.T) {
	t.Run("AU-45_IsAllowedRedirectDifferentHost", func(t *testing.T) {
		if isAllowedRedirect("https://evil.example.com/phish", auSiteURL) {
			t.Error("expected false for a different host")
		}
	})
}

func TestAU46IsAllowedRedirectDifferentScheme(t *testing.T) {
	siteURL, err := url.Parse(auSiteURL)
	if err != nil {
		t.Fatalf("failed to parse the site URL: %v", err)
	}

	t.Run("AU-46_IsAllowedRedirectDifferentScheme", func(t *testing.T) {
		if isAllowedRedirect("http://"+siteURL.Host+"/done", auSiteURL) {
			t.Error("expected false for a different scheme")
		}
	})
}

func TestAU47IsAllowedRedirectUnparseableRedirectURI(t *testing.T) {
	t.Run("AU-47_IsAllowedRedirectUnparseableRedirectURI", func(t *testing.T) {
		if isAllowedRedirect("http://example.com/%zz", auSiteURL) {
			t.Error("expected false for an unparseable redirect URI")
		}
	})
}

func TestAU48IsAllowedRedirectUnparseableSiteURL(t *testing.T) {
	t.Run("AU-48_IsAllowedRedirectUnparseableSiteURL", func(t *testing.T) {
		if isAllowedRedirect("https://neuralnexus.test/done", "http://example.com/%zz") {
			t.Error("expected false when the site URL itself fails to parse")
		}
	})
}

func auWithSession(r *http.Request, userID string) *http.Request {
	session := &session.Session{ID: "link-session", UserID: userID}
	return r.WithContext(context.WithValue(r.Context(), reqctx.SessionKey, session))
}

func auDiscordTransportWithoutScope(t *testing.T) {
	t.Helper()
	testutil.SwapTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost && req.URL.Host == "discord.com" && req.URL.Path == "/api/oauth2/token" {
			return auJSONResponse(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600}`), nil
		}
		return nil, fmt.Errorf("unexpected outbound request %s %s", req.Method, req.URL.String())
	}))
}

func auMicrosoftTransport(t *testing.T) {
	t.Helper()
	testutil.SwapTransport(t, auRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
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
	if key != reqctx.SessionKey {
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

		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auOAuthRequest(t, state))

		requireProblemRedirect(t, w, redirect, http.StatusBadRequest, msgInvalidPlatform)
	})

	t.Run("AU-61_OAuthTokenWithoutScope", func(t *testing.T) {
		auDiscordTransportWithoutScope(t)
		w := httptest.NewRecorder()

		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auOAuthRequest(t, discordLogin))

		requireProblemRedirect(t, w, redirect, http.StatusBadRequest, msgMissingOAuthScope)
	})

	t.Run("AU-62_OpenIDSteamIDMismatch", func(t *testing.T) {
		auSteamTransportBody(t, http.StatusOK, http.StatusOK, `{"response":{"players":[{"steamid":"76561198999999999","personaname":"someone"}]}}`)
		w := httptest.NewRecorder()

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auOpenIDRequest(t, steamLogin))

		requireProblemRedirect(t, w, redirect, http.StatusBadRequest, msgSteamAccountMismatch)
	})

	t.Run("AU-63_OAuthLinkedToDifferentAccount", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()

		OAuthHandler(auConfig(), &stubAccountService{}, linkedElsewhere, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSession(auOAuthRequest(t, discordLink), "u1"))

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

		OAuthHandler(auConfig(), &stubAccountService{}, las, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auOAuthRequest(t, state))

		requireProblemRedirect(t, w, redirect, http.StatusConflict, msgConflictingMicrosoftIdentities)
	})

	t.Run("AU-65_OAuthLinkFails", func(t *testing.T) {
		auDiscordTransport(t)
		w := httptest.NewRecorder()

		OAuthHandler(auConfig(), &stubAccountService{}, addFails(testutil.ErrDBDown), &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSession(auOAuthRequest(t, discordLink), "u1"))

		requireProblemRedirect(t, w, redirect, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	t.Run("AU-67_OpenIDLinkedToDifferentAccount", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, linkedElsewhere, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSession(auOpenIDRequest(t, steamLink), "u1"))

		requireProblemRedirect(t, w, redirect, http.StatusConflict, msgPlatformAlreadyLinked)
	})

	t.Run("AU-68_OpenIDLinkFails", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, addFails(testutil.ErrDBDown), &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSession(auOpenIDRequest(t, steamLink), "u1"))

		requireProblemRedirect(t, w, redirect, http.StatusInternalServerError, msgAuthenticationFailed)
	})

	expired := &session.Session{ID: "link-session", UserID: "u1", ExpiresAt: 1}

	t.Run("AU-66_OAuthSessionGoneBeforeLinking", func(t *testing.T) {
		w := httptest.NewRecorder()

		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSessionChange(auOAuthRequest(t, discordLink), nil))

		requireProblemRedirect(t, w, redirect, http.StatusUnauthorized, msgLoginRequiredToLink)
	})

	t.Run("AU-69_OAuthSessionExpiredBeforeLinking", func(t *testing.T) {
		w := httptest.NewRecorder()

		OAuthHandler(auConfig(), &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSessionChange(auOAuthRequest(t, discordLink), expired))

		requireProblemRedirect(t, w, redirect, http.StatusUnauthorized, msgSessionExpired)
	})

	t.Run("AU-70_OpenIDSessionGoneBeforeLinking", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSessionChange(auOpenIDRequest(t, steamLink), nil))

		requireProblemRedirect(t, w, redirect, http.StatusUnauthorized, msgLoginRequiredToLink)
	})

	t.Run("AU-71_OpenIDSessionExpiredBeforeLinking", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, &authtest.FakeSessionService{CreateJWTFunc: okJWT}, auSiteURL)(w, auWithSessionChange(auOpenIDRequest(t, steamLink), expired))

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
		ss := &authtest.FakeSessionService{CreateJWTFunc: okJWT}

		OAuthHandler(auConfig(), &stubAccountService{}, disabledLink, ss, auSiteURL)(w, auOAuthRequest(t, oauthState))

		requireProblemRedirect(t, w, oauthState.RedirectURI, http.StatusForbidden, msgPlatformLoginDisabled)
	})

	t.Run("AU-73_OpenIDSteamUserMapsNoPlayers", func(t *testing.T) {
		auSteamTransportBody(t, http.StatusOK, http.StatusOK, `{"response":{"players":[]}}`)
		w := httptest.NewRecorder()
		ss := &authtest.FakeSessionService{CreateJWTFunc: okJWT}

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, &stubLinkAccountStore{}, ss, auSiteURL)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusNotFound, msgSteamAccountNotFound)
	})

	t.Run("AU-74_OpenIDProcessMapsLoginDisabled", func(t *testing.T) {
		auSteamTransport(t, http.StatusOK, http.StatusOK)
		w := httptest.NewRecorder()
		ss := &authtest.FakeSessionService{CreateJWTFunc: okJWT}

		OpenIDHandler(auConfig().Steam, &stubAccountService{}, disabledLink, ss, auSiteURL)(w, auOpenIDRequest(t, steamState))

		requireProblemRedirect(t, w, steamState.RedirectURI, http.StatusForbidden, msgPlatformLoginDisabled)
	})
}

type stubService struct {
	links    []*LinkedAccount
	linksErr error

	unlinkErr   error
	unlinkCalls []Platform

	setEnableErr   error
	setEnableCalls []bool
}

var _ Service = (*stubService)(nil)

func (s *stubService) GetUserLinkedAccounts(context.Context, string) ([]*LinkedAccount, error) {
	return s.links, s.linksErr
}
func (s *stubService) UnlinkPlatform(_ context.Context, _ string, platform Platform) error {
	s.unlinkCalls = append(s.unlinkCalls, platform)
	return s.unlinkErr
}
func (s *stubService) SetPlatformLoginEnabled(_ context.Context, _ string, _ Platform, enabled bool) error {
	s.setEnableCalls = append(s.setEnableCalls, enabled)
	return s.setEnableErr
}
func (s *stubService) UpdateFromPlatform(context.Context, Platform, string, PlatformData) (string, error) {
	return "", nil
}

func TestUR28GetUserLinkedAccountsHandlerSelfHappyPath(t *testing.T) {
	svc := &stubService{links: []*LinkedAccount{{Platform: PlatformDiscord}}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("LKU-28_GetUserLinkedAccountsSelfHappyPath", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR29GetUserLinkedAccountsHandlerAdminHappyPath(t *testing.T) {
	svc := &stubService{links: []*LinkedAccount{{Platform: PlatformDiscord}}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.AdminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("LKU-29_GetUserLinkedAccountsAdminHappyPath", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR30GetUserLinkedAccountsHandlerForbidden(t *testing.T) {
	svc := &stubService{}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("LKU-30_GetUserLinkedAccountsForbidden", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToViewLinkedAccounts)
	})
}

func TestUR31GetUserLinkedAccountsHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubService{linksErr: testutil.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("LKU-31_GetUserLinkedAccountsServiceErrorMapsTo500", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetLinkedAccounts)
	})
}

func TestUR32UnlinkPlatformHandlerHappyPath(t *testing.T) {
	svc := &stubService{}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.SelfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("LKU-32_UnlinkPlatformHappyPath", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNoContent)
		if len(svc.unlinkCalls) != 1 || svc.unlinkCalls[0] != PlatformDiscord {
			t.Errorf("expected UnlinkPlatform(discord) to be called, got %v", svc.unlinkCalls)
		}
	})
}

func TestUR33UnlinkPlatformHandlerForbidden(t *testing.T) {
	svc := &stubService{}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.SelfSession("u1"), "someone-else", "discord", "")
	w := httptest.NewRecorder()

	t.Run("LKU-33_UnlinkPlatformForbidden", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToUnlinkPlatforms)
		if len(svc.unlinkCalls) != 0 {
			t.Error("expected UnlinkPlatform to never be called for a forbidden request")
		}
	})
}

func TestUR34UnlinkPlatformHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubService{unlinkErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.SelfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("LKU-34_UnlinkPlatformNotFoundMapsTo404", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlatformNotLinked)
	})
}

func TestUR35UnlinkPlatformHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubService{unlinkErr: account.ErrWouldLockAccount}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.SelfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("LKU-35_UnlinkPlatformWouldLockAccountMapsTo400", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgSetPasswordOrLinkBeforeUnlinking)
	})
}

func TestUR36UnlinkPlatformHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubService{unlinkErr: testutil.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.SelfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("LKU-36_UnlinkPlatformUnclassifiedErrorMapsTo500", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToUnlinkPlatform)
	})
}

func TestUR37SetPlatformLoginEnabledHandlerHappyPath(t *testing.T) {
	svc := &stubService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("LKU-37_SetPlatformLoginEnabledHappyPath", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNoContent)
		if len(svc.setEnableCalls) != 1 || svc.setEnableCalls[0] != true {
			t.Errorf("expected SetPlatformLoginEnabled(true), got %v", svc.setEnableCalls)
		}
	})
}

func TestUR38SetPlatformLoginEnabledHandlerForbidden(t *testing.T) {
	svc := &stubService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "someone-else", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("LKU-38_SetPlatformLoginEnabledForbidden", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdatePlatforms)
	})
}

func TestUR39SetPlatformLoginEnabledHandlerMalformedBody(t *testing.T) {
	svc := &stubService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "discord", `not json`)
	w := httptest.NewRecorder()

	t.Run("LKU-39_SetPlatformLoginEnabledMalformedBody", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setEnableCalls) != 0 {
			t.Error("expected SetPlatformLoginEnabled to never be called for a malformed body")
		}
	})
}

func TestUR40SetPlatformLoginEnabledHandlerMissingField(t *testing.T) {
	svc := &stubService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "discord", `{}`)
	w := httptest.NewRecorder()

	t.Run("LKU-40_SetPlatformLoginEnabledMissingField", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setEnableCalls) != 0 {
			t.Error("expected SetPlatformLoginEnabled to never be called when login_enabled is omitted")
		}
	})
}

func TestUR41SetPlatformLoginEnabledHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubService{setEnableErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("LKU-41_SetPlatformLoginEnabledNotFoundMapsTo404", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlatformNotLinked)
	})
}

func TestUR42SetPlatformLoginEnabledHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubService{setEnableErr: account.ErrWouldLockAccount}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "discord", `{"login_enabled":false}`)
	w := httptest.NewRecorder()

	t.Run("LKU-42_SetPlatformLoginEnabledWouldLockAccountMapsTo400", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgSetPasswordOrLinkBeforeDisabling)
	})
}

func TestUR43SetPlatformLoginEnabledHandlerUnverifiedMapsTo400(t *testing.T) {
	svc := &stubService{setEnableErr: ErrLinkedAccountUnverified}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("LKU-43_SetPlatformLoginEnabledUnverifiedMapsTo400", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgLinkedAccountUnverified)
	})
}

func TestUR44SetPlatformLoginEnabledHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubService{setEnableErr: testutil.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("LKU-44_SetPlatformLoginEnabledUnclassifiedErrorMapsTo500", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToUpdatePlatform)
	})
}
