package link

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil/authtest"
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
func (f *oaIdentity) CreateLinkedAccount(userID string) *LinkedAccount {
	return NewLinkedAccount(userID, PlatformDiscord, f.username, f.id, f)
}

type oaCtxMarkerKey struct{}

type oaCtxRecord struct {
	Errs    []error
	Markers []any
}

func (r *oaCtxRecord) record(ctx context.Context) {
	r.Errs = append(r.Errs, ctx.Err())
	r.Markers = append(r.Markers, ctx.Value(oaCtxMarkerKey{}))
}

type oaMockAccountService struct {
	AddAccountFunc     func(*account.Account) error
	GetAccountByIDFunc func(string) (*account.Account, error)
	DeleteAccountFunc  func(string) error

	AddAccountCalls     int
	GetAccountByIDCalls int
	DeleteAccountCalls  int
	DeleteCtx           oaCtxRecord
}

var _ account.Service = (*oaMockAccountService)(nil)

func (m *oaMockAccountService) AddAccount(_ context.Context, a *account.Account) error {
	m.AddAccountCalls++
	if m.AddAccountFunc != nil {
		return m.AddAccountFunc(a)
	}
	return nil
}
func (m *oaMockAccountService) GetAccountByID(_ context.Context, userID string) (*account.Account, error) {
	m.GetAccountByIDCalls++
	if m.GetAccountByIDFunc != nil {
		return m.GetAccountByIDFunc(userID)
	}
	return nil, account.ErrNotFound
}
func (m *oaMockAccountService) GetAccountByUsername(context.Context, string) (*account.Account, error) {
	return nil, account.ErrNotFound
}
func (m *oaMockAccountService) GetAccountByEmail(context.Context, string) (*account.Account, error) {
	return nil, account.ErrNotFound
}
func (m *oaMockAccountService) UpdateAccount(context.Context, *account.Account) error { return nil }
func (m *oaMockAccountService) DeleteAccount(ctx context.Context, userID string) error {
	m.DeleteAccountCalls++
	m.DeleteCtx.record(ctx)
	if m.DeleteAccountFunc != nil {
		return m.DeleteAccountFunc(userID)
	}
	return nil
}
func (m *oaMockAccountService) IsPasswordAuthEnabled(context.Context, string) (bool, error) {
	return false, nil
}
func (m *oaMockAccountService) NewSession(_ context.Context, a *account.Account, expiresAt int64) (*session.Session, error) {
	return &session.Session{ID: "stub-session", UserID: a.UserID, Permissions: []string{"stub:perm"}, ExpiresAt: expiresAt}, nil
}

type oaMockLinkAccountStore struct {
	GetLinkedAccountByPlatformIDFunc func(Platform, string) (*LinkedAccount, error)
	AddLinkedAccountToDBFunc         func(*LinkedAccount) error

	GetLinkedAccountByPlatformIDCalls int
	AddLinkedAccountToDBCalls         int
	AddedLinkedAccounts               []*LinkedAccount
	AddCtx                            oaCtxRecord
}

var _ Store = (*oaMockLinkAccountStore)(nil)

func (m *oaMockLinkAccountStore) AddLinkedAccountToDB(ctx context.Context, la *LinkedAccount) error {
	m.AddLinkedAccountToDBCalls++
	m.AddCtx.record(ctx)
	if m.AddLinkedAccountToDBFunc != nil {
		if err := m.AddLinkedAccountToDBFunc(la); err != nil {
			return err
		}
	}
	m.AddedLinkedAccounts = append(m.AddedLinkedAccounts, la)
	return nil
}
func (m *oaMockLinkAccountStore) UpdateLinkedAccount(context.Context, *LinkedAccount) error {
	return nil
}
func (m *oaMockLinkAccountStore) GetLinkedAccountByPlatformID(_ context.Context, platform Platform, platformID string) (*LinkedAccount, error) {
	m.GetLinkedAccountByPlatformIDCalls++
	if m.GetLinkedAccountByPlatformIDFunc != nil {
		return m.GetLinkedAccountByPlatformIDFunc(platform, platformID)
	}
	return nil, account.ErrNotFound
}
func (m *oaMockLinkAccountStore) GetLinkedAccountByPlatformName(context.Context, Platform, string) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}
func (m *oaMockLinkAccountStore) GetLinkedAccountByUserID(context.Context, string, Platform) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}
func (m *oaMockLinkAccountStore) GetLinkedAccountsByUserID(context.Context, string) ([]*LinkedAccount, error) {
	return nil, nil
}
func (m *oaMockLinkAccountStore) DeleteLinkedAccount(context.Context, string, Platform) error {
	return nil
}
func (m *oaMockLinkAccountStore) SetLinkedAccountLoginEnabled(context.Context, string, Platform, bool) error {
	return nil
}

func oaRequestWithSession(session *session.Session) *http.Request {
	req := testutil.NewRequest(http.MethodGet, "/", nil)
	if session == nil {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), reqctx.SessionKey, session))
}

func oaJSONServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func oaSetupDiscordPlatform(t *testing.T, cfg *config.Config) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600,"scope":"identify"}`)
	t.Cleanup(tokenTS.Close)
	usersTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"d1","username":"alice","email":"a@b.com"}`))
	}))
	t.Cleanup(usersTS.Close)
	setDiscordUsersEndpoint(t, usersTS)

	cfg.Discord.OAuth2Config = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenTS.URL}}
}

func oaSetupDiscordExchangeFailure(t *testing.T, cfg *config.Config) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusBadRequest, `{"error":"invalid_grant"}`)
	t.Cleanup(tokenTS.Close)
	cfg.Discord.OAuth2Config = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenTS.URL}}
}

func oaSetupTwitchPlatform(t *testing.T, cfg *config.Config) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"twitch-at","token_type":"bearer","expires_in":14400,"scope":["user:read:email"]}`)
	t.Cleanup(tokenTS.Close)
	cfg.Twitch.OAuth2Config = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenTS.URL}}

	usersTS := oaJSONServer(http.StatusOK, `{"data":[{"id":"tw1","login":"alice","email":"a@b.com"}]}`)
	t.Cleanup(usersTS.Close)
	cfg.Twitch.APIBaseUrl = usersTS.URL
	// helix.NewClient requires a ClientID to construct at all.
	cfg.Twitch.ClientId = "test-client-id"
}

func oaSetupMicrosoftLoginPlatform(t *testing.T, cfg *config.Config) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"ms-login-at","token_type":"Bearer","expires_in":3600,"scope":"openid profile email"}`)
	t.Cleanup(tokenTS.Close)
	cfg.Microsoft.LoginOAuth2Config = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenTS.URL}}

	userInfoTS := oaJSONServer(http.StatusOK, `{"sub":"ms1","name":"Alice","email":"a@b.com"}`)
	t.Cleanup(userInfoTS.Close)
	cfg.Microsoft.MicrosoftUserInfoURL = userInfoTS.URL
}

