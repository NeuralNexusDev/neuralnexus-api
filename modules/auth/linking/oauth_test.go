package linking

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

func mustUUID(s string) uuid.UUID {
	return uuid.MustParse(s)
}

type oaIdentity struct {
	id       string
	email    string
	username string
}

func (f *oaIdentity) GetID() string       { return f.id }
func (f *oaIdentity) GetEmail() string    { return f.email }
func (f *oaIdentity) GetUsername() string { return f.username }
func (f *oaIdentity) GetData() string     { return "{}" }
func (f *oaIdentity) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformDiscord, f.username, f.id, f)
}

type oaMockAccountService struct {
	AddAccountFunc     func(*auth.Account) error
	GetAccountByIDFunc func(string) (*auth.Account, error)
	DeleteAccountFunc  func(string) error

	AddAccountCalls     int
	GetAccountByIDCalls int
	DeleteAccountCalls  int
}

var _ auth.AccountService = (*oaMockAccountService)(nil)

func (m *oaMockAccountService) AddAccount(a *auth.Account) error {
	m.AddAccountCalls++
	if m.AddAccountFunc != nil {
		return m.AddAccountFunc(a)
	}
	return nil
}
func (m *oaMockAccountService) GetAccountByID(userID string) (*auth.Account, error) {
	m.GetAccountByIDCalls++
	if m.GetAccountByIDFunc != nil {
		return m.GetAccountByIDFunc(userID)
	}
	return nil, auth.ErrNotFound
}
func (m *oaMockAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *oaMockAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *oaMockAccountService) UpdateAccount(*auth.Account) error { return nil }
func (m *oaMockAccountService) DeleteAccount(userID string) error {
	m.DeleteAccountCalls++
	if m.DeleteAccountFunc != nil {
		return m.DeleteAccountFunc(userID)
	}
	return nil
}
func (m *oaMockAccountService) IsPasswordAuthEnabled(string) (bool, error) { return false, nil }

type oaMockLinkAccountStore struct {
	GetLinkedAccountByPlatformIDFunc func(auth.Platform, string) (*auth.LinkedAccount, error)
	AddLinkedAccountToDBFunc         func(*auth.LinkedAccount) error

	GetLinkedAccountByPlatformIDCalls int
	AddLinkedAccountToDBCalls         int
	AddedLinkedAccounts               []*auth.LinkedAccount
}

var _ auth.LinkAccountStore = (*oaMockLinkAccountStore)(nil)

func (m *oaMockLinkAccountStore) AddLinkedAccountToDB(la *auth.LinkedAccount) error {
	m.AddLinkedAccountToDBCalls++
	if m.AddLinkedAccountToDBFunc != nil {
		if err := m.AddLinkedAccountToDBFunc(la); err != nil {
			return err
		}
	}
	m.AddedLinkedAccounts = append(m.AddedLinkedAccounts, la)
	return nil
}
func (m *oaMockLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error { return nil }
func (m *oaMockLinkAccountStore) GetLinkedAccountByPlatformID(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
	m.GetLinkedAccountByPlatformIDCalls++
	if m.GetLinkedAccountByPlatformIDFunc != nil {
		return m.GetLinkedAccountByPlatformIDFunc(platform, platformID)
	}
	return nil, auth.ErrNotFound
}
func (m *oaMockLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *oaMockLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *oaMockLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*auth.LinkedAccount, error) {
	return nil, nil
}
func (m *oaMockLinkAccountStore) DeleteLinkedAccount(string, auth.Platform) error { return nil }
func (m *oaMockLinkAccountStore) SetLinkedAccountLoginEnabled(string, auth.Platform, bool) error {
	return nil
}

type oaMockSessionService struct {
	AddSessionFunc  func(*auth.Session) error
	AddSessionCalls int
}

var _ auth.SessionService = (*oaMockSessionService)(nil)

func (m *oaMockSessionService) AddSession(session *auth.Session) error {
	m.AddSessionCalls++
	if m.AddSessionFunc != nil {
		return m.AddSessionFunc(session)
	}
	return nil
}
func (m *oaMockSessionService) GetSession(string) (*auth.Session, error) {
	return nil, auth.ErrNotFound
}
func (m *oaMockSessionService) UpdateSession(*auth.Session) error       { return nil }
func (m *oaMockSessionService) DeleteSession(string) error              { return nil }
func (m *oaMockSessionService) CreateJWT(*auth.Session) (string, error) { return "", nil }
func (m *oaMockSessionService) ReadJWT(string) (*auth.Session, error)   { return nil, auth.ErrNotFound }

func oaRequestWithSession(session *auth.Session) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if session == nil {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), mw.SessionKey, session))
}

func oaSetVar(t *testing.T, target *string, value string) {
	t.Helper()
	original := *target
	*target = value
	t.Cleanup(func() { *target = original })
}

func oaJSONServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// oaSetDiscordUsersEndpoint points discordgo's EndpointUsers at ts for the
// duration of the calling test (see discord_test.go for why this works).
func oaSetDiscordUsersEndpoint(t *testing.T, ts *httptest.Server) {
	t.Helper()
	original := discordgo.EndpointUsers
	discordgo.EndpointUsers = ts.URL + "/users/"
	t.Cleanup(func() { discordgo.EndpointUsers = original })
}

func oaSetupDiscordPlatform(t *testing.T) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600,"scope":"identify"}`)
	t.Cleanup(tokenTS.Close)
	usersTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"d1","username":"alice","email":"a@b.com"}`))
	}))
	t.Cleanup(usersTS.Close)
	oaSetDiscordUsersEndpoint(t, usersTS)

	originalTokenURL := discordConfig.Endpoint.TokenURL
	discordConfig.Endpoint.TokenURL = tokenTS.URL
	t.Cleanup(func() { discordConfig.Endpoint.TokenURL = originalTokenURL })
}

func oaSetupDiscordExchangeFailure(t *testing.T) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusBadRequest, `{"error":"invalid_grant"}`)
	t.Cleanup(tokenTS.Close)
	originalTokenURL := discordConfig.Endpoint.TokenURL
	discordConfig.Endpoint.TokenURL = tokenTS.URL
	t.Cleanup(func() { discordConfig.Endpoint.TokenURL = originalTokenURL })
}

