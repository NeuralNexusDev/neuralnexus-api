package linking

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// -------------- SteamData --------------

func TestSteamDataImplementsPlatformData(t *testing.T) {
	s := &SteamData{SteamID64: "1234", PersonaName: "TestPlayer", ProfileURL: "https://steamcommunity.com/id/testplayer"}

	if s.GetID() != "1234" {
		t.Errorf("expected GetID() to return the SteamID64, got %q", s.GetID())
	}
	if s.GetUsername() != "TestPlayer" {
		t.Errorf("expected GetUsername() to return the persona name, got %q", s.GetUsername())
	}
	if s.GetEmail() != "" {
		t.Errorf("expected GetEmail() to always be empty, got %q", s.GetEmail())
	}

	la := s.CreateLinkedAccount("user-1")
	if la.Platform != auth.PlatformSteam || la.PlatformID != "1234" || la.PlatformUsername != "TestPlayer" {
		t.Errorf("unexpected linked account: %+v", la)
	}
}

// -------------- VerifySteamOpenIDCallback --------------

func newValidSteamCallbackQuery() url.Values {
	return url.Values{
		"openid.mode":       {"id_res"},
		"openid.claimed_id": {"https://steamcommunity.com/openid/id/76561198000000000"},
		"openid.sig":        {"fake-sig"},
		"openid.signed":     {"signed,fields"},
	}
}

func TestVerifySteamOpenIDCallbackSuccess(t *testing.T) {
	withServer(t, &steamOpenIDLoginURL, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse check_authentication request body: %v", err)
		}
		if got := r.PostForm.Get("openid.mode"); got != "check_authentication" {
			t.Errorf("expected openid.mode=check_authentication in the verification request, got %q", got)
		}
		if got := r.PostForm.Get("openid.sig"); got != "fake-sig" {
			t.Errorf("expected the original assertion's fields to be echoed back, got openid.sig=%q", got)
		}
		w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"))
	})

	steamID64, err := VerifySteamOpenIDCallback(newValidSteamCallbackQuery())
	if err != nil {
		t.Fatalf("VerifySteamOpenIDCallback returned error: %v", err)
	}
	if steamID64 != "76561198000000000" {
		t.Errorf("expected the extracted SteamID64, got %q", steamID64)
	}
}

func TestVerifySteamOpenIDCallbackWrongMode(t *testing.T) {
	query := newValidSteamCallbackQuery()
	query.Set("openid.mode", "cancel")

	if _, err := VerifySteamOpenIDCallback(query); err == nil {
		t.Fatal("expected an error when openid.mode is not id_res")
	}
}

func TestVerifySteamOpenIDCallbackInvalidClaimedID(t *testing.T) {
	tests := []string{
		"",
		"not-a-url",
		"https://evil.example.com/openid/id/76561198000000000",
		"https://steamcommunity.com/openid/id/not-numeric",
	}
	for _, claimedID := range tests {
		t.Run(claimedID, func(t *testing.T) {
			query := newValidSteamCallbackQuery()
			query.Set("openid.claimed_id", claimedID)

			if _, err := VerifySteamOpenIDCallback(query); err == nil {
				t.Errorf("expected an error for claimed_id %q", claimedID)
			}
		})
	}
}

func TestVerifySteamOpenIDCallbackRejectedByServer(t *testing.T) {
	withServer(t, &steamOpenIDLoginURL, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:false\n"))
	})

	if _, err := VerifySteamOpenIDCallback(newValidSteamCallbackQuery()); err == nil {
		t.Fatal("expected an error when Steam reports is_valid:false")
	}
}

func TestVerifySteamOpenIDCallbackNonOKStatus(t *testing.T) {
	withServer(t, &steamOpenIDLoginURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := VerifySteamOpenIDCallback(newValidSteamCallbackQuery()); err == nil {
		t.Fatal("expected an error for a non-2xx response from Steam")
	}
}

// -------------- GetSteamUser --------------

func TestGetSteamUserSuccess(t *testing.T) {
	withServer(t, &steamPlayerSummaryURL, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("steamids"); got != "76561198000000000" {
			t.Errorf("expected steamids=76561198000000000, got %q", got)
		}
		w.Write([]byte(`{"response":{"players":[{"steamid":"76561198000000000","personaname":"TestPlayer","profileurl":"https://steamcommunity.com/id/testplayer","avatarfull":"https://avatar.example/full.jpg"}]}}`))
	})
	t.Cleanup(setSteamAPIKey(t, "test-key"))

	user, err := GetSteamUser("76561198000000000")
	if err != nil {
		t.Fatalf("GetSteamUser returned error: %v", err)
	}
	if user.GetID() != "76561198000000000" || user.GetUsername() != "TestPlayer" {
		t.Errorf("unexpected user: %+v", user)
	}
}

