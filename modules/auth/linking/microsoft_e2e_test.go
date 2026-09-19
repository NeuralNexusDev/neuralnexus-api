package linking

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// mockSessionService implements auth.SessionService for end-to-end
// ProcessOAuthLogin tests. Only AddSession is ever exercised by that path;
// the rest exist solely to satisfy the interface.
type mockSessionService struct {
	addErr    error
	addedOnce *auth.Session
}

var _ auth.SessionService = (*mockSessionService)(nil)

func (m *mockSessionService) AddSession(session *auth.Session) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.addedOnce = session
	return nil
}
func (m *mockSessionService) GetSession(string) (*auth.Session, error) { return nil, auth.ErrNotFound }
func (m *mockSessionService) UpdateSession(*auth.Session) error        { return nil }
func (m *mockSessionService) DeleteSession(string) error               { return nil }
func (m *mockSessionService) CreateJWT(*auth.Session) (string, error)  { return "", nil }
func (m *mockSessionService) ReadJWT(string) (*auth.Session, error)    { return nil, auth.ErrNotFound }

func newMinecraftOAuthState() *OAuthState {
	return &OAuthState{Platform: auth.PlatformMinecraft, Nonce: "n", RedirectURI: "https://example.com/done", Mode: ModeLogin}
}

// -------------- ProcessOAuthLogin (Minecraft/Microsoft) end to end --------------

func TestProcessOAuthLoginMinecraftOwnsJavaCreatesAccountWithBothIdentities(t *testing.T) {
	newFullChainServer(t, true)
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newMinecraftOAuthState())
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session == nil || ss.addedOnce != session {
		t.Fatal("expected the new session to be added via SessionService")
	}
	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	var linkedPlatforms []auth.Platform
	for _, la := range als.addCalls {
		linkedPlatforms = append(linkedPlatforms, la.Platform)
	}
	if len(linkedPlatforms) != 2 {
		t.Fatalf("expected both Xbox Live and Java identities to be linked, got: %v", linkedPlatforms)
	}
}

func TestProcessOAuthLoginMinecraftBedrockOnlyCreatesAccountWithXboxIdentityOnly(t *testing.T) {
	newFullChainServer(t, false)
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newMinecraftOAuthState())
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected a session for a Bedrock-only account")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive {
		t.Fatalf("expected only the Xbox Live identity to be linked, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLoginMinecraftReusesExistingLinkedAccount(t *testing.T) {
	newFullChainServer(t, true)
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing-acct"}, nil
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newMinecraftOAuthState())
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session.UserID != "existing-acct" {
		t.Errorf("expected the session to belong to the existing account, got %q", session.UserID)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d", len(as.accounts))
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no new links when both identities already resolve to the same account, got %d", len(als.addCalls))
	}
}

func TestProcessOAuthLoginMinecraftXstsErrorPropagates(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token"})
	})
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"XErr": 2148916233})
	})
	originalTokenURL := MicrosoftConfig.Endpoint.TokenURL
	t.Cleanup(func() { MicrosoftConfig.Endpoint.TokenURL = originalTokenURL })
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "ms-token", "scope": "XboxLive.signin"})
	}))
	t.Cleanup(tokenServer.Close)
	MicrosoftConfig.Endpoint.TokenURL = tokenServer.URL

	as := newMockAccountService()
	als := &mockLinkAccountStore{}
	ss := &mockSessionService{}

	_, err := ProcessOAuthLogin(as, als, ss, "some-code", newMinecraftOAuthState())
	if !errors.Is(err, ErrNoXboxAccount) {
		t.Errorf("expected ErrNoXboxAccount to propagate out of ProcessOAuthLogin, got: %v", err)
	}
	if len(as.accounts) != 0 {
		t.Error("no account should be created when the Xbox Live authorization step itself fails")
	}
}

// TestProcessOAuthLoginMinecraftJavaProfileFailureFallsBackToXboxOnly is a
// regression test from the review pipeline (Finding B): a transient failure
// in the Java-ownership check, after Xbox Live authentication has already
// fully succeeded, must not fail the whole login - it should fall back to
// the Xbox Live identity alone, the same as a legitimate Bedrock-only
// account with no Java profile at all.
func TestProcessOAuthLoginMinecraftJavaProfileFailureFallsBackToXboxOnly(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token"})
	})
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Token": "xsts-token",
			"DisplayClaims": map[string]interface{}{
				"xui": []map[string]string{{"uhs": "uhs-1", "xid": "9999", "gtg": "TestGamer"}},
			},
		})
	})
	withServer(t, &minecraftLoginURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mcLoginWithXboxResponse{AccessToken: "mc-token"})
	})
	withServer(t, &minecraftProfileURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	originalTokenURL := MicrosoftConfig.Endpoint.TokenURL
	t.Cleanup(func() { MicrosoftConfig.Endpoint.TokenURL = originalTokenURL })
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "ms-token", "scope": "XboxLive.signin"})
	}))
	t.Cleanup(tokenServer.Close)
	MicrosoftConfig.Endpoint.TokenURL = tokenServer.URL

	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newMinecraftOAuthState())
	if err != nil {
		t.Fatalf("expected the login to fall back to Xbox-only instead of failing, got error: %v", err)
	}
	if session == nil {
		t.Fatal("expected a session despite the Java profile lookup failing")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive {
		t.Fatalf("expected only the Xbox Live identity to be linked, got: %+v", als.addCalls)
	}
}

