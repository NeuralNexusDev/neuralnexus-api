package linking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"
)

// newTwitchOAuthServer serves both the Twitch OAuth2 token endpoint and the
// /users endpoint behind one test server, so ProcessOAuthLogin/
// ProcessOAuthLink can be driven end to end from an authorization code
// without hitting the real Twitch API.
func newTwitchOAuthServer(t *testing.T) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/twitch-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "twitch-access-token",
			"token_type":   "bearer",
			"expires_in":   14400,
			"scope":        []string{"user:read:email"},
		})
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "8888", "login": "twitchuser", "email": "twitchuser@example.com"}]}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	originalTokenURL := twitch.Config.Endpoint.TokenURL
	t.Cleanup(func() { twitch.Config.Endpoint.TokenURL = originalTokenURL })
	twitch.Config.Endpoint.TokenURL = server.URL + "/twitch-token"

	originalAPIBaseURL := twitch.APIBaseURL
	t.Cleanup(func() { twitch.APIBaseURL = originalAPIBaseURL })
	twitch.APIBaseURL = server.URL

	if twitch.CLIENT_ID == "" {
		twitch.CLIENT_ID = "test-client-id"
		t.Cleanup(func() { twitch.CLIENT_ID = "" })
	}
}

func newTwitchOAuthState(mode Mode) *OAuthState {
	return &OAuthState{Platform: auth.PlatformTwitch, Nonce: "n", RedirectURI: "https://example.com/done", Mode: mode}
}

func TestProcessOAuthLoginTwitchCreatesAccount(t *testing.T) {
	newTwitchOAuthServer(t)
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newTwitchOAuthState(ModeLogin))
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session == nil || ss.addedOnce != session {
		t.Fatal("expected the new session to be added via SessionService")
	}
	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformTwitch || als.addCalls[0].PlatformID != "8888" {
		t.Fatalf("expected the Twitch identity to be linked, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLoginTwitchReusesExistingLinkedAccount(t *testing.T) {
	newTwitchOAuthServer(t)
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: true}, nil
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newTwitchOAuthState(ModeLogin))
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

func TestProcessOAuthLinkTwitchLinksToSession(t *testing.T) {
	newTwitchOAuthServer(t)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}

	got, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", newTwitchOAuthState(ModeLink))
	if err != nil {
		t.Fatalf("ProcessOAuthLink returned error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].UserID != "u1" || als.addCalls[0].Platform != auth.PlatformTwitch {
		t.Fatalf("expected the Twitch identity to be linked to the session's account, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLinkTwitchAlreadyLinkedToDifferentAccountRejected(t *testing.T) {
	newTwitchOAuthServer(t)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}

	_, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", newTwitchOAuthState(ModeLink))
	if err == nil {
		t.Fatal("expected an error when the Twitch identity is already linked to a different account")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no link to be attempted, got: %+v", als.addCalls)
	}
}