func oaSetupTwitchPlatform(t *testing.T) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"twitch-at","token_type":"bearer","expires_in":14400,"scope":["user:read:email"]}`)
	t.Cleanup(tokenTS.Close)
	originalTokenURL := twitch.Config.Endpoint.TokenURL
	twitch.Config.Endpoint.TokenURL = tokenTS.URL
	t.Cleanup(func() { twitch.Config.Endpoint.TokenURL = originalTokenURL })

	usersTS := oaJSONServer(http.StatusOK, `{"data":[{"id":"tw1","login":"alice","email":"a@b.com"}]}`)
	t.Cleanup(usersTS.Close)
	originalAPIBaseURL := twitch.APIBaseURL
	twitch.APIBaseURL = usersTS.URL
	t.Cleanup(func() { twitch.APIBaseURL = originalAPIBaseURL })

	// helix.NewClient requires a ClientID to construct at all, regardless of
	// APIBaseURL, so fill one in if TWITCH_CLIENT_ID isn't set.
	if twitch.CLIENT_ID == "" {
		twitch.CLIENT_ID = "test-client-id"
		t.Cleanup(func() { twitch.CLIENT_ID = "" })
	}
}

func oaSetupMicrosoftLoginPlatform(t *testing.T) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"ms-login-at","token_type":"Bearer","expires_in":3600,"scope":"openid profile email"}`)
	t.Cleanup(tokenTS.Close)
	originalTokenURL := MicrosoftLoginConfig.Endpoint.TokenURL
	MicrosoftLoginConfig.Endpoint.TokenURL = tokenTS.URL
	t.Cleanup(func() { MicrosoftLoginConfig.Endpoint.TokenURL = originalTokenURL })

	userInfoTS := oaJSONServer(http.StatusOK, `{"sub":"ms1","name":"Alice","email":"a@b.com"}`)
	t.Cleanup(userInfoTS.Close)
	oaSetVar(t, &microsoftUserInfoURL, userInfoTS.URL)
}

type oaMicrosoftChainOpts struct {
	javaUUID          string
	xuid, gamertag    string
	xstsFails         bool
	mcLoginFails      bool
	mcProfileFails    bool
	xstsXboxLiveFails bool
}

func oaSetupMicrosoftChain(t *testing.T, opts oaMicrosoftChainOpts) string {
	t.Helper()
	xuid := opts.xuid
	if xuid == "" {
		xuid = "xid1"
	}
	gamertag := opts.gamertag
	if gamertag == "" {
		gamertag = "Tag"
	}

	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"ms-at","token_type":"Bearer","expires_in":3600,"scope":["XboxLive.signin","offline_access"]}`)
	t.Cleanup(tokenTS.Close)
	originalTokenURL := MicrosoftConfig.Endpoint.TokenURL
	MicrosoftConfig.Endpoint.TokenURL = tokenTS.URL
	t.Cleanup(func() { MicrosoftConfig.Endpoint.TokenURL = originalTokenURL })

	xblTS := oaJSONServer(http.StatusOK, `{"Token":"xbl-t"}`)
	t.Cleanup(xblTS.Close)
	oaSetVar(t, &xboxLiveAuthenticateURL, xblTS.URL)

	if opts.xstsFails {
		xstsTS := oaJSONServer(http.StatusInternalServerError, `{}`)
		t.Cleanup(xstsTS.Close)
		oaSetVar(t, &xstsAuthorizeURL, xstsTS.URL)
		return xuid
	}

	xstsTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RelyingParty string `json:"RelyingParty"`
		}
		_ = readJSONBody(r, &body)
		w.Header().Set("Content-Type", "application/json")
		if body.RelyingParty == xstsXboxLiveRelyingParty {
			if opts.xstsXboxLiveFails {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"Token":"xsts-xbl-t","DisplayClaims":{"xui":[{"uhs":"hash1","xid":%q,"gtg":%q}]}}`, xuid, gamertag)))
			return
		}
		_, _ = w.Write([]byte(`{"Token":"xsts-mc-t","DisplayClaims":{"xui":[{"uhs":"hash1"}]}}`))
	}))
	t.Cleanup(xstsTS.Close)
	oaSetVar(t, &xstsAuthorizeURL, xstsTS.URL)

	mcLoginTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if opts.mcLoginFails {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"mc-t"}`))
	}))
	t.Cleanup(mcLoginTS.Close)
	oaSetVar(t, &minecraftLoginURL, mcLoginTS.URL)

	mcProfileTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if opts.mcProfileFails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if opts.javaUUID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"name":"Steve","skins":[],"capes":[]}`, opts.javaUUID)))
	}))
	t.Cleanup(mcProfileTS.Close)
	oaSetVar(t, &minecraftProfileURL, mcProfileTS.URL)

	return xuid
}