// -------------- ProcessOAuthLink (Minecraft/Microsoft) end to end --------------

func linkRequestWithSession(session *auth.Session) *http.Request {
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	return httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
}

func TestProcessOAuthLinkMinecraftLinksBothIdentitiesToSession(t *testing.T) {
	newFullChainServer(t, true)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	state := newMinecraftOAuthState()
	state.Mode = ModeLink

	got, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err != nil {
		t.Fatalf("ProcessOAuthLink returned error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 2 {
		t.Fatalf("expected both identities to be linked to the session's account, got: %+v", als.addCalls)
	}
	for _, la := range als.addCalls {
		if la.UserID != "u1" {
			t.Errorf("expected both links to belong to the session's user, got %+v", la)
		}
	}
}

func TestProcessOAuthLinkMinecraftXboxAlreadyLinkedToDifferentAccountRejected(t *testing.T) {
	newFullChainServer(t, false)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}
	state := newMinecraftOAuthState()
	state.Mode = ModeLink

	_, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err == nil {
		t.Fatal("expected an error when the Xbox Live identity is already linked to a different account")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no links to be attempted, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLinkMinecraftBedrockOnlySkipsJavaLink(t *testing.T) {
	newFullChainServer(t, false)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	state := newMinecraftOAuthState()
	state.Mode = ModeLink

	_, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err != nil {
		t.Fatalf("ProcessOAuthLink returned error: %v", err)
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive {
		t.Fatalf("expected only the Xbox Live identity to be linked, got: %+v", als.addCalls)
	}
}

// TestProcessOAuthLinkMinecraftJavaAlreadyLinkedToDifferentAccountCommitsNothing
// is a regression test from the review pipeline (Finding A): when Xbox is
// unlinked but Java already belongs to a different, pre-existing account
// (an ordinary conflict, not even a race), the whole link must be rejected
// with NOTHING committed - not even the Xbox identity, which the old code
// would have already linked by the time the Java conflict was discovered,
// with no way to roll it back.
func TestProcessOAuthLinkMinecraftJavaAlreadyLinkedToDifferentAccountCommitsNothing(t *testing.T) {
	newFullChainServer(t, true)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformXboxLive {
				return nil, auth.ErrNotFound
			}
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}
	state := newMinecraftOAuthState()
	state.Mode = ModeLink

	_, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err == nil {
		t.Fatal("expected an error when the Java identity is already linked to a different account")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected NOTHING to be committed (not even Xbox) when Java conflicts, got: %+v", als.addCalls)
	}
}

// TestProcessOAuthLinkMinecraftJavaProfileFailureFallsBackToXboxOnly mirrors
// TestProcessOAuthLoginMinecraftJavaProfileFailureFallsBackToXboxOnly for the
// link path.
func TestProcessOAuthLinkMinecraftJavaProfileFailureFallsBackToXboxOnly(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token"})
	})
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Token": "xsts-token",
			"DisplayClaims": map[string]interface{}{
				"xui": []map[string]string{{"uhs": "uhs-1", "xid": "9999", "gtg": "TestGamer"}},
			},
		})
	})
	withServer(t, &minecraftLoginURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mcLoginWithXboxResponse{AccessToken: "mc-token"})
	})
	withServer(t, &minecraftProfileURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	originalTokenURL := MicrosoftConfig.Endpoint.TokenURL
	t.Cleanup(func() { MicrosoftConfig.Endpoint.TokenURL = originalTokenURL })
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "ms-token", "scope": "XboxLive.signin"})
	}))
	t.Cleanup(tokenServer.Close)
	MicrosoftConfig.Endpoint.TokenURL = tokenServer.URL

	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	state := newMinecraftOAuthState()
	state.Mode = ModeLink

	got, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err != nil {
		t.Fatalf("expected the link to fall back to Xbox-only instead of failing, got error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive {
		t.Fatalf("expected only the Xbox Live identity to be linked, got: %+v", als.addCalls)
	}
}
