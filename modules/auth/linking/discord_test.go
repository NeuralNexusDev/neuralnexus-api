package linking

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/bwmarrin/discordgo"
)

// withDiscordUsersEndpoint points discordgo's global EndpointUsers var at a
// test server for the duration of the test. This is discordgo's own package
// state, not ours - unlike MicrosoftConfig/twitch.APIBaseURL, the library
// gives no per-call way to redirect requests, so this is the only lever
// available. Safe here since these tests never run with t.Parallel() and
// discordgo is used nowhere else in this codebase.
func withDiscordUsersEndpoint(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	original := discordgo.EndpointUsers
	discordgo.EndpointUsers = server.URL + "/users/"
	t.Cleanup(func() { discordgo.EndpointUsers = original })
	return server
}

func TestGetDiscordUserSuccess(t *testing.T) {
	withDiscordUsersEndpoint(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/@me" {
			t.Errorf("expected a request to /users/@me, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("expected the access token to be sent as a bearer token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "9999", "username": "someuser", "email": "someuser@example.com"}`))
	})

	data, err := GetDiscordUser(&auth.OAuthToken{AccessToken: "test-access-token"})
	if err != nil {
		t.Fatalf("GetDiscordUser returned error: %v", err)
	}
	if data.GetID() != "9999" || data.GetUsername() != "someuser" || data.GetEmail() != "someuser@example.com" {
		t.Errorf("unexpected user data: %+v", data.User)
	}
}

func TestGetDiscordUserAPIError(t *testing.T) {
	withDiscordUsersEndpoint(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "401: Unauthorized", "code": 0}`))
	})

	if _, err := GetDiscordUser(&auth.OAuthToken{AccessToken: "bad-token"}); err == nil {
		t.Fatal("expected an error for a non-2xx response from Discord")
	}
}