func TestGetSteamUserMissingAPIKey(t *testing.T) {
	t.Cleanup(setSteamAPIKey(t, ""))

	if _, err := GetSteamUser("76561198000000000"); err == nil {
		t.Fatal("expected an error when STEAM_API_KEY is not set")
	}
}

func TestGetSteamUserNonOKStatus(t *testing.T) {
	withServer(t, &steamPlayerSummaryURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	t.Cleanup(setSteamAPIKey(t, "test-key"))

	if _, err := GetSteamUser("76561198000000000"); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestGetSteamUserNoPlayersReturned(t *testing.T) {
	withServer(t, &steamPlayerSummaryURL, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"response":{"players":[]}}`))
	})
	t.Cleanup(setSteamAPIKey(t, "test-key"))

	if _, err := GetSteamUser("76561198000000000"); err == nil {
		t.Fatal("expected an error when Steam returns no players for the given SteamID64")
	}
}

// setSteamAPIKey overrides STEAM_API_KEY for the duration of a test,
// returning a func to restore the original value.
func setSteamAPIKey(t *testing.T, value string) func() {
	t.Helper()
	original := STEAM_API_KEY
	STEAM_API_KEY = value
	return func() { STEAM_API_KEY = original }
}

// -------------- ProcessSteamLogin / ProcessSteamLink --------------

func TestProcessSteamLoginCreatesAccount(t *testing.T) {
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	ss := &mockSessionService{}
	user := &SteamData{SteamID64: "76561198000000000", PersonaName: "TestPlayer"}

	session, err := ProcessSteamLogin(as, als, ss, user)
	if err != nil {
		t.Fatalf("ProcessSteamLogin returned error: %v", err)
	}
	if session == nil || ss.addedOnce != session {
		t.Fatal("expected the new session to be added via SessionService")
	}
	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformSteam || als.addCalls[0].PlatformID != "76561198000000000" {
		t.Fatalf("expected the Steam identity to be linked, got: %+v", als.addCalls)
	}
}

func TestProcessSteamLoginReusesExistingLinkedAccount(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: true}, nil
		},
	}
	ss := &mockSessionService{}
	user := &SteamData{SteamID64: "76561198000000000", PersonaName: "TestPlayer"}

	session, err := ProcessSteamLogin(as, als, ss, user)
	if err != nil {
		t.Fatalf("ProcessSteamLogin returned error: %v", err)
	}
	if session.UserID != "existing-acct" {
		t.Errorf("expected the session to belong to the existing account, got %q", session.UserID)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d accounts", len(as.accounts))
	}
}

func TestProcessSteamLinkLinksToSession(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	user := &SteamData{SteamID64: "76561198000000000", PersonaName: "TestPlayer"}

	got, err := ProcessSteamLink(linkRequestWithSession(session), als, user)
	if err != nil {
		t.Fatalf("ProcessSteamLink returned error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 1 || als.addCalls[0].UserID != "u1" || als.addCalls[0].Platform != auth.PlatformSteam {
		t.Fatalf("expected the Steam identity to be linked to the session's account, got: %+v", als.addCalls)
	}
}

func TestProcessSteamLinkAlreadyLinkedToDifferentAccountRejected(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}
	user := &SteamData{SteamID64: "76561198000000000", PersonaName: "TestPlayer"}

	_, err := ProcessSteamLink(linkRequestWithSession(session), als, user)
	if err == nil {
		t.Fatal("expected an error when the Steam identity is already linked to a different account")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no link to be attempted, got: %+v", als.addCalls)
	}
}

func TestProcessSteamLinkNoSessionRejected(t *testing.T) {
	als := &mockLinkAccountStore{}
	user := &SteamData{SteamID64: "76561198000000000", PersonaName: "TestPlayer"}

	r := httptest.NewRequest(http.MethodGet, "/api/openid", nil)
	if _, err := ProcessSteamLink(r, als, user); err == nil {
		t.Fatal("expected an error when there's no session in the request context")
	}
}