type oaMicrosoftChainOpts struct {
	javaUUID          string
	xuid, gamertag    string
	xstsFails         bool
	mcLoginFails      bool
	mcProfileFails    bool
	xstsXboxLiveFails bool
}

func oaSetupMicrosoftChain(t *testing.T, cfg *config.Config, opts oaMicrosoftChainOpts) string {
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
	cfg.Microsoft.OAuth2Config = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenTS.URL}}

	xblTS := oaJSONServer(http.StatusOK, `{"Token":"xbl-t"}`)
	t.Cleanup(xblTS.Close)
	cfg.Microsoft.XboxLiveAuthenticateURL = xblTS.URL

	if opts.xstsFails {
		xstsTS := oaJSONServer(http.StatusInternalServerError, `{}`)
		t.Cleanup(xstsTS.Close)
		cfg.Microsoft.XstsAuthorizeURL = xstsTS.URL
		return xuid
	}

	xstsTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RelyingParty string `json:"RelyingParty"`
		}
		_ = readJSONBody(r, &body)
		w.Header().Set("Content-Type", "application/json")
		if body.RelyingParty == XstsXboxLiveRelyingParty {
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
	cfg.Microsoft.XstsAuthorizeURL = xstsTS.URL

	mcLoginTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if opts.mcLoginFails {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"mc-t"}`))
	}))
	t.Cleanup(mcLoginTS.Close)
	cfg.Microsoft.MinecraftLoginURL = mcLoginTS.URL

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
	cfg.Microsoft.MinecraftProfileURL = mcProfileTS.URL

	return xuid
}

func TestOA01to04ExtCodeForToken(t *testing.T) {
	t.Run("OA-01_ArrayScope", func(t *testing.T) {
		ts := oaJSONServer(http.StatusOK, `{"access_token":"AT","token_type":"Bearer","refresh_token":"RT","expires_in":3600,"scope":["identify","email"]}`)
		defer ts.Close()
		config := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: ts.URL, AuthStyle: oauth2.AuthStyleInHeader}}

		got, err := ExtCodeForToken(context.Background(), config, "code")
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

		got, err := ExtCodeForToken(context.Background(), config, "code")
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

		got, err := ExtCodeForToken(context.Background(), config, "bad-code")
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

		_, err := ExtCodeForToken(context.Background(), config, "code")
		if !errors.Is(err, ErrNoScopeInToken) {
			t.Fatalf("ExtCodeForToken() error = %v, want %v", err, ErrNoScopeInToken)
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

		got, err := RefreshToken(context.Background(), config, expiredToken())
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

		got, err := RefreshToken(context.Background(), config, expiredToken())
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

		got, err := RefreshToken(context.Background(), config, expiredToken())
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

		_, err := RefreshToken(context.Background(), config, expiredToken())
		if !errors.Is(err, ErrNoScopeInToken) {
			t.Fatalf("RefreshToken() error = %v, want %v", err, ErrNoScopeInToken)
		}
	})
}

func TestOA09to16ProcessOAuthLogin(t *testing.T) {
	t.Run("OA-09_DiscordCreatesAccount", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordPlatform(t, cfg)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		session, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if len(session.Permissions) != 1 || session.Permissions[0] != "stub:perm" {
			t.Errorf("Permissions = %v, want those the account service built", session.Permissions)
		}
		if as.AddAccountCalls != 1 {
			t.Errorf("AddAccount called %d times, want 1", as.AddAccountCalls)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(ss.Added) != 1 {
			t.Errorf("AddSession called %d times, want 1", len(ss.Added))
		}
	})

	t.Run("OA-10_DiscordReusesExistingLinkedAccount", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordPlatform(t, cfg)
		existing := &account.Account{UserID: "user-1"}
		as := &oaMockAccountService{
			GetAccountByIDFunc: func(id string) (*account.Account, error) {
				if id == "user-1" {
					return existing, nil
				}
				return nil, account.ErrNotFound
			},
		}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "user-1", Platform: PlatformDiscord, PlatformID: "d1", Verified: true, LoginEnabled: true}, nil
			},
		}
		ss := &authtest.FakeSessionService{}

		session, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLogin})
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
		ss := &authtest.FakeSessionService{}

		cfg := testutil.NewConfig()
		_, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: "bogus", Mode: ModeLogin})
		if !errors.Is(err, ErrInvalidPlatform) {
			t.Fatalf("ProcessOAuthLogin(context.Background(), ) error = %v, want %v", err, ErrInvalidPlatform)
		}
		if as.AddAccountCalls != 0 || las.AddLinkedAccountToDBCalls != 0 || len(ss.Added) != 0 {
			t.Error("expected no store calls for an invalid platform")
		}
	})

	t.Run("OA-12_ExchangeFails", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordExchangeFailure(t, cfg)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		_, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "bad-code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLogin})
		if err == nil {
			t.Fatal("expected an error")
		}
		if as.AddAccountCalls != 0 || las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no store calls when the code exchange fails")
		}
	})

	t.Run("OA-13_LinkedAccountLoginDisabled", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordPlatform(t, cfg)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "user-1", Platform: PlatformDiscord, PlatformID: "d1", Verified: true, LoginEnabled: false}, nil
			},
		}
		ss := &authtest.FakeSessionService{}

		_, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLogin})
		if !errors.Is(err, ErrPlatformLoginDisabled) {
			t.Fatalf("ProcessOAuthLogin(context.Background(), ) error = %v, want ErrPlatformLoginDisabled", err)
		}
	})

	t.Run("OA-14_MinecraftFullChainNewAccount", func(t *testing.T) {
		javaUUID := "11111111-1111-1111-1111-111111111111"
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID})
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		session, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLogin})
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
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{xstsFails: true})
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		_, err := ProcessOAuthLogin(testutil.LogContext(context.Background()), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLogin})
		if err == nil {
			t.Fatal("expected an error")
		}
		if las.AddLinkedAccountToDBCalls != 0 || len(ss.Added) != 0 {
			t.Error("expected no store calls when the identity fetch fails")
		}
	})

	t.Run("OA-16_AddSessionFails", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordPlatform(t, cfg)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{AddSessionErr: testutil.ErrDBDown}

		session, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLogin})
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
		cfg := testutil.NewConfig()
		oaSetupTwitchPlatform(t, cfg)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		session, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformTwitch, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if len(session.Permissions) != 1 || session.Permissions[0] != "stub:perm" {
			t.Errorf("Permissions = %v, want those the account service built", session.Permissions)
		}
		if as.AddAccountCalls != 1 {
			t.Errorf("AddAccount called %d times, want 1", as.AddAccountCalls)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != PlatformTwitch {
			t.Errorf("expected a Twitch linked account, got %+v", las.AddedLinkedAccounts)
		}
		if len(ss.Added) != 1 {
			t.Errorf("AddSession called %d times, want 1", len(ss.Added))
		}
	})

	t.Run("OA-77_MicrosoftPlainCreatesAccount", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupMicrosoftLoginPlatform(t, cfg)
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		session, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformMicrosoft, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if len(session.Permissions) != 1 || session.Permissions[0] != "stub:perm" {
			t.Errorf("Permissions = %v, want those the account service built", session.Permissions)
		}
		if as.AddAccountCalls != 1 {
			t.Errorf("AddAccount called %d times, want 1", as.AddAccountCalls)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != PlatformMicrosoft {
			t.Errorf("expected a Microsoft linked account, got %+v", las.AddedLinkedAccounts)
		}
	})

	t.Run("OA-78_XboxLiveOnlyNeverTouchesJavaEvenIfOwned", func(t *testing.T) {
		javaUUID := "88888888-8888-8888-8888-888888888888"
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID})

		var mcLoginCalled, mcProfileCalled bool
		mcLoginTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcLoginCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcLoginTS.Close)
		cfg.Microsoft.MinecraftLoginURL = mcLoginTS.URL
		mcProfileTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcProfileCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcProfileTS.Close)
		cfg.Microsoft.MinecraftProfileURL = mcProfileTS.URL

		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		session, err := ProcessOAuthLogin(context.Background(), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformXboxLive, Mode: ModeLogin})
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
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != PlatformXboxLive {
			t.Errorf("expected only the Xbox Live identity to be linked, got %+v", las.AddedLinkedAccounts)
		}
	})
}

func TestOA17to26ResolveOrCreateAccountForPlatformUser(t *testing.T) {
	user := &oaIdentity{id: "p1", username: "alice", email: "a@b.com"}

	t.Run("OA-17_NoExistingLink", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}

		got, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
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
		existing := &account.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
			},
		}

		got, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
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
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "user-1", Verified: false, LoginEnabled: true}, nil
			},
		}

		_, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
		if !errors.Is(err, ErrPlatformLoginDisabled) {
			t.Fatalf("error = %v, want ErrPlatformLoginDisabled", err)
		}
	})

	t.Run("OA-20_LookupErrorPropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, wantErr }}

		_, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-21_AddAccountFails", func(t *testing.T) {
		wantErr := testutil.ErrInsertFailed
		as := &oaMockAccountService{AddAccountFunc: func(*account.Account) error { return wantErr }}
		las := &oaMockLinkAccountStore{}

		_, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("AddLinkedAccountToDB should never be called when AddAccount fails")
		}
	})

	t.Run("OA-22_AddLinkFailsCleanupSucceeds", func(t *testing.T) {
		wantErr := testutil.ErrInsertFailed
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return wantErr }}

		_, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1 (cleanup)", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-23_AddLinkFailsCleanupAlsoFails", func(t *testing.T) {
		linkErr := testutil.ErrInsertFailed
		cleanupErr := testutil.ErrBoom
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return linkErr }}

		_, err := resolveOrCreateAccountForPlatformUser(testutil.LogContext(context.Background()), as, las, PlatformDiscord, user)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !errors.Is(err, cleanupErr) {
			t.Errorf("error = %v, want the cleanup error %v", err, cleanupErr)
		}
	})

	t.Run("OA-24_RaceLostCleanupAndWinnerFetchSucceed", func(t *testing.T) {
		winner := &account.Account{UserID: "winner-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(id string) (*account.Account, error) {
			if id == "winner-1" {
				return winner, nil
			}
			return nil, account.ErrNotFound
		}}
		var lookupCalls int
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				lookupCalls++
				if lookupCalls == 1 {
					return nil, account.ErrNotFound
				}
				return &LinkedAccount{UserID: "winner-1"}, nil
			},
		}

		got, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
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
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{}
		var lookupCalls int
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				lookupCalls++
				if lookupCalls == 1 {
					return nil, account.ErrNotFound
				}
				return nil, wantErr
			},
		}

		_, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-26_RaceLostWinnerAccountFetchFails", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return nil, wantErr }}
		var lookupCalls int
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				lookupCalls++
				if lookupCalls == 1 {
					return nil, account.ErrNotFound
				}
				return &LinkedAccount{UserID: "winner-1"}, nil
			},
		}

		_, err := resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}

func oaSetupDiscordUserFailure(t *testing.T, cfg *config.Config) {
	t.Helper()
	tokenTS := oaJSONServer(http.StatusOK, `{"access_token":"disc-at","token_type":"Bearer","expires_in":3600,"scope":"identify"}`)
	t.Cleanup(tokenTS.Close)
	usersTS := oaJSONServer(http.StatusUnauthorized, `{"message":"401: Unauthorized","code":0}`)
	t.Cleanup(usersTS.Close)
	setDiscordUsersEndpoint(t, usersTS)

	cfg.Discord.OAuth2Config = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenTS.URL}}
}

func TestOA27to42ProcessOAuthLink(t *testing.T) {
	activeSession := func(userID string) *session.Session {
		return &session.Session{ID: "sess-1", UserID: userID, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	}

	t.Run("OA-27_DiscordLinksToSession", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordPlatform(t, cfg)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLink})
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

		cfg := testutil.NewConfig()
		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLink})
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("OA-29_ExpiredSession", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}
		expired := &session.Session{ID: "sess-1", UserID: "user-1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		req := oaRequestWithSession(expired)

		cfg := testutil.NewConfig()
		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLink})
		if !errors.Is(err, ErrSessionExpired) {
			t.Fatalf("error = %v, want %v", err, ErrSessionExpired)
		}
	})

	t.Run("OA-30_InvalidPlatform", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		cfg := testutil.NewConfig()
		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: "bogus", Mode: ModeLink})
		if !errors.Is(err, ErrInvalidPlatform) {
			t.Fatalf("error = %v, want %v", err, ErrInvalidPlatform)
		}
	})

	t.Run("OA-31_ExchangeFails", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordExchangeFailure(t, cfg)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "bad-code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("OA-32_MinecraftBothIdentitiesNew", func(t *testing.T) {
		javaUUID := "22222222-2222-2222-2222-222222222222"
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
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
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{})
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
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
		cfg := testutil.NewConfig()
		xuid := oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: "33333333-3333-3333-3333-333333333333"})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive && id == xuid {
					return &LinkedAccount{UserID: "other-user", Platform: platform, PlatformID: id}, nil
				}
				return nil, account.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformXboxLive, Mode: ModeLink})
		if !errors.Is(err, ErrPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("error = %v, want ErrPlatformAlreadyLinkedToDifferentAccount", err)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no links to be written")
		}
	})

	t.Run("OA-35_JavaAlreadyLinkedToDifferentAccountCommitsNothing", func(t *testing.T) {
		javaUUID := "44444444-4444-4444-4444-444444444444"
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformMinecraft && id == javaUUID {
					return &LinkedAccount{UserID: "other-user", Platform: platform, PlatformID: id}, nil
				}
				return nil, account.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
		if !errors.Is(err, ErrPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("error = %v, want ErrPlatformAlreadyLinkedToDifferentAccount", err)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected the Xbox identity to NOT be committed once the Java check fails")
		}
	})

	t.Run("OA-36_XboxPrecheckLookupErrorPropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive {
					return nil, wantErr
				}
				return nil, account.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformXboxLive, Mode: ModeLink})
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no links to be written")
		}
	})

	t.Run("OA-37_JavaPrecheckLookupErrorPropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		javaUUID := "55555555-5555-5555-5555-555555555555"
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformMinecraft {
					return nil, wantErr
				}
				return nil, account.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
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
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID, xuid: xuid})
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if (platform == PlatformXboxLive && id == xuid) || (platform == PlatformMinecraft && id == javaUUID) {
					return &LinkedAccount{UserID: "user-1", Platform: platform, PlatformID: id}, nil
				}
				return nil, account.ErrNotFound
			},
		}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
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
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{xstsFails: true})
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
		if las.GetLinkedAccountByPlatformIDCalls != 0 || las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no link-store calls when the identity fetch itself fails")
		}
	})

	t.Run("OA-40_XboxLinkWriteFailsJavaNeverAttempted", func(t *testing.T) {
		javaUUID := "77777777-7777-7777-7777-777777777777"
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID})
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return testutil.ErrInsertFailed }}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
		if !errors.Is(err, ErrLinkAccountFailed) {
			t.Fatalf("error = %v, want %v", err, ErrLinkAccountFailed)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1 (java never attempted)", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-41_PlatformUserFetchFails", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordUserFailure(t, cfg)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Error("expected no link-store calls when the platform user fetch fails")
		}
	})

	t.Run("OA-42_FinalLinkWriteFails", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupDiscordPlatform(t, cfg)
		wantErr := testutil.ErrInsertFailed
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return wantErr }}
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformDiscord, Mode: ModeLink})
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestOA79to81ProcessOAuthLinkAdditionalPlatformDispatch(t *testing.T) {
	activeSession := func(userID string) *session.Session {
		return &session.Session{ID: "sess-1", UserID: userID, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	}

	t.Run("OA-79_TwitchLinksToSession", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupTwitchPlatform(t, cfg)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformTwitch, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil || session.UserID != "user-1" {
			t.Errorf("session = %+v, want UserID=user-1", session)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != PlatformTwitch {
			t.Errorf("expected a Twitch linked account, got %+v", las.AddedLinkedAccounts)
		}
	})

	t.Run("OA-80_MicrosoftPlainLinksToSession", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupMicrosoftLoginPlatform(t, cfg)
		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMicrosoft, Mode: ModeLink})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil || session.UserID != "user-1" {
			t.Errorf("session = %+v, want UserID=user-1", session)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != PlatformMicrosoft {
			t.Errorf("expected a Microsoft linked account, got %+v", las.AddedLinkedAccounts)
		}
	})

	t.Run("OA-81_XboxLiveOnlyNeverTouchesJavaEvenIfOwned", func(t *testing.T) {
		javaUUID := "99999999-9999-9999-9999-999999999999"
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{javaUUID: javaUUID})

		var mcLoginCalled, mcProfileCalled bool
		mcLoginTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcLoginCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcLoginTS.Close)
		cfg.Microsoft.MinecraftLoginURL = mcLoginTS.URL
		mcProfileTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mcProfileCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(mcProfileTS.Close)
		cfg.Microsoft.MinecraftProfileURL = mcProfileTS.URL

		las := &oaMockLinkAccountStore{}
		req := oaRequestWithSession(activeSession("user-1"))

		session, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformXboxLive, Mode: ModeLink})
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
		if len(las.AddedLinkedAccounts) != 1 || las.AddedLinkedAccounts[0].Platform != PlatformXboxLive {
			t.Errorf("expected only the Xbox Live identity to be linked, got %+v", las.AddedLinkedAccounts)
		}
	})
}

func TestOA43to55ResolveOrCreateAccountForMicrosoftUser(t *testing.T) {
	xbox := &XboxLiveData{XUID: "xid1", Gamertag: "Tag"}
	java := &MinecraftData{ID: mustUUID("11111111-1111-1111-1111-111111111111"), Username: "Steve"}

	notFoundStore := func() *oaMockLinkAccountStore {
		return &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, account.ErrNotFound }}
	}

	t.Run("OA-43_NeitherLinkedJavaPresentPrefersJavaUsername", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := notFoundStore()

		got, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, java)
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

		got, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, nil)
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
		existing := &account.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive {
					return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return nil, account.ErrNotFound
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, nil)
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
		existing := &account.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformMinecraft {
					return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return nil, account.ErrNotFound
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, java)
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
		existing := &account.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, java)
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
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive {
					return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return &LinkedAccount{UserID: "user-2", Verified: true, LoginEnabled: true}, nil
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, java)
		if !errors.Is(err, ErrConflictingMicrosoftIdentities) {
			t.Fatalf("error = %v, want ErrConflictingMicrosoftIdentities", err)
		}
	})

	t.Run("OA-49_NoEligibleIdentityRejected", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive {
					return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: false}, nil
				}
				return nil, account.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, nil)
		if !errors.Is(err, ErrPlatformLoginDisabled) {
			t.Fatalf("error = %v, want ErrPlatformLoginDisabled", err)
		}
	})

	t.Run("OA-50_OneIneligibleOtherEligibleSucceeds", func(t *testing.T) {
		existing := &account.Account{UserID: "user-1"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return existing, nil }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive {
					return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: false}, nil
				}
				return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
			},
		}

		got, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, java)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != existing {
			t.Errorf("account = %+v, want the existing account", got)
		}
	})

	t.Run("OA-51_XboxLookupErrorPropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive {
					return nil, wantErr
				}
				return nil, account.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-52_JavaLookupErrorPropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformMinecraft {
					return nil, wantErr
				}
				return nil, account.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, java)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-53_AddAccountFails", func(t *testing.T) {
		wantErr := testutil.ErrInsertFailed
		as := &oaMockAccountService{AddAccountFunc: func(*account.Account) error { return wantErr }}
		las := notFoundStore()

		_, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-54_GetAccountByIDFails", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return nil, wantErr }}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(platform Platform, id string) (*LinkedAccount, error) {
				if platform == PlatformXboxLive {
					return &LinkedAccount{UserID: "user-1", Verified: true, LoginEnabled: true}, nil
				}
				return nil, account.ErrNotFound
			},
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-55_EnsureIdentityLinkedFails", func(t *testing.T) {
		wantErr := testutil.ErrInsertFailed
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, account.ErrNotFound },
			AddLinkedAccountToDBFunc:         func(*LinkedAccount) error { return wantErr },
		}

		_, err := resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, nil)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}

func TestOA56to66EnsureMicrosoftIdentityLinked(t *testing.T) {
	xbox := &XboxLiveData{XUID: "xid1", Gamertag: "Tag"}
	acct := &account.Account{UserID: "user-1"}

	t.Run("OA-56_Success", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}

		gotAccount, gotIsNew, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, true, PlatformXboxLive, xbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAccount != acct || gotIsNew != false {
			t.Errorf("got (%+v, %v), want (%+v, false)", gotAccount, gotIsNew, acct)
		}
	})

	t.Run("OA-57_LinkFailsNewAccountCleanupSucceeds", func(t *testing.T) {
		wantErr := testutil.ErrInsertFailed
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return wantErr }}

		_, _, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-58_LinkFailsExistingAccountNoCleanup", func(t *testing.T) {
		wantErr := testutil.ErrInsertFailed
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return wantErr }}

		_, _, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, false, PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 0 {
			t.Errorf("DeleteAccount called %d times, want 0 (not a new account)", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-59_LinkFailsCleanupAlsoFails", func(t *testing.T) {
		linkErr := testutil.ErrInsertFailed
		cleanupErr := testutil.ErrBoom
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return linkErr }}

		_, _, err := ensureMicrosoftIdentityLinked(testutil.LogContext(context.Background()), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, cleanupErr) {
			t.Errorf("error = %v, want the cleanup error %v", err, cleanupErr)
		}
	})

	t.Run("OA-60_AlreadyLinkedButWeAreTheOwner", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "user-1"}, nil
			},
		}

		gotAccount, gotIsNew, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, true, PlatformXboxLive, xbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAccount != acct || gotIsNew != false {
			t.Errorf("got (%+v, %v), want (%+v, false)", gotAccount, gotIsNew, acct)
		}
		if as.DeleteAccountCalls != 0 {
			t.Error("expected no cleanup: we already own the identity")
		}
	})

	t.Run("OA-61_AlreadyLinkedRefetchFailsNewAccountCleanupSucceeds", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc:         func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, wantErr },
		}

		_, _, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		if as.DeleteAccountCalls != 1 {
			t.Errorf("DeleteAccount called %d times, want 1", as.DeleteAccountCalls)
		}
	})

	t.Run("OA-86_AlreadyLinkedOwnerVanishedExistingAccountConflict", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc:         func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, account.ErrNotFound },
		}

		_, _, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, false, PlatformXboxLive, xbox)
		if !errors.Is(err, ErrConflictingMicrosoftIdentities) {
			t.Fatalf("error = %v, want ErrConflictingMicrosoftIdentities", err)
		}
	})

	t.Run("OA-62_AlreadyLinkedRefetchFailsCleanupAlsoFails", func(t *testing.T) {
		lookupErr := testutil.ErrDBDown
		cleanupErr := testutil.ErrBoom
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc:         func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, lookupErr },
		}

		_, _, err := ensureMicrosoftIdentityLinked(testutil.LogContext(context.Background()), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, cleanupErr) {
			t.Errorf("error = %v, want the cleanup error %v", err, cleanupErr)
		}
	})

	t.Run("OA-63_RealConflictExistingAccountRejected", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "other-user"}, nil
			},
		}

		_, _, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, false, PlatformXboxLive, xbox)
		if !errors.Is(err, ErrConflictingMicrosoftIdentities) {
			t.Fatalf("error = %v, want ErrConflictingMicrosoftIdentities", err)
		}
	})

	t.Run("OA-64_RealConflictNewAccountUsesWinner", func(t *testing.T) {
		winner := &account.Account{UserID: "other-user"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(id string) (*account.Account, error) {
			if id == "other-user" {
				return winner, nil
			}
			return nil, account.ErrNotFound
		}}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "other-user"}, nil
			},
		}

		gotAccount, gotIsNew, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, true, PlatformXboxLive, xbox)
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
		cleanupErr := testutil.ErrBoom
		as := &oaMockAccountService{DeleteAccountFunc: func(string) error { return cleanupErr }}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "other-user"}, nil
			},
		}

		_, _, err := ensureMicrosoftIdentityLinked(testutil.LogContext(context.Background()), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("error = %v, want the cleanup error %v", err, cleanupErr)
		}
	})

	t.Run("OA-66_RealConflictNewAccountWinnerFetchFails", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return nil, wantErr }}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "other-user"}, nil
			},
		}

		_, _, err := ensureMicrosoftIdentityLinked(context.Background(), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}

func TestOA67to71and85LinkPlatformUserToSession(t *testing.T) {
	user := &oaIdentity{id: "p1", username: "alice"}

	t.Run("OA-67_NotYetLinked", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}

		err := linkPlatformUserToSession(context.Background(), las, "user-1", PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if las.AddLinkedAccountToDBCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-68_AlreadyLinkedToSameAccountNoOp", func(t *testing.T) {
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "user-1"}, nil
			},
		}

		err := linkPlatformUserToSession(context.Background(), las, "user-1", PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 0", las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-69_AlreadyLinkedToDifferentAccountRejected", func(t *testing.T) {
		las := &oaMockLinkAccountStore{
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "other-user"}, nil
			},
		}

		err := linkPlatformUserToSession(context.Background(), las, "user-1", PlatformDiscord, user)
		if !errors.Is(err, ErrPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("error = %v, want ErrPlatformAlreadyLinkedToDifferentAccount", err)
		}
	})

	t.Run("OA-70_LookupErrorPropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		las := &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, wantErr }}

		err := linkPlatformUserToSession(context.Background(), las, "user-1", PlatformDiscord, user)
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("OA-71_AddLinkedAccountFails", func(t *testing.T) {
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return testutil.ErrInsertFailed }}

		err := linkPlatformUserToSession(testutil.LogContext(context.Background()), las, "user-1", PlatformDiscord, user)
		if !errors.Is(err, ErrLinkAccountFailed) {
			t.Fatalf("error = %v, want %v", err, ErrLinkAccountFailed)
		}
	})

	t.Run("OA-85_AddLinkedAccountAlreadyLinkedWinnerVanishedOnReread", func(t *testing.T) {
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked }}

		err := linkPlatformUserToSession(testutil.LogContext(context.Background()), las, "user-1", PlatformDiscord, user)
		if !errors.Is(err, ErrLinkAccountFailed) {
			t.Fatalf("error = %v, want %v", err, ErrLinkAccountFailed)
		}
	})

	lostRaceStore := func(rereads func() (*LinkedAccount, error)) *oaMockLinkAccountStore {
		calls := 0
		return &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				calls++
				if calls == 1 {
					return nil, account.ErrNotFound
				}
				return rereads()
			},
		}
	}

	t.Run("OA-93_LostRaceSameOwnerLinksNil", func(t *testing.T) {
		las := lostRaceStore(func() (*LinkedAccount, error) { return &LinkedAccount{UserID: "user-1"}, nil })

		err := linkPlatformUserToSession(testutil.LogContext(context.Background()), las, "user-1", PlatformDiscord, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("OA-94_LostRaceOtherOwnerConflict", func(t *testing.T) {
		las := lostRaceStore(func() (*LinkedAccount, error) { return &LinkedAccount{UserID: "other-user"}, nil })

		err := linkPlatformUserToSession(testutil.LogContext(context.Background()), las, "user-1", PlatformDiscord, user)
		if !errors.Is(err, ErrPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("error = %v, want %v", err, ErrPlatformAlreadyLinkedToDifferentAccount)
		}
	})

	t.Run("OA-95_LostRaceRereadFails", func(t *testing.T) {
		las := lostRaceStore(func() (*LinkedAccount, error) { return nil, testutil.ErrDBDown })

		err := linkPlatformUserToSession(testutil.LogContext(context.Background()), las, "user-1", PlatformDiscord, user)
		if !errors.Is(err, ErrLinkAccountFailed) {
			t.Fatalf("error = %v, want %v", err, ErrLinkAccountFailed)
		}
	})

	t.Run("OA-101_SecondAccountOnSamePlatformConflicts", func(t *testing.T) {
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrUserAlreadyLinkedToPlatform }}

		err := linkPlatformUserToSession(testutil.LogContext(context.Background()), las, "user-1", PlatformDiscord, user)
		if err != ErrUserAlreadyLinkedToPlatform {
			t.Fatalf("error = %v, want %v", err, ErrUserAlreadyLinkedToPlatform)
		}
	})
}

func TestOA87to92LinkHelpersDetachedWritesIgnoreCancel(t *testing.T) {
	const marker = "marker-87-92"
	cancelledCtx := func() context.Context {
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), oaCtxMarkerKey{}, marker))
		cancel()
		return ctx
	}
	checkRecord := func(t *testing.T, name string, rec oaCtxRecord, wantCalls int) {
		t.Helper()
		if len(rec.Errs) != wantCalls {
			t.Fatalf("%s recorded %d calls, want %d", name, len(rec.Errs), wantCalls)
		}
		for i := range rec.Errs {
			if rec.Errs[i] != nil {
				t.Errorf("%s call %d: ctx.Err() = %v, want nil", name, i, rec.Errs[i])
			}
			if rec.Markers[i] != marker {
				t.Errorf("%s call %d: marker = %v, want %q", name, i, rec.Markers[i], marker)
			}
		}
	}
	user := &oaIdentity{id: "p1", username: "alice", email: "a@b.com"}
	xbox := &XboxLiveData{XUID: "xid1", Gamertag: "Tag"}
	acct := &account.Account{UserID: "user-1"}

	t.Run("OA-87_CreatedAccountLinkFailsCleanupRunsDetached", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return testutil.ErrInsertFailed }}

		_, err := resolveOrCreateAccountForPlatformUser(cancelledCtx(), as, las, PlatformDiscord, user)
		if !errors.Is(err, testutil.ErrInsertFailed) {
			t.Fatalf("error = %v, want %v", err, testutil.ErrInsertFailed)
		}
		checkRecord(t, "AddLinkedAccountToDB", las.AddCtx, 1)
		checkRecord(t, "DeleteAccount", as.DeleteCtx, 1)
	})

	t.Run("OA-88_MicrosoftLinkSucceedsDetached", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}

		gotAccount, gotIsNew, err := ensureMicrosoftIdentityLinked(cancelledCtx(), as, las, acct, true, PlatformXboxLive, xbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAccount != acct || gotIsNew {
			t.Errorf("got (%+v, %v), want (%+v, false)", gotAccount, gotIsNew, acct)
		}
		checkRecord(t, "AddLinkedAccountToDB", las.AddCtx, 1)
	})

	t.Run("OA-89_MicrosoftLinkFailsNewAccountCleanupDetached", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return testutil.ErrInsertFailed }}

		_, _, err := ensureMicrosoftIdentityLinked(cancelledCtx(), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, testutil.ErrInsertFailed) {
			t.Fatalf("error = %v, want %v", err, testutil.ErrInsertFailed)
		}
		checkRecord(t, "DeleteAccount", as.DeleteCtx, 1)
	})

	t.Run("OA-90_MicrosoftLookupFailsNewAccountCleanupDetached", func(t *testing.T) {
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return nil, testutil.ErrDBDown
			},
		}

		_, _, err := ensureMicrosoftIdentityLinked(cancelledCtx(), as, las, acct, true, PlatformXboxLive, xbox)
		if !errors.Is(err, testutil.ErrDBDown) {
			t.Fatalf("error = %v, want %v", err, testutil.ErrDBDown)
		}
		checkRecord(t, "DeleteAccount", as.DeleteCtx, 1)
	})

	t.Run("OA-91_MicrosoftLostRaceNewAccountCleanupDetached", func(t *testing.T) {
		winner := &account.Account{UserID: "winner"}
		as := &oaMockAccountService{GetAccountByIDFunc: func(string) (*account.Account, error) { return winner, nil }}
		las := &oaMockLinkAccountStore{
			AddLinkedAccountToDBFunc: func(*LinkedAccount) error { return ErrAlreadyLinked },
			GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
				return &LinkedAccount{UserID: "winner"}, nil
			},
		}

		got, gotIsNew, err := ensureMicrosoftIdentityLinked(cancelledCtx(), as, las, acct, true, PlatformXboxLive, xbox)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != winner || gotIsNew {
			t.Errorf("got (%+v, %v), want (%+v, false)", got, gotIsNew, winner)
		}
		checkRecord(t, "DeleteAccount", as.DeleteCtx, 1)
	})

	t.Run("OA-92_SessionLinkWriteDetached", func(t *testing.T) {
		las := &oaMockLinkAccountStore{}

		if err := linkPlatformUserToSession(cancelledCtx(), las, "user-1", PlatformDiscord, user); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		checkRecord(t, "AddLinkedAccountToDB", las.AddCtx, 1)
	})
}

// The functions under test hold no locks themselves; the race is between
// separate callers, arbitrated by the store's (platform, platform_id)
// uniqueness constraint. The doubles below enforce that constraint
// atomically under a mutex, so launching real goroutines against them
// exercises the actual race instead of just asserting behavior for a canned
// ErrAlreadyLinked return.

type oaConcurrentAccountService struct {
	*accountMap
}

var _ account.Service = (*oaConcurrentAccountService)(nil)

func newOAConcurrentAccountService() *oaConcurrentAccountService {
	return &oaConcurrentAccountService{newAccountMap()}
}

func (m *oaConcurrentAccountService) GetAccountByUsername(context.Context, string) (*account.Account, error) {
	return nil, account.ErrNotFound
}
func (m *oaConcurrentAccountService) GetAccountByEmail(context.Context, string) (*account.Account, error) {
	return nil, account.ErrNotFound
}
func (m *oaConcurrentAccountService) IsPasswordAuthEnabled(context.Context, string) (bool, error) {
	return false, nil
}
func (m *oaConcurrentAccountService) NewSession(_ context.Context, a *account.Account, expiresAt int64) (*session.Session, error) {
	return &session.Session{ID: "stub-session", UserID: a.UserID, Permissions: []string{"stub:perm"}, ExpiresAt: expiresAt}, nil
}

func oaLinkKey(platform Platform, platformID string) string {
	return string(platform) + "|" + platformID
}

type oaConcurrentLinkAccountStore struct {
	mu    sync.Mutex
	byKey map[string]*LinkedAccount
}

var _ Store = (*oaConcurrentLinkAccountStore)(nil)

func newOAConcurrentLinkAccountStore() *oaConcurrentLinkAccountStore {
	return &oaConcurrentLinkAccountStore{byKey: map[string]*LinkedAccount{}}
}

func (m *oaConcurrentLinkAccountStore) AddLinkedAccountToDB(_ context.Context, la *LinkedAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := oaLinkKey(la.Platform, la.PlatformID)
	if _, exists := m.byKey[key]; exists {
		return ErrAlreadyLinked
	}
	m.byKey[key] = la
	return nil
}
func (m *oaConcurrentLinkAccountStore) UpdateLinkedAccount(context.Context, *LinkedAccount) error {
	return nil
}
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountByPlatformID(_ context.Context, platform Platform, platformID string) (*LinkedAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	la, ok := m.byKey[oaLinkKey(platform, platformID)]
	if !ok {
		return nil, account.ErrNotFound
	}
	return la, nil
}
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountByPlatformName(context.Context, Platform, string) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountByUserID(context.Context, string, Platform) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}
func (m *oaConcurrentLinkAccountStore) GetLinkedAccountsByUserID(context.Context, string) ([]*LinkedAccount, error) {
	return nil, nil
}
func (m *oaConcurrentLinkAccountStore) DeleteLinkedAccount(context.Context, string, Platform) error {
	return nil
}
func (m *oaConcurrentLinkAccountStore) SetLinkedAccountLoginEnabled(context.Context, string, Platform, bool) error {
	return nil
}

func TestOA72ResolveOrCreateAccountForPlatformUserConcurrentRace(t *testing.T) {
	t.Run("OA-72_ConcurrentCallersConvergeOnOneAccount", func(t *testing.T) {
		as := newOAConcurrentAccountService()
		las := newOAConcurrentLinkAccountStore()
		user := &oaIdentity{id: "race-1", username: "alice"}

		const n = 2
		var wg sync.WaitGroup
		results := make([]*account.Account, n)
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				defer wg.Done()
				results[i], errs[i] = resolveOrCreateAccountForPlatformUser(context.Background(), as, las, PlatformDiscord, user)
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

		placeholderA := &account.Account{UserID: "placeholder-a"}
		placeholderB := &account.Account{UserID: "placeholder-b"}
		_ = as.AddAccount(context.Background(), placeholderA)
		_ = as.AddAccount(context.Background(), placeholderB)

		var wg sync.WaitGroup
		var resultA, resultB *account.Account
		var errA, errB error
		wg.Add(2)
		go func() {
			defer wg.Done()
			resultA, _, errA = ensureMicrosoftIdentityLinked(context.Background(), as, las, placeholderA, true, PlatformXboxLive, xbox)
		}()
		go func() {
			defer wg.Done()
			resultB, _, errB = ensureMicrosoftIdentityLinked(context.Background(), as, las, placeholderB, true, PlatformXboxLive, xbox)
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

		existingAccount := &account.Account{UserID: "existing-account"}
		freshPlaceholder := &account.Account{UserID: "fresh-placeholder"}
		_ = as.AddAccount(context.Background(), existingAccount)
		_ = as.AddAccount(context.Background(), freshPlaceholder)

		var wg sync.WaitGroup
		var resultA, resultB *account.Account
		var errA, errB error
		wg.Add(2)
		go func() {
			defer wg.Done()
			resultA, _, errA = ensureMicrosoftIdentityLinked(context.Background(), as, las, existingAccount, false, PlatformXboxLive, xbox)
		}()
		go func() {
			defer wg.Done()
			resultB, _, errB = ensureMicrosoftIdentityLinked(context.Background(), as, las, freshPlaceholder, true, PlatformXboxLive, xbox)
		}()
		wg.Wait()

		if _, err := as.GetAccountByID(context.Background(), existingAccount.UserID); err != nil {
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
			if _, err := as.GetAccountByID(context.Background(), freshPlaceholder.UserID); !errors.Is(err, account.ErrNotFound) {
				t.Errorf("expected B's placeholder to have been deleted after losing, got err=%v", err)
			}
			if got := as.count(); got != 1 {
				t.Errorf("account store has %d accounts, want exactly 1", got)
			}
		case errA != nil && errB == nil:
			if !errors.Is(errA, ErrConflictingMicrosoftIdentities) {
				t.Errorf("A's error = %v, want ErrConflictingMicrosoftIdentities", errA)
			}
			if resultB.UserID != freshPlaceholder.UserID {
				t.Errorf("B resolved to %s, want its own account %s", resultB.UserID, freshPlaceholder.UserID)
			}
			if _, err := as.GetAccountByID(context.Background(), freshPlaceholder.UserID); err != nil {
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
		results := make([]*account.Account, n)
		errs := make([]error, n)
		wg.Add(n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				defer wg.Done()
				results[i], errs[i] = resolveOrCreateAccountForMicrosoftUser(context.Background(), as, las, xbox, java)
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

		xboxLink, err := las.GetLinkedAccountByPlatformID(context.Background(), PlatformXboxLive, xbox.XUID)
		if err != nil || xboxLink.UserID != results[0].UserID {
			t.Errorf("xbox link = %+v (err=%v), want it linked to %s", xboxLink, err, results[0].UserID)
		}
		javaLink, err := las.GetLinkedAccountByPlatformID(context.Background(), PlatformMinecraft, java.ID.String())
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
	byKey map[string]*LinkedAccount

	barrierMu    sync.Mutex
	barrierWG    sync.WaitGroup
	barrierN     int
	barrierCount int
}

var _ Store = (*oaBarrierLinkAccountStore)(nil)

func newOABarrierLinkAccountStore(barrierN int) *oaBarrierLinkAccountStore {
	s := &oaBarrierLinkAccountStore{byKey: map[string]*LinkedAccount{}, barrierN: barrierN}
	s.barrierWG.Add(barrierN)
	return s
}

func (m *oaBarrierLinkAccountStore) GetLinkedAccountByPlatformID(_ context.Context, platform Platform, platformID string) (*LinkedAccount, error) {
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
		return nil, account.ErrNotFound
	}
	cp := *la
	return &cp, nil
}
func (m *oaBarrierLinkAccountStore) AddLinkedAccountToDB(_ context.Context, la *LinkedAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := oaLinkKey(la.Platform, la.PlatformID)
	if _, exists := m.byKey[key]; exists {
		return ErrAlreadyLinked
	}
	cp := *la
	m.byKey[key] = &cp
	return nil
}
func (m *oaBarrierLinkAccountStore) UpdateLinkedAccount(context.Context, *LinkedAccount) error {
	return nil
}
func (m *oaBarrierLinkAccountStore) GetLinkedAccountByPlatformName(context.Context, Platform, string) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}
func (m *oaBarrierLinkAccountStore) GetLinkedAccountByUserID(context.Context, string, Platform) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}
func (m *oaBarrierLinkAccountStore) GetLinkedAccountsByUserID(context.Context, string) ([]*LinkedAccount, error) {
	return nil, nil
}
func (m *oaBarrierLinkAccountStore) DeleteLinkedAccount(context.Context, string, Platform) error {
	return nil
}
func (m *oaBarrierLinkAccountStore) SetLinkedAccountLoginEnabled(context.Context, string, Platform, bool) error {
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
			errs[0] = linkPlatformUserToSession(context.Background(), las, "user-A", PlatformDiscord, user)
		}()
		go func() {
			defer wg.Done()
			errs[1] = linkPlatformUserToSession(context.Background(), las, "user-B", PlatformDiscord, user)
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

		la, err := las.GetLinkedAccountByPlatformID(context.Background(), PlatformDiscord, "race-82")
		if err != nil {
			t.Fatalf("expected the linked account to exist after the race, got err=%v", err)
		}
		if la.UserID != "user-A" && la.UserID != "user-B" {
			t.Errorf("linked account UserID = %q, want user-A or user-B (the winner), not duplicated or overwritten by a third value", la.UserID)
		}
	})
}

func TestOA96LinkPlatformUserToSessionConcurrentLinksSettle(t *testing.T) {
	t.Run("OA-96_ConcurrentLinksSettle", func(t *testing.T) {
		link := func(las Store, user *oaIdentity, userIDs ...string) []error {
			errs := make([]error, len(userIDs))
			var wg sync.WaitGroup
			wg.Add(len(userIDs))
			for i, userID := range userIDs {
				go func() {
					defer wg.Done()
					errs[i] = linkPlatformUserToSession(testutil.LogContext(context.Background()), las, userID, PlatformDiscord, user)
				}()
			}
			wg.Wait()
			return errs
		}

		for _, err := range link(newOAConcurrentLinkAccountStore(), &oaIdentity{id: "race-96a", username: "alice"}, "user-A", "user-A") {
			if err != nil {
				t.Errorf("same user: error = %v, want nil", err)
			}
		}

		nils, conflicts := 0, 0
		for _, err := range link(newOAConcurrentLinkAccountStore(), &oaIdentity{id: "race-96b", username: "alice"}, "user-A", "user-B") {
			switch {
			case err == nil:
				nils++
			case errors.Is(err, ErrPlatformAlreadyLinkedToDifferentAccount):
				conflicts++
			default:
				t.Errorf("two users: unexpected error %v", err)
			}
		}
		if nils != 1 || conflicts != 1 {
			t.Errorf("two users: %d nil and %d conflict, want 1 and 1", nils, conflicts)
		}
	})
}

func TestOA97to100MicrosoftJavaFailure(t *testing.T) {
	activeSession := func(userID string) *session.Session {
		return &session.Session{ID: "sess-1", UserID: userID, ExpiresAt: time.Now().Add(time.Hour).Unix()}
	}
	linkedXboxStore := func(owner string) *oaMockLinkAccountStore {
		return &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(platform Platform, _ string) (*LinkedAccount, error) {
			if platform == PlatformXboxLive {
				return &LinkedAccount{UserID: owner, Verified: true, LoginEnabled: true}, nil
			}
			return nil, account.ErrNotFound
		}}
	}

	t.Run("OA-97_LoginContinuesWhenOnlyJavaFailsForLinkedXbox", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{mcLoginFails: true})
		as := &oaMockAccountService{GetAccountByIDFunc: func(id string) (*account.Account, error) { return &account.Account{UserID: id}, nil }}
		las := linkedXboxStore("account-A")
		ss := &authtest.FakeSessionService{}

		got, err := ProcessOAuthLogin(testutil.LogContext(context.Background()), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLogin})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.UserID != "account-A" {
			t.Fatalf("session = %+v, want a session for account-A", got)
		}
		if as.AddAccountCalls != 0 || las.AddLinkedAccountToDBCalls != 0 {
			t.Errorf("AddAccount called %d times, AddLinkedAccountToDB %d times, want 0 and 0", as.AddAccountCalls, las.AddLinkedAccountToDBCalls)
		}
	})

	t.Run("OA-98_LoginFailsWhenJavaFailsAndXboxUnlinked", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{mcLoginFails: true})
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{}
		ss := &authtest.FakeSessionService{}

		_, err := ProcessOAuthLogin(testutil.LogContext(context.Background()), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLogin})
		if !errors.Is(err, ErrMinecraftLoginWithXbox) {
			t.Fatalf("error = %v, want %v", err, ErrMinecraftLoginWithXbox)
		}
		if as.AddAccountCalls != 0 || len(ss.Added) != 0 {
			t.Errorf("AddAccount called %d times, %d sessions added, want 0 and 0", as.AddAccountCalls, len(ss.Added))
		}
	})

	t.Run("OA-99_LoginFailsWhenXboxLookupErrors", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{mcLoginFails: true})
		as := &oaMockAccountService{}
		las := &oaMockLinkAccountStore{GetLinkedAccountByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) { return nil, testutil.ErrDBDown }}
		ss := &authtest.FakeSessionService{}

		_, err := ProcessOAuthLogin(testutil.LogContext(context.Background()), cfg, as, las, ss, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLogin})
		if !errors.Is(err, ErrMinecraftLoginWithXbox) {
			t.Fatalf("error = %v, want %v", err, ErrMinecraftLoginWithXbox)
		}
		if as.AddAccountCalls != 0 || len(ss.Added) != 0 {
			t.Errorf("AddAccount called %d times, %d sessions added, want 0 and 0", as.AddAccountCalls, len(ss.Added))
		}
	})

	t.Run("OA-100_LinkStillFailsWhenJavaFails", func(t *testing.T) {
		cfg := testutil.NewConfig()
		oaSetupMicrosoftChain(t, cfg, oaMicrosoftChainOpts{mcLoginFails: true})
		las := linkedXboxStore("user-1")
		req := oaRequestWithSession(activeSession("user-1"))

		_, err := ProcessOAuthLink(req, cfg, las, "code", &OAuthState{Platform: PlatformMinecraft, Mode: ModeLink})
		if !errors.Is(err, ErrMinecraftLoginWithXbox) {
			t.Fatalf("error = %v, want %v", err, ErrMinecraftLoginWithXbox)
		}
		if las.AddLinkedAccountToDBCalls != 0 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 0", las.AddLinkedAccountToDBCalls)
		}
	})
}
