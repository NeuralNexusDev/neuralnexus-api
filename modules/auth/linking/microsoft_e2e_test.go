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

func newXboxLiveOAuthState() *OAuthState {
	return &OAuthState{Platform: auth.PlatformXboxLive, Nonce: "n", RedirectURI: "https://example.com/done", Mode: ModeLogin}
}

func newMicrosoftOAuthState() *OAuthState {
	return &OAuthState{Platform: auth.PlatformMicrosoft, Nonce: "n", RedirectURI: "https://example.com/done", Mode: ModeLogin}
}

// newMicrosoftLoginServer wires up a fake Microsoft OIDC token endpoint
// (overriding MicrosoftLoginConfig, not MicrosoftConfig - the plain
// "sign in with Microsoft" login never touches XBL/XSTS/Minecraft Services
// at all) plus the userinfo endpoint, returning the given sub/name/email.
func newMicrosoftLoginServer(t *testing.T, sub, name, email string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ms-login-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "ms-login-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"scope":        "openid profile email offline_access",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer ms-login-access-token" {
			t.Errorf("expected Authorization: Bearer ms-login-access-token, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"sub": sub, "name": name, "email": email})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	originalTokenURL := MicrosoftLoginConfig.Endpoint.TokenURL
	t.Cleanup(func() { MicrosoftLoginConfig.Endpoint.TokenURL = originalTokenURL })
	MicrosoftLoginConfig.Endpoint.TokenURL = server.URL + "/ms-login-token"

	originalUserInfoURL := microsoftUserInfoURL
	t.Cleanup(func() { microsoftUserInfoURL = originalUserInfoURL })
	microsoftUserInfoURL = server.URL + "/userinfo"
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
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: true}, nil
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

// TestProcessOAuthLoginMinecraftJavaProfileFailureIsSurfaced: a real failure
// checking Java ownership (as opposed to a legitimate Bedrock-only account,
// where getMinecraftProfile returns nil, nil rather than an error) must
// surface as a login failure, not silently fall back to logging the user in
// with just their Xbox Live identity - someone who asked to log in with
// Minecraft should be told when that couldn't be verified, not quietly
// handed a different kind of account.
func TestProcessOAuthLoginMinecraftJavaProfileFailureIsSurfaced(t *testing.T) {
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
	if err == nil {
		t.Fatal("expected the login to fail when the Java-ownership check itself fails")
	}
	if session != nil {
		t.Errorf("expected no session when Java ownership couldn't be verified, got: %+v", session)
	}
	if len(as.accounts) != 0 {
		t.Errorf("expected no account to be created, got %d", len(as.accounts))
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no identity to be linked, got: %+v", als.addCalls)
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

// TestProcessOAuthLinkMinecraftJavaProfileFailureIsSurfaced mirrors
// TestProcessOAuthLoginMinecraftJavaProfileFailureIsSurfaced for the link
// path.
func TestProcessOAuthLinkMinecraftJavaProfileFailureIsSurfaced(t *testing.T) {
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
	if err == nil {
		t.Fatal("expected the link to fail when the Java-ownership check itself fails")
	}
	if got != nil {
		t.Errorf("expected no session returned, got: %+v", got)
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no identity to be linked, got: %+v", als.addCalls)
	}
}

// -------------- ProcessOAuthLogin/Link (Xbox Live only) end to end --------------

// TestProcessOAuthLoginXboxLiveOnlyNeverLinksJavaEvenIfOwned is the core
// regression test for wanting distinct Xbox Live vs Java linking: the
// account in this test DOES own Java (newFullChainServer(t, true)), but a
// caller that explicitly asked for auth.PlatformXboxLive must still end up
// with only the Xbox Live identity linked - proving Java is skipped by
// request, not just by accident of ownership.
func TestProcessOAuthLoginXboxLiveOnlyNeverLinksJavaEvenIfOwned(t *testing.T) {
	newFullChainServer(t, true)
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newXboxLiveOAuthState())
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session == nil {
		t.Fatal("expected a session")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive {
		t.Fatalf("expected only the Xbox Live identity to be linked even though Java is owned, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLinkXboxLiveOnlyNeverLinksJavaEvenIfOwned(t *testing.T) {
	newFullChainServer(t, true)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	state := newXboxLiveOAuthState()
	state.Mode = ModeLink

	got, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err != nil {
		t.Fatalf("ProcessOAuthLink returned error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive {
		t.Fatalf("expected only the Xbox Live identity to be linked even though Java is owned, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLinkXboxLiveOnlyAlreadyLinkedToDifferentAccountRejected(t *testing.T) {
	newFullChainServer(t, false)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}
	state := newXboxLiveOAuthState()
	state.Mode = ModeLink

	_, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err == nil {
		t.Fatal("expected an error when the Xbox Live identity is already linked to a different account")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no links to be attempted, got: %+v", als.addCalls)
	}
}

// -------------- ProcessOAuthLogin/Link (plain Microsoft) end to end --------------

func TestProcessOAuthLoginMicrosoftCreatesAccount(t *testing.T) {
	newMicrosoftLoginServer(t, "ms-oid-1", "Jane Doe", "jane@example.com")
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newMicrosoftOAuthState())
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session == nil || ss.addedOnce != session {
		t.Fatal("expected the new session to be added via SessionService")
	}
	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformMicrosoft || als.addCalls[0].PlatformID != "ms-oid-1" {
		t.Fatalf("expected the Microsoft identity to be linked, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLoginMicrosoftReusesExistingLinkedAccount(t *testing.T) {
	newMicrosoftLoginServer(t, "ms-oid-1", "Jane Doe", "jane@example.com")
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: true}, nil
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newMicrosoftOAuthState())
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session.UserID != "existing-acct" {
		t.Errorf("expected the session to belong to the existing account, got %q", session.UserID)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d", len(as.accounts))
	}
}

func TestProcessOAuthLinkMicrosoftLinksToSession(t *testing.T) {
	newMicrosoftLoginServer(t, "ms-oid-1", "Jane Doe", "jane@example.com")
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	state := newMicrosoftOAuthState()
	state.Mode = ModeLink

	got, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err != nil {
		t.Fatalf("ProcessOAuthLink returned error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformMicrosoft || als.addCalls[0].UserID != "u1" {
		t.Fatalf("expected the Microsoft identity to be linked to the session's account, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLinkMicrosoftAlreadyLinkedToDifferentAccountRejected(t *testing.T) {
	newMicrosoftLoginServer(t, "ms-oid-1", "Jane Doe", "jane@example.com")
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}
	state := newMicrosoftOAuthState()
	state.Mode = ModeLink

	_, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", state)
	if err == nil {
		t.Fatal("expected an error when the Microsoft identity is already linked to a different account")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no links to be attempted, got: %+v", als.addCalls)
	}
}
