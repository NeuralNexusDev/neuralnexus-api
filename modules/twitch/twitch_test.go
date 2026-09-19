package twitch

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// withAPIBaseURL points APIBaseURL at a test server for the duration of the
// test, restoring the original value afterward. It also fills in CLIENT_ID
// if it's empty (e.g. TWITCH_CLIENT_ID isn't set in the test environment) -
// helix.NewClient refuses to construct a client without one, regardless of
// APIBaseURL.
func withAPIBaseURL(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	original := APIBaseURL
	APIBaseURL = server.URL
	t.Cleanup(func() { APIBaseURL = original })

	if CLIENT_ID == "" {
		CLIENT_ID = "test-client-id"
		t.Cleanup(func() { CLIENT_ID = "" })
	}
	return server
}

func TestGetUserSuccess(t *testing.T) {
	withAPIBaseURL(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			t.Errorf("expected a request to /users, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("expected the user access token to be sent as a bearer token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "12345", "login": "someuser", "email": "someuser@example.com"}]}`))
	})

	data, err := GetUser(&auth.OAuthToken{AccessToken: "test-access-token"})
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if data.GetID() != "12345" || data.GetUsername() != "someuser" || data.GetEmail() != "someuser@example.com" {
		t.Errorf("unexpected user data: %+v", data.User)
	}
}

func TestGetUserNoUsersReturned(t *testing.T) {
	withAPIBaseURL(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": []}`))
	})

	if _, err := GetUser(&auth.OAuthToken{AccessToken: "test-access-token"}); err == nil {
		t.Fatal("expected an error when Twitch returns no user data")
	}
}

func TestGetUserAPIError(t *testing.T) {
	withAPIBaseURL(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Unauthorized", "status": 401, "message": "Invalid OAuth token"}`))
	})

	if _, err := GetUser(&auth.OAuthToken{AccessToken: "bad-token"}); err == nil {
		t.Fatal("expected an error for a non-2xx response from Twitch")
	}
}
