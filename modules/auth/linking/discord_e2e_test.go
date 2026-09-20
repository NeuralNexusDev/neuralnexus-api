package linking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/bwmarrin/discordgo"
)

// newDiscordOAuthServer serves both the Discord OAuth2 token endpoint and
// the /users/@me endpoint behind one test server, so ProcessOAuthLogin/
// ProcessOAuthLink can be driven end to end from an authorization code
// without hitting the real Discord API.
func newDiscordOAuthServer(t *testing.T) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/discord-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "discord-access-token",
			"token_type":   "Bearer",
			"expires_in":   604800,
			"scope":        "identify email",
		})
	})
	mux.HandleFunc("/users/@me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "9999", "username": "discorduser", "email": "discorduser@example.com"}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	originalTokenURL := discordConfig.Endpoint.TokenURL
	t.Cleanup(func() { discordConfig.Endpoint.TokenURL = originalTokenURL })
	discordConfig.Endpoint.TokenURL = server.URL + "/discord-token"

	originalUsersEndpoint := discordgo.EndpointUsers
	t.Cleanup(func() { discordgo.EndpointUsers = originalUsersEndpoint })
	discordgo.EndpointUsers = server.URL + "/users/"
}

func newDiscordOAuthState(mode Mode) *OAuthState {
	return &OAuthState{Platform: auth.PlatformDiscord, Nonce: "n", RedirectURI: "https://example.com/done", Mode: mode}
}

func TestProcessOAuthLoginDiscordCreatesAccount(t *testing.T) {
	newDiscordOAuthServer(t)
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newDiscordOAuthState(ModeLogin))
	if err != nil {
		t.Fatalf("ProcessOAuthLogin returned error: %v", err)
	}
	if session == nil || ss.addedOnce != session {
		t.Fatal("expected the new session to be added via SessionService")
	}
	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformDiscord || als.addCalls[0].PlatformID != "9999" {
		t.Fatalf("expected the Discord identity to be linked, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLoginDiscordReusesExistingLinkedAccount(t *testing.T) {
	newDiscordOAuthServer(t)
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: true}, nil
		},
	}
	ss := &mockSessionService{}

	session, err := ProcessOAuthLogin(as, als, ss, "some-code", newDiscordOAuthState(ModeLogin))
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

func TestProcessOAuthLinkDiscordLinksToSession(t *testing.T) {
	newDiscordOAuthServer(t)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}

	got, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", newDiscordOAuthState(ModeLink))
	if err != nil {
		t.Fatalf("ProcessOAuthLink returned error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].UserID != "u1" || als.addCalls[0].Platform != auth.PlatformDiscord {
		t.Fatalf("expected the Discord identity to be linked to the session's account, got: %+v", als.addCalls)
	}
}

func TestProcessOAuthLinkDiscordAlreadyLinkedToDifferentAccountRejected(t *testing.T) {
	newDiscordOAuthServer(t)
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}

	_, err := ProcessOAuthLink(linkRequestWithSession(session), als, "some-code", newDiscordOAuthState(ModeLink))
	if err == nil {
		t.Fatal("expected an error when the Discord identity is already linked to a different account")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no link to be attempted, got: %+v", als.addCalls)
	}
}