func TestOA01to04ExtCodeForToken(t *testing.T) {
	t.Run("OA-01_ArrayScope", func(t *testing.T) {
		ts := oaJSONServer(http.StatusOK, `{"access_token":"AT","token_type":"Bearer","refresh_token":"RT","expires_in":3600,"scope":["identify","email"]}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		got, err := ExtCodeForToken(config, "code")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AccessToken != "AT" || got.TokenType != "Bearer" || got.RefreshToken != "RT" {
			t.Errorf("token = %+v, want AccessToken=AT TokenType=Bearer RefreshToken=RT", got)
		}
		if len(got.Scope) != 2 || got.Scope[0] != "identify" || got.Scope[1] != "email" {
			t.Errorf("Scope = %v, want [identify email]", got.Scope)
		}
	})

	t.Run("OA-02_StringScope", func(t *testing.T) {
		ts := oaJSONServer(http.StatusOK, `{"access_token":"AT","token_type":"Bearer","expires_in":3600,"scope":"identify"}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		got, err := ExtCodeForToken(config, "code")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Scope) != 1 || got.Scope[0] != "identify" {
			t.Errorf("Scope = %v, want [identify]", got.Scope)
		}
	})

	t.Run("OA-03_ExchangeFails", func(t *testing.T) {
		ts := oaJSONServer(http.StatusBadRequest, `{"error":"invalid_grant"}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		got, err := ExtCodeForToken(config, "bad-code")
		if err == nil {
			t.Fatal("expected an error")
		}
		if got != nil {
			t.Errorf("expected nil token on error, got %+v", got)
		}
	})

	t.Run("OA-04_MissingScope", func(t *testing.T) {
		ts := oaJSONServer(http.StatusOK, `{"access_token":"AT","token_type":"Bearer","expires_in":3600}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		_, err := ExtCodeForToken(config, "code")
		if err == nil || err.Error() != "failed to get scope from token" {
			t.Fatalf("ExtCodeForToken() error = %v, want \"failed to get scope from token\"", err)
		}
	})
}

func TestOA05to08RefreshToken(t *testing.T) {
	expiredToken := func() *oauth2.Token {
		return &oauth2.Token{AccessToken: "old-at", RefreshToken: "old-rt", Expiry: time.Now().Add(-time.Hour)}
	}

	t.Run("OA-05_ArrayScope", func(t *testing.T) {
		ts := oaJSONServer(http.StatusOK, `{"access_token":"NEW","token_type":"Bearer","refresh_token":"NEWRT","expires_in":3600,"scope":["identify","email"]}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		got, err := RefreshToken(config, expiredToken())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AccessToken != "NEW" || len(got.Scope) != 2 {
			t.Errorf("token = %+v, want AccessToken=NEW Scope=[identify email]", got)
		}
	})

	t.Run("OA-06_StringScope", func(t *testing.T) {
		ts := oaJSONServer(http.StatusOK, `{"access_token":"NEW","token_type":"Bearer","expires_in":3600,"scope":"identify"}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		got, err := RefreshToken(config, expiredToken())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Scope) != 1 || got.Scope[0] != "identify" {
			t.Errorf("Scope = %v, want [identify]", got.Scope)
		}
	})

	t.Run("OA-07_RefreshFails", func(t *testing.T) {
		ts := oaJSONServer(http.StatusBadRequest, `{"error":"invalid_grant"}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		got, err := RefreshToken(config, expiredToken())
		if err == nil {
			t.Fatal("expected an error")
		}
		if got != nil {
			t.Errorf("expected nil token on error, got %+v", got)
		}
	})

	t.Run("OA-08_MissingScope", func(t *testing.T) {
		ts := oaJSONServer(http.StatusOK, `{"access_token":"NEW","token_type":"Bearer","expires_in":3600}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		_, err := RefreshToken(config, expiredToken())
		if err == nil || err.Error() != "failed to get scope from token" {
			t.Fatalf("RefreshToken() error = %v, want \"failed to get scope from token\"", err)
		}
	})
}

func TestOA09to16ProcessOAuthLogin(t *testing.T) {
	t.Run("OA-09_DiscordCreatesAccount", func(t *testing.T) {
		oaSetupDiscordPlatform(t)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		session, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if as.AddAccountCalls != 1 {
			t.Errorf("AddAccount called %d times, want 1", as.AddAccountCalls)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if ss.AddSessionCalls != 1 {
			t.Errorf("AddSession called %d times, want 1", ss.AddSessionCalls)
		}
	})

	t.Run("OA-10_DiscordReusesExistingLinkedAccount", func(t *testing.T) {
		oaSetupDiscordPlatform(t)
		existing := &auth.Account{UserID: "user-1"}
		as := &oaMockAccountService{
			GetAccountByIDFunc: func(id string) (*auth.Account, error) {
				if id == "user-1" {
					return existing, nil
				}
				return nil, auth.ErrNotFound
			},
		}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "user-1", Platform: auth.PlatformDiscord, PlatformID: "d1", Verified: true, LoginEnabled: true}, nil
			},
		}
		ss := &oaMockSessionService{}

		session, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil || session.UserID != "user-1" {
			t.Errorf("session = %+v, want UserID=user-1", session)
		}
		if as.AddAccountCalls != 0 {
			t.Errorf("AddAccount called %d times, want 0 (existing account reused)", as.AddAccountCalls)
		}
	})

	t.Run("OA-11_InvalidPlatform", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		_, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: "bogus", Mode: ModeLogin})
		if err == nil || err.Error() != "invalid platform" {
			t.Fatalf("ProcessOAuthLogin() error = %v, want \"invalid platform\"", err)
		}
		if as.AddAccountCalls != 0 || las.AddLinkedAccountToDBCalls != 0 || ss.AddSessionCalls != 0 {
			t.Error("expected no store calls for an invalid platform")
		}
	})

	t.Run("OA-12_ExchangeFails", func(t *testing.T) {
		oaSetupDiscordExchangeFailure(t)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		_, err := ProcessOAuthLogin(as, las, ss, "bad-code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLogin})
		if err == nil {
			t.Fatal("expected an error")
		}
		if as.AddAccountCalls != 0 || las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no store calls when the code exchange fails")
		}
	})

	t.Run("OA-13_LinkedAccountLoginDisabled", func(t *testing.T) {
		oaSetupDiscordPlatform(t)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "user-1", Platform: auth.PlatformDiscord, PlatformID: "d1", Verified: true, LoginEnabled: false}, nil
			},
		}
		ss := &oaMockSessionService{}

		_, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLogin})
		if !errors.Is(err, errPlatformLoginDisabled) {
			t.Fatalf("ProcessOAuthLogin() error = %v, want errPlatformLoginDisabled", err)
		}
	})

	t.Run("OA-14_MinecraftFullChainNewAccount", func(t *testing.T) {
		javaUUID := "11111111-1111-1111-1111-111111111111"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID})
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		session, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if las.AddLinkedAccountToDBCalls != 2 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 2 (xbox + java)", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-15_MinecraftIdentityFetchFails", func(t *testing.T) {
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{xstsFails: true})
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		_, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLogin})
		if err == nil {
			t.Fatal("expected an error")
		}
		if las.AddLinkedAccountToDBCalls != 0 || ss.AddSessionCalls != 0 {
			t.Error("expected no store calls when the identity fetch fails")
		}
	})

	t.Run("OA-16_AddSessionFails", func(t *testing.T) {
		oaSetupDiscordPlatform(t)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{AddSessionFunc: func(*auth.Session) error { return errors.New("session store down") }}

		session, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLogin})
		if err == nil {
			t.Fatal("expected an error")
		}
		if session != nil {
			t.Errorf("expected nil session on error, got %+v", session)
		}
	})
}

func TestOA76to78ProcessOAuthLoginAdditionalPlatformDispatch(t *testing.T) {
	t.Run("OA-76_TwitchCreatesAccount", func(t *testing.T) {
		oaSetupTwitchPlatform(t)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		session, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformTwitch, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if as.AddAccountCalls != 1 {
			t.Errorf("AddAccount called %d times, want 1", as.AddAccountCalls)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != auth.PlatformTwitch {
			t.Errorf("expected a Twitch linked account, got %+v", las.AddedLinkedAccounts)
		}
		if ss.AddSessionCalls != 1 {
			t.Errorf("AddSession called %d times, want 1", ss.AddSessionCalls)
		}
	})

	t.Run("OA-77_MicrosoftPlainCreatesAccount", func(t *testing.T) {
		oaSetupMicrosoftLoginPlatform(t)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		session, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformMicrosoft, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if as.AddAccountCalls != 1 {
			t.Errorf("AddAccount called %d times, want 1", as.AddAccountCalls)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != auth.PlatformMicrosoft {
			t.Errorf("expected a Microsoft linked account, got %+v", las.AddedLinkedAccounts)
		}
	})

	t.Run("OA-78_XboxLiveOnlyNeverTouchesJavaEvenIfOwned", func(t *testing.T) {
		javaUUID := "88888888-8888-8888-8888-888888888888"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID})

		var mcLoginCalled, mcProfileCalled bool
		mcLoginTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcLoginCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcLoginTS.Close)
		oaSetVar(t, &minecraftLoginURL, mcLoginTS.URL)
		mcProfileTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcProfileCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcProfileTS.Close)
		oaSetVar(t, &minecraftProfileURL, mcProfileTS.URL)

		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &oaMockSessionService{}

		session, err := ProcessOAuthLogin(as, las, ss, "code", &OAuthState{Platform: auth.PlatformXboxLive, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if mcLoginCalled || mcProfileCalled {
			t.Error("expected PlatformXboxLive to never call the Minecraft Services login-with-xbox or profile endpoints, even though the account owns Java")
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1 (xbox only)", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != auth.PlatformXboxLive {
			t.Errorf("expected only the Xbox Live identity to be linked, got %+v", las.AddedLinkedAccounts)
		}
	})
}

