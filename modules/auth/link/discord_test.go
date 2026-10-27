package link

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/goccy/go-json"
)

func TestDC04DiscordDataGetData(t *testing.T) {
	d := &DiscordData{User: &discordgo.User{
		ID:       "123",
		Username: "alice",
		Email:    "a@b.com",
	}}

	t.Run("DC-04_GetData", func(t *testing.T) {
		data := d.GetData()
		var roundTripped discordgo.User
		if err := json.Unmarshal([]byte(data), &roundTripped); err != nil {
			t.Fatalf("GetData() did not round-trip as JSON: %v", err)
		}
		if roundTripped.ID != d.ID || roundTripped.Username != d.Username {
			t.Errorf("GetData() round-trip mismatch: got %+v, want equivalent of %+v", roundTripped, *d.User)
		}
	})
}

// setDiscordUsersEndpoint points discordgo's EndpointUsers at ts for the
// duration of the calling test, restoring it on cleanup. discordgo reads the
// var live rather than caching it, so reassigning it here redirects
// GetDiscordUser's request with no source change.
func setDiscordUsersEndpoint(t *testing.T, ts *httptest.Server) {
	t.Helper()
	original := discordgo.EndpointUsers
	discordgo.EndpointUsers = ts.URL + "/users/"
	t.Cleanup(func() {
		discordgo.EndpointUsers = original
	})
}

func TestDC06to08GetDiscordUser(t *testing.T) {
	t.Run("DC-06_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/users/@me" {
				t.Errorf("unexpected request path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"123","username":"alice","email":"a@b.com"}`))
		}))
		defer ts.Close()
		setDiscordUsersEndpoint(t, ts)

		token := &OAuthToken{AccessToken: "tok"}
		got, err := GetDiscordUser(token)
		if err != nil {
			t.Fatalf("GetDiscordUser() unexpected error: %v", err)
		}
		if got.GetID() != "123" || got.GetUsername() != "alice" || got.GetEmail() != "a@b.com" {
			t.Errorf("GetDiscordUser() = %+v, want id=123 username=alice email=a@b.com", got.User)
		}
	})

	t.Run("DC-07_APIError", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"401: Unauthorized","code":0}`))
		}))
		defer ts.Close()
		setDiscordUsersEndpoint(t, ts)

		token := &OAuthToken{AccessToken: "bad-tok"}
		got, err := GetDiscordUser(token)
		if err == nil {
			t.Fatalf("GetDiscordUser() expected an error, got none (result: %+v)", got)
		}
		if got != nil {
			t.Errorf("GetDiscordUser() expected nil result on error, got %+v", got)
		}
	})

	t.Run("DC-08_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		setDiscordUsersEndpoint(t, ts)

		token := &OAuthToken{AccessToken: "tok"}
		got, err := GetDiscordUser(token)
		if err == nil {
			t.Fatalf("GetDiscordUser() expected an error for malformed JSON, got none (result: %+v)", got)
		}
		if got != nil {
			t.Errorf("GetDiscordUser() expected nil result on error, got %+v", got)
		}
	})
}
