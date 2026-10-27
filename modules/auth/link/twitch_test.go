package link

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func newTwitchConfig(t *testing.T, handler http.HandlerFunc) config.Twitch {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return testutil.NewConfig(func(c *config.Config) {
		c.Twitch.ClientId = "test-client-id"
		c.Twitch.APIBaseUrl = server.URL
	}).Twitch
}

func TestGetUserSuccess(t *testing.T) {
	cfg := newTwitchConfig(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			t.Errorf("expected a request to /users, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("expected the user access token to be sent as a bearer token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "12345", "login": "someuser", "email": "someuser@example.com"}]}`))
	})

	data, err := GetTwitchUser(context.Background(), cfg, &OAuthToken{AccessToken: "test-access-token"})
	if err != nil {
		t.Fatalf("GetTwitchUser returned error: %v", err)
	}
	if data.GetID() != "12345" || data.GetUsername() != "someuser" || data.GetEmail() != "someuser@example.com" {
		t.Errorf("unexpected user data: %+v", data.User)
	}
}

func TestGetUserNoUsersReturned(t *testing.T) {
	cfg := newTwitchConfig(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": []}`))
	})

	if _, err := GetTwitchUser(context.Background(), cfg, &OAuthToken{AccessToken: "test-access-token"}); err == nil {
		t.Fatal("expected an error when Twitch returns no user data")
	}
}

func TestGetUserAPIError(t *testing.T) {
	cfg := newTwitchConfig(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Unauthorized", "status": 401, "message": "Invalid OAuth token"}`))
	})

	if _, err := GetTwitchUser(context.Background(), cfg, &OAuthToken{AccessToken: "bad-token"}); err == nil {
		t.Fatal("expected an error for a non-2xx response from Twitch")
	}
}