func TestOA17to26ResolveOrCreateAccountForPlatformUser(t *testing.T) {
	user := &oaIdentity{id: "p1", username: "alice", email: "a@b.com"}

	t.Run("OA-17_NoExistingLink", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}

		got, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected a non-nil account")
		}
		if as.AddAccountCalls != 1 || las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddAccountCalls=%d AddLinkedAccountToDBCalls=%d, want 1 and 1", as.AddAccountCalls, las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-18_ExistingVerifiedEnabledLink", func(t *testing.T) {
		existing := &auth.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
			},
		}

		got, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != existing {
			t.Errorf("got %+v, want the existing account %+v", got, existing)
		}
	})

	t.Run("OA-19_ExistingLinkDisabled", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "user-1", Verified: false, LoginEnabled: true}, nil
			},
		}

		_, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if !errors.Is(err, errPlatformLoginDisabled) {
			t.Fatalf("error = %v, want errPlatformLoginDisabled", err)
		}
	})

	t.Run("OA-20_LookupErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("db down")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) { return nil, wantErr }}

		_, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-21_AddAccountFails", func(t *testing.T) {
		wantErr := errors.New("insert failed")
		as := &oaMockAccountService{AddAccountFunc: func(*auth.Account) error { return wantErr }}
		las := &oaMockLinkAccountStore{}

		_, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("AddLinkedAccountToDB should never be called when AddAccount fails")
		}
	})

	t.Run("OA-22_AddLinkFailsCleanupSucceeds", func(t *testing.T) {
		wantErr := errors.New("link insert failed")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return wantErr }}

		_, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1 (cleanup)", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-23_AddLinkFailsCleanupAlsoFails", func(t *testing.T) {
		linkErr := errors.New("link insert failed")
		cleanupErr := errors.New("delete failed")
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return linkErr }}

		_, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !errors.Is(err, linkErr) || !errors.Is(err, cleanupErr) {
			t.Errorf("error = %v, want it to wrap both %v and %v", err, linkErr, cleanupErr)
		}
	})

	t.Run("OA-24_RaceLostCleanupAndWinnerFetchSucceed", func(t *testing.T) {
		winner := &auth.Account{UserID: "winner-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(id string) (*auth.Account, error) {
			if id == "winner-1" {
				return winner, nil
			}
			return nil, auth.ErrNotFound
		}}
		var lookupCalls int
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				lookupCalls++
				if lookupCalls == 1 {
					return nil, auth.ErrNotFound
				}
				return &auth.LinkedAccount{UserID: "winner-1"}, nil
			},
		}

		got, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != winner {
			t.Errorf("got %+v, want the winner account %+v", got, winner)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1 (placeholder cleanup)", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-25_RaceLostWinnerLookupFails", func(t *testing.T) {
		wantErr := errors.New("lookup failed")
		as := &oaMockAccountService{}
		var lookupCalls int
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				lookupCalls++
				if lookupCalls == 1 {
					return nil, auth.ErrNotFound
				}
				return nil, wantErr
			},
		}

		_, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-26_RaceLostWinnerAccountFetchFails", func(t *testing.T) {
		wantErr := errors.New("account fetch failed")
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return nil, wantErr }}
		var lookupCalls int
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				lookupCalls++
				if lookupCalls == 1 {
					return nil, auth.ErrNotFound
				}
				return &auth.LinkedAccount{UserID: "winner-1"}, nil
			},
		}

		_, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}

func oaSetupDiscordUserFailure(t *testing.T) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600,"scope":"identify"}`)
	t.Cleanup(tokenTS.Close)
	usersTS := oaJSONServer(http.StatusUnauthorized, `{"message":"401: Unauthorized","code":0}`)
	t.Cleanup(usersTS.Close)
	oaSetDiscordUsersEndpoint(t, usersTS)

	originalTokenURL := discordConfig.Endpoint.TokenURL
	discordConfig.Endpoint.TokenURL = tokenTS.URL
	t.Cleanup(func() { discordConfig.Endpoint.TokenURL = originalTokenURL })
}

func TestOA27to42ProcessOAuthLink(t *testing.T) {
	activeSession := func(userID string) *auth.Session {
		return &auth.Session{ID: "sess-1", UserID: userID, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	}

	t.Run("OA-27_DiscordLinksToSession", func(t *testing.T) {
		oaSetupDiscordPlatform(t)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil || session.UserID != "user-1" {
			t.Errorf("session = %+v, want UserID=user-1", session)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-28_NoSessionInContext", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(nil)

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink})
		if err == nil || err.Error() != "session not found" {
			t.Fatalf("error = %v, want \"session not found\"", err)
		}
	})

	t.Run("OA-29_ExpiredSession", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}
		expired := &auth.Session{ID: "sess-1", UserID: "user-1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		req := oaRequestWithSession(expired)

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink})
		if err == nil || err.Error() != "session expired" {
			t.Fatalf("error = %v, want \"session expired\"", err)
		}
	})

	t.Run("OA-30_InvalidPlatform", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: "bogus", Mode: ModeLink})
		if err == nil || err.Error() != "invalid platform" {
			t.Fatalf("error = %v, want \"invalid platform\"", err)
		}
	})

	t.Run("OA-31_ExchangeFails", func(t *testing.T) {
		oaSetupDiscordExchangeFailure(t)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "bad-code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("OA-32_MinecraftBothIdentitiesNew", func(t *testing.T) {
		javaUUID := "22222222-2222-2222-2222-222222222222"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if las.AddLinkedAccountToDBCalls != 2 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 2", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-33_MinecraftJavaAbsent", func(t *testing.T) {
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{})
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1 (xbox only)", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-34_XboxAlreadyLinkedToDifferentAccountRejected", func(t *testing.T) {
		xuid := oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: "33333333-3333-3333-3333-333333333333"})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive && id == xuid {
					return &auth.LinkedAccount{UserID: "other-user", Platform: platform, PlatformID: id}, nil
				}
				return nil, auth.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformXboxLive, Mode: ModeLink})
		if !errors.Is(err, errPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("error = %v, want errPlatformAlreadyLinkedToDifferentAccount", err)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no links to be written")
		}
	})

	t.Run("OA-35_JavaAlreadyLinkedToDifferentAccountCommitsNothing", func(t *testing.T) {
		javaUUID := "44444444-4444-4444-4444-444444444444"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformMinecraft && id == javaUUID {
					return &auth.LinkedAccount{UserID: "other-user", Platform: platform, PlatformID: id}, nil
				}
				return nil, auth.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLink})
		if !errors.Is(err, errPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("error = %v, want errPlatformAlreadyLinkedToDifferentAccount", err)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected the Xbox identity to NOT be committed once the Java check fails")
		}
	})

	t.Run("OA-36_XboxPrecheckLookupErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("db down")
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive {
					return nil, wantErr
				}
				return nil, auth.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformXboxLive, Mode: ModeLink})
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no links to be written")
		}
	})

	t.Run("OA-37_JavaPrecheckLookupErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("db down")
		javaUUID := "55555555-5555-5555-5555-555555555555"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformMinecraft {
					return nil, wantErr
				}
				return nil, auth.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLink})
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no links to be written")
		}
	})

	t.Run("OA-38_BothAlreadyLinkedToSameAccountIsNoOp", func(t *testing.T) {
		xuid := "xid1"
		javaUUID := "66666666-6666-6666-6666-666666666666"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID, xuid: xuid})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if (platform == auth.PlatformXboxLive && id == xuid) || (platform == auth.PlatformMinecraft && id == javaUUID) {
					return &auth.LinkedAccount{UserID: "user-1", Platform: platform, PlatformID: id}, nil
				}
				return nil, auth.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 0 (already linked to this account)", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-39_IdentityFetchFails", func(t *testing.T) {
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{xstsFails: true})
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
		if las.GetLinkedAccountByPlatformIDCalls != 0 || las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no link-store calls when the identity fetch itself fails")
		}
	})

	t.Run("OA-40_XboxLinkWriteFailsJavaNeverAttempted", func(t *testing.T) {
		javaUUID := "77777777-7777-7777-7777-777777777777"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return errors.New("write failed") }}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMinecraft, Mode: ModeLink})
		if err == nil || err.Error() != "failed to link account" {
			t.Fatalf("error = %v, want \"failed to link account\"", err)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1 (java never attempted)", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-41_PlatformUserFetchFails", func(t *testing.T) {
		oaSetupDiscordUserFailure(t)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no link-store calls when the platform user fetch fails")
		}
	})

	t.Run("OA-42_FinalLinkWriteFails", func(t *testing.T) {
		oaSetupDiscordPlatform(t)
		wantErr := errors.New("write failed")
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return wantErr }}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestOA79to81ProcessOAuthLinkAdditionalPlatformDispatch(t *testing.T) {
	activeSession := func(userID string) *auth.Session {
		return &auth.Session{ID: "sess-1", UserID: userID, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	}

	t.Run("OA-79_TwitchLinksToSession", func(t *testing.T) {
		oaSetupTwitchPlatform(t)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformTwitch, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil || session.UserID != "user-1" {
			t.Errorf("session = %+v, want UserID=user-1", session)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != auth.PlatformTwitch {
			t.Errorf("expected a Twitch linked account, got %+v", las.AddedLinkedAccounts)
		}
	})

	t.Run("OA-80_MicrosoftPlainLinksToSession", func(t *testing.T) {
		oaSetupMicrosoftLoginPlatform(t)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformMicrosoft, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil || session.UserID != "user-1" {
			t.Errorf("session = %+v, want UserID=user-1", session)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != auth.PlatformMicrosoft {
			t.Errorf("expected a Microsoft linked account, got %+v", las.AddedLinkedAccounts)
		}
	})

	t.Run("OA-81_XboxLiveOnlyNeverTouchesJavaEvenIfOwned", func(t *testing.T) {
		javaUUID := "99999999-9999-9999-9999-999999999999"
		oaSetupMicrosoftChain(t, oaMicrosoftChainOpts{javaUUID: javaUUID})

		var mcLoginCalled, mcProfileCalled bool
		mcLoginTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcLoginCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcLoginTS.Close)
		oaSetVar(t, &minecraftLoginURL, mcLoginTS.URL)
		mcProfileTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcProfileCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcProfileTS.Close)
		oaSetVar(t, &minecraftProfileURL, mcProfileTS.URL)

		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, las, "code", &OAuthState{Platform: auth.PlatformXboxLive, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if mcLoginCalled || mcProfileCalled {
			t.Error("expected PlatformXboxLive to never call the Minecraft Services login-with-xbox or profile endpoints, even though the account owns Java")
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1 (xbox only)", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != auth.PlatformXboxLive {
			t.Errorf("expected only the Xbox Live identity to be linked, got %+v", las.AddedLinkedAccounts)
		}
	})
}

func TestOA43to55ResolveOrCreateAccountForMicrosoftUser(t *testing.T) {
	xbox := &XboxLiveData{XUID: "xid1", Gamertag: "Tag"}
	java := &MinecraftData{ID: mustUUID("11111111-1111-1111-1111-111111111111"), Username: "Steve"}

	notFoundStore := func() *oaMockLinkAccountStore {
		return &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) { return nil, auth.ErrNotFound }}
	}

	t.Run("OA-43_NeitherLinkedJavaPresentPrefersJavaUsername", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := notFoundStore()

		got, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.Username != "Steve" {
			t.Errorf("account = %+v, want Username=Steve", got)
		}
		if las.AddLinkedAccountToDBCalls != 2 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 2", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-44_NeitherLinkedJavaAbsentUsesGamertag", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := notFoundStore()

		got, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.Username != "Tag" {
			t.Errorf("account = %+v, want Username=Tag", got)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-45_XboxAlreadyLinkedJavaAbsent", func(t *testing.T) {
		existing := &auth.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive {
					return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return nil, auth.ErrNotFound
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != existing {
			t.Errorf("account = %+v, want the existing account", got)
		}
		if as.AddAccountCalls != 0 || las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no new account or link writes")
		}
	})

	t.Run("OA-46_JavaAlreadyLinkedXboxNot", func(t *testing.T) {
		existing := &auth.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformMinecraft {
					return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return nil, auth.ErrNotFound
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != existing {
			t.Errorf("account = %+v, want the existing account", got)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1 (xbox linked to the resolved account)", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-47_BothAlreadyLinkedSameAccountIsNoOp", func(t *testing.T) {
		existing := &auth.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != existing {
			t.Errorf("account = %+v, want the existing account", got)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 0", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-48_ConflictingAccountsRejected", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive {
					return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return &auth.LinkedAccount{UserID: "user-2", Verified: true, LoginEnabled: true}, nil
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
		if !errors.Is(err, errConflictingMicrosoftIdentities) {
			t.Fatalf("error = %v, want errConflictingMicrosoftIdentities", err)
		}
	})

	t.Run("OA-49_NoEligibleIdentityRejected", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive {
					return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: false}, nil
				}
				return nil, auth.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, nil)
		if !errors.Is(err, errPlatformLoginDisabled) {
			t.Fatalf("error = %v, want errPlatformLoginDisabled", err)
		}
	})

	t.Run("OA-50_OneIneligibleOtherEligibleSucceeds", func(t *testing.T) {
		existing := &auth.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive {
					return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: false}, nil
				}
				return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != existing {
			t.Errorf("account = %+v, want the existing account", got)
		}
	})

	t.Run("OA-51_XboxLookupErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("db down")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive {
					return nil, wantErr
				}
				return nil, auth.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-52_JavaLookupErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("db down")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformMinecraft {
					return nil, wantErr
				}
				return nil, auth.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-53_AddAccountFails", func(t *testing.T) {
		wantErr := errors.New("insert failed")
		as := &oaMockAccountService{AddAccountFunc: func(*auth.Account) error { return wantErr }}
		las := notFoundStore()

		_, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-54_GetAccountByIDFails", func(t *testing.T) {
		wantErr := errors.New("fetch failed")
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return nil, wantErr }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform auth.Platform, id string) (*auth.LinkedAccount, error) {
				if platform == auth.PlatformXboxLive {
					return &auth.LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return nil, auth.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-55_EnsureIdentityLinkedFails", func(t *testing.T) {
		wantErr := errors.New("link failed")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) { return nil, auth.ErrNotFound },
			AddLinkedAccountToDBFunc:         func(*auth.LinkedAccount) error { return wantErr },
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}

func TestOA56to66EnsureMicrosoftIdentityLinked(t *testing.T) {
	xbox := &XboxLiveData{XUID: "xid1", Gamertag: "Tag"}
	account := &auth.Account{UserID: "user-1"}

	t.Run("OA-56_Success", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}

		gotAccount, gotIsNew, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAccount != account || gotIsNew != false {
			t.Errorf("got (%+v, %v), want (%+v, false)", gotAccount, gotIsNew, account)
		}
	})

	t.Run("OA-57_LinkFailsNewAccountCleanupSucceeds", func(t *testing.T) {
		wantErr := errors.New("link failed")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return wantErr }}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-58_LinkFailsExistingAccountNoCleanup", func(t *testing.T) {
		wantErr := errors.New("link failed")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return wantErr }}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, false, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 0 {
			t.Errorf("DeleteAccount called %d times, want 0 (not a new account)", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-59_LinkFailsCleanupAlsoFails", func(t *testing.T) {
		linkErr := errors.New("link failed")
		cleanupErr := errors.New("cleanup failed")
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return linkErr }}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, linkErr) || !errors.Is(err, cleanupErr) {
			t.Errorf("error = %v, want it to wrap both %v and %v", err, linkErr, cleanupErr)
		}
	})

	t.Run("OA-60_AlreadyLinkedButWeAreTheOwner", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "user-1"}, nil
			},
		}

		gotAccount, gotIsNew, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAccount != account || gotIsNew != false {
			t.Errorf("got (%+v, %v), want (%+v, false)", gotAccount, gotIsNew, account)
		}
		if as.DeleteAccountCalls != 0 {
			t.Error("expected no cleanup: we already own the identity")
		}
	})

	t.Run("OA-61_AlreadyLinkedRefetchFailsNewAccountCleanupSucceeds", func(t *testing.T) {
		wantErr := errors.New("lookup failed")
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc:         func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) { return nil, wantErr },
		}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-62_AlreadyLinkedRefetchFailsCleanupAlsoFails", func(t *testing.T) {
		lookupErr := errors.New("lookup failed")
		cleanupErr := errors.New("cleanup failed")
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc:         func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) { return nil, lookupErr },
		}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, lookupErr) || !errors.Is(err, cleanupErr) {
			t.Errorf("error = %v, want it to wrap both %v and %v", err, lookupErr, cleanupErr)
		}
	})

	t.Run("OA-63_RealConflictExistingAccountRejected", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "other-user"}, nil
			},
		}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, false, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, errConflictingMicrosoftIdentities) {
			t.Fatalf("error = %v, want errConflictingMicrosoftIdentities", err)
		}
	})

	t.Run("OA-64_RealConflictNewAccountUsesWinner", func(t *testing.T) {
		winner := &auth.Account{UserID: "other-user"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(id string) (*auth.Account, error) {
			if id == "other-user" {
				return winner, nil
			}
			return nil, auth.ErrNotFound
		}}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "other-user"}, nil
			},
		}

		gotAccount, gotIsNew, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAccount != winner || gotIsNew != false {
			t.Errorf("got (%+v, %v), want (%+v, false)", gotAccount, gotIsNew, winner)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-65_RealConflictNewAccountCleanupFails", func(t *testing.T) {
		cleanupErr := errors.New("cleanup failed")
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "other-user"}, nil
			},
		}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("error = %v, want it to wrap %v", err, cleanupErr)
		}
	})

	t.Run("OA-66_RealConflictNewAccountWinnerFetchFails", func(t *testing.T) {
		wantErr := errors.New("winner fetch failed")
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*auth.Account, error) { return nil, wantErr }}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return auth.ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "other-user"}, nil
			},
		}

		_, _, err := ensureMicrosoftIdentityLinked(as, las, account, true, auth.PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}

func TestOA67to71LinkPlatformUserToSession(t *testing.T) {
	user := &oaIdentity{id: "p1", username: "alice"}

	t.Run("OA-67_NotYetLinked", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}

		err := linkPlatformUserToSession(las, "user-1", auth.PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-68_AlreadyLinkedToSameAccountNoOp", func(t *testing.T) {
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "user-1"}, nil
			},
		}

		err := linkPlatformUserToSession(las, "user-1", auth.PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 0", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-69_AlreadyLinkedToDifferentAccountRejected", func(t *testing.T) {
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
				return &auth.LinkedAccount{UserID: "other-user"}, nil
			},
		}

		err := linkPlatformUserToSession(las, "user-1", auth.PlatformDiscord, user)
		if !errors.Is(err, errPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("error = %v, want errPlatformAlreadyLinkedToDifferentAccount", err)
		}
	})

	t.Run("OA-70_LookupErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("db down")
		las := &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) { return nil, wantErr }}

		err := linkPlatformUserToSession(las, "user-1", auth.PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-71_AddLinkedAccountFails", func(t *testing.T) {
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*auth.LinkedAccount) error { return errors.New("insert failed") }}

		err := linkPlatformUserToSession(las, "user-1", auth.PlatformDiscord, user)
		if err == nil || err.Error() != "failed to link account" {
			t.Fatalf("error = %v, want \"failed to link account\"", err)
		}
	})
}

// The functions under test hold no locks themselves; the race is between
// separate callers, arbitrated by the store's (platform, platform_id)
// uniqueness constraint. The doubles below enforce that constraint
// atomically under a mutex, so launching real goroutines against them
// exercises the actual race instead of just asserting behavior for a canned
// auth.ErrAlreadyLinked return.

type oaConcurrentAccountService struct {
	mu       sync.Mutex
	accounts map[string]*auth.Account
}

var _ auth.AccountService = (*oaConcurrentAccountService)(nil)

func newOAConcurrentAccountService() *oaConcurrentAccountService {
	return &oaConcurrentAccountService{accounts: map[string]*auth.Account{}}
}

func (m *oaConcurrentAccountService) AddAccount(a *auth.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.UserID] = a
	return nil
}
func (m *oaConcurrentAccountService) GetAccountByID(userID string) (*auth.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[userID]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return a, nil
}
func (m *oaConcurrentAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *oaConcurrentAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *oaConcurrentAccountService) UpdateAccount(a *auth.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.UserID] = a
	return nil
}
func (m *oaConcurrentAccountService) DeleteAccount(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.accounts, userID)
	return nil
}
func (m *oaConcurrentAccountService) IsPasswordAuthEnabled(string) (bool, error) { return false, nil }

func (m *oaConcurrentAccountService) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.accounts)
}

func oaLinkKey(platform auth.Platform, platformID string) string {
	return string(platform) + "|" + platformID
}

type oaConcurrentLinkAccountStore struct {
	mu    sync.Mutex
	byKey map[string]*auth.LinkedAccount
}

var _ auth.LinkAccountStore = (*oaConcurrentLinkAccountStore)(nil)

func newOAConcurrentLinkAccountStore() *oaConcurrentLinkAccountStore {
	return &oaConcurrentLinkAccountStore{byKey: map[string]*auth.LinkedAccount{}}
}

func (m *oaConcurrentLinkAccountStore) AddLinkedAccountToDB(la *auth.LinkedAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := oaLinkKey(la.Platform, la.PlatformID)
	if _, exists := m.byKey[key]; exists {
		return auth.ErrAlreadyLinked
	}
	m.byKey[key] = la
	return nil
}
func (m *oaConcurrentLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error { return nil }
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountByPlatformID(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	la, ok := m.byKey[oaLinkKey(platform, platformID)]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return la, nil
}
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*auth.LinkedAccount, error) {
	return nil, nil
}
func (m *oaConcurrentLinkAccountStore) DeleteLinkedAccount(string, auth.Platform) error { return nil }
func (m *oaConcurrentLinkAccountStore) SetLinkedAccountLoginEnabled(string, auth.Platform, bool) error {
	return nil
}

func TestOA72ResolveOrCreateAccountForPlatformUserConcurrentRace(t *testing.T) {
	t.Run("OA-72_ConcurrentCallersConvergeOnOneAccount", func(t *testing.T) {
		as := newOAConcurrentAccountService()
		las := newOAConcurrentLinkAccountStore()
		user := &oaIdentity{id: "race-1", username: "alice"}

		const n = 2
		var wg sync.WaitGroup
		results := make([]*auth.Account, n)
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				defer wg.Done()
				results[i], errs[i] = resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformDiscord, user)
			}()
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Fatalf("goroutine %d: unexpected error: %v", i, err)
			}
		}
		if results[0] == nil || results[1] == nil {
			t.Fatal("expected both goroutines to resolve a non-nil account")
		}
		if results[0].UserID != results[1].UserID {
			t.Errorf("goroutines resolved to different accounts: %s vs %s, want exactly one winner", results[0].UserID, results[1].UserID)
		}
		if got := as.count(); got != 1 {
			t.Errorf("account store has %d accounts after the race, want exactly 1 (no orphaned placeholder)", got)
		}
	})
}

func TestOA73And74EnsureMicrosoftIdentityLinkedConcurrentRace(t *testing.T) {
	t.Run("OA-73_NeitherLinkedYetConvergesAndCleansUpLoser", func(t *testing.T) {
		as := newOAConcurrentAccountService()
		las := newOAConcurrentLinkAccountStore()
		xbox := &XboxLiveData{XUID: "race-xuid-73", Gamertag: "Tag"}

		placeholderA := &auth.Account{UserID: "placeholder-a"}
		placeholderB := &auth.Account{UserID: "placeholder-b"}
		_ = as.AddAccount(placeholderA)
		_ = as.AddAccount(placeholderB)

		var wg sync.WaitGroup
		var resultA, resultB *auth.Account
		var errA, errB error
		wg.Add(2)
		go func() {
			defer wg.Done()
			resultA, _, errA = ensureMicrosoftIdentityLinked(as, las, placeholderA, true, auth.PlatformXboxLive, xbox)
		}()
		go func() {
			defer wg.Done()
			resultB, _, errB = ensureMicrosoftIdentityLinked(as, las, placeholderB, true, auth.PlatformXboxLive, xbox)
		}()
		wg.Wait()

		if errA != nil {
			t.Fatalf("goroutine A: unexpected error: %v", errA)
		}
		if errB != nil {
			t.Fatalf("goroutine B: unexpected error: %v", errB)
		}
		if resultA.UserID != resultB.UserID {
			t.Errorf("goroutines converged on different accounts: %s vs %s", resultA.UserID, resultB.UserID)
		}
		if got := as.count(); got != 1 {
			t.Errorf("account store has %d accounts after the race, want exactly 1 (loser's placeholder cleaned up)", got)
		}
	})

	t.Run("OA-74_PartialStateRaceNeverDeletesAnExistingAccount", func(t *testing.T) {
		as := newOAConcurrentAccountService()
		las := newOAConcurrentLinkAccountStore()
		xbox := &XboxLiveData{XUID: "race-xuid-74", Gamertag: "Tag2"}

		existingAccount := &auth.Account{UserID: "existing-account"}
		freshPlaceholder := &auth.Account{UserID: "fresh-placeholder"}
		_ = as.AddAccount(existingAccount)
		_ = as.AddAccount(freshPlaceholder)

		var wg sync.WaitGroup
		var resultA, resultB *auth.Account
		var errA, errB error
		wg.Add(2)
		go func() {
			defer wg.Done()
			resultA, _, errA = ensureMicrosoftIdentityLinked(as, las, existingAccount, false, auth.PlatformXboxLive, xbox)
		}()
		go func() {
			defer wg.Done()
			resultB, _, errB = ensureMicrosoftIdentityLinked(as, las, freshPlaceholder, true, auth.PlatformXboxLive, xbox)
		}()
		wg.Wait()

		if _, err := as.GetAccountByID(existingAccount.UserID); err != nil {
			t.Fatalf("existing-account was deleted (or is missing) after the race: %v", err)
		}

		switch {
		case errA == nil && errB == nil:
			if resultA.UserID != existingAccount.UserID {
				t.Errorf("A resolved to %s, want its own account %s", resultA.UserID, existingAccount.UserID)
			}
			if resultB.UserID != existingAccount.UserID {
				t.Errorf("B converged on %s, want the existing account %s", resultB.UserID, existingAccount.UserID)
			}
			if _, err := as.GetAccountByID(freshPlaceholder.UserID); !errors.Is(err, auth.ErrNotFound) {
				t.Errorf("expected B's placeholder to have been deleted after losing, got err=%v", err)
			}
			if got := as.count(); got != 1 {
				t.Errorf("account store has %d accounts, want exactly 1", got)
			}
		case errA != nil && errB == nil:
			if !errors.Is(errA, errConflictingMicrosoftIdentities) {
				t.Errorf("A's error = %v, want errConflictingMicrosoftIdentities", errA)
			}
			if resultB.UserID != freshPlaceholder.UserID {
				t.Errorf("B resolved to %s, want its own account %s", resultB.UserID, freshPlaceholder.UserID)
			}
			if _, err := as.GetAccountByID(freshPlaceholder.UserID); err != nil {
				t.Errorf("B's account should still exist: %v", err)
			}
			if got := as.count(); got != 2 {
				t.Errorf("account store has %d accounts, want exactly 2 (both survive: A was never deleted, B won)", got)
			}
		default:
			t.Fatalf("unexpected outcome: errA=%v errB=%v", errA, errB)
		}
	})
}

func TestOA75ResolveOrCreateAccountForMicrosoftUserConcurrentRace(t *testing.T) {
	t.Run("OA-75_ConcurrentFreshLoginsConvergeOnOneAccount", func(t *testing.T) {
		as := newOAConcurrentAccountService()
		las := newOAConcurrentLinkAccountStore()
		xbox := &XboxLiveData{XUID: "race-full-xuid", Gamertag: "Tag3"}
		java := &MinecraftData{ID: mustUUID("99999999-9999-9999-9999-999999999999"), Username: "Steve3"}

		const n = 2
		var wg sync.WaitGroup
		results := make([]*auth.Account, n)
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				defer wg.Done()
				results[i], errs[i] = resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
			}()
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Fatalf("goroutine %d: unexpected error: %v", i, err)
			}
		}
		if results[0].UserID != results[1].UserID {
			t.Errorf("goroutines converged on different accounts: %s vs %s", results[0].UserID, results[1].UserID)
		}
		if got := as.count(); got != 1 {
			t.Errorf("account store has %d accounts after the race, want exactly 1", got)
		}

		xboxLink, err := las.GetLinkedAccountByPlatformID(auth.PlatformXboxLive, xbox.XUID)
		if err != nil || xboxLink.UserID != results[0].UserID {
			t.Errorf("xbox link = %+v (err=%v), want it linked to %s", xboxLink, err, results[0].UserID)
		}
		javaLink, err := las.GetLinkedAccountByPlatformID(auth.PlatformMinecraft, java.ID.String())
		if err != nil || javaLink.UserID != results[0].UserID {
			t.Errorf("java link = %+v (err=%v), want it linked to %s", javaLink, err, results[0].UserID)
		}
	})
}

// oaBarrierLinkAccountStore extends oaConcurrentLinkAccountStore: its
// GetLinkedAccountByPlatformID also holds every caller at a barrier until
// each has read, forcing linkPlatformUserToSession's check-then-act window to
// overlap deterministically - without it a fast in-memory store could let one
// goroutine finish before another starts, only occasionally racing.
type oaBarrierLinkAccountStore struct {
	mu    sync.Mutex
	byKey map[string]*auth.LinkedAccount

	barrierMu    sync.Mutex
	barrierWG    sync.WaitGroup
	barrierN     int
	barrierCount int
}

var _ auth.LinkAccountStore = (*oaBarrierLinkAccountStore)(nil)

func newOABarrierLinkAccountStore(barrierN int) *oaBarrierLinkAccountStore {
	s := &oaBarrierLinkAccountStore{byKey: map[string]*auth.LinkedAccount{}, barrierN: barrierN}
	s.barrierWG.Add(barrierN)
	return s
}

func (m *oaBarrierLinkAccountStore) GetLinkedAccountByPlatformID(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
	m.barrierMu.Lock()
	participates := m.barrierCount < m.barrierN
	if participates {
		m.barrierCount++
	}
	m.barrierMu.Unlock()
	if participates {
		m.barrierWG.Done()
		m.barrierWG.Wait()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	la, ok := m.byKey[oaLinkKey(platform, platformID)]
	if !ok {
		return nil, auth.ErrNotFound
	}
	cp := *la
	return &cp, nil
}
func (m *oaBarrierLinkAccountStore) AddLinkedAccountToDB(la *auth.LinkedAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := oaLinkKey(la.Platform, la.PlatformID)
	if _, exists := m.byKey[key]; exists {
		return auth.ErrAlreadyLinked
	}
	cp := *la
	m.byKey[key] = &cp
	return nil
}
func (m *oaBarrierLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error { return nil }
func (m *oaBarrierLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *oaBarrierLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *oaBarrierLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*auth.LinkedAccount, error) {
	return nil, nil
}
func (m *oaBarrierLinkAccountStore) DeleteLinkedAccount(string, auth.Platform) error { return nil }
func (m *oaBarrierLinkAccountStore) SetLinkedAccountLoginEnabled(string, auth.Platform, bool) error {
	return nil
}

func TestOA82LinkPlatformUserToSessionConcurrentRace(t *testing.T) {
	t.Run("OA-82_ConcurrentCallersToDifferentSessionsExactlyOneWins", func(t *testing.T) {
		las := newOABarrierLinkAccountStore(2)
		user := &oaIdentity{id: "race-82", username: "alice"}

		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs[0] = linkPlatformUserToSession(las, "user-A", auth.PlatformDiscord, user)
		}()
		go func() {
			defer wg.Done()
			errs[1] = linkPlatformUserToSession(las, "user-B", auth.PlatformDiscord, user)
		}()
		wg.Wait()

		successes := 0
		for _, err := range errs {
			if err == nil {
				successes++
			}
		}
		if successes != 1 {
			t.Fatalf("errs = %v, want exactly one nil (one winner) and one non-nil (the observable loser)", errs)
		}

		la, err := las.GetLinkedAccountByPlatformID(auth.PlatformDiscord, "race-82")
		if err != nil {
			t.Fatalf("expected the linked account to exist after the race, got err=%v", err)
		}
		if la.UserID != "user-A" && la.UserID != "user-B" {
			t.Errorf("linked account UserID = %q, want user-A or user-B (the winner), not duplicated or overwritten by a third value", la.UserID)
		}
	})
}
