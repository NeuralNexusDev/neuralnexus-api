package linking

import (
	"errors"
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
		"openid.signed":     {"op_endpoint,claimed_id,identity,return_to"},
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

	_, err := VerifySteamOpenIDCallback(query)
	if err == nil {
		t.Fatal("expected an error when openid.mode is not id_res")
	}
	if !errors.Is(err, ErrInvalidAssertion) {
		t.Errorf("expected ErrInvalidAssertion (a client-fault, 400-worthy rejection), got: %v", err)
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

			_, err := VerifySteamOpenIDCallback(query)
			if err == nil {
				t.Fatalf("expected an error for claimed_id %q", claimedID)
			}
			if !errors.Is(err, ErrInvalidAssertion) {
				t.Errorf("expected ErrInvalidAssertion for claimed_id %q, got: %v", claimedID, err)
			}
		})
	}
}

func TestVerifySteamOpenIDCallbackSignedDoesNotCoverClaimedIDRejected(t *testing.T) {
	withServer(t, &steamOpenIDLoginURL, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"))
	})

	query := newValidSteamCallbackQuery()
	query.Set("openid.signed", "op_endpoint,identity,return_to")

	_, err := VerifySteamOpenIDCallback(query)
	if err == nil {
		t.Fatal("expected an error when openid.signed does not list claimed_id, even if Steam reports is_valid:true")
	}
	if !errors.Is(err, ErrInvalidAssertion) {
		t.Errorf("expected ErrInvalidAssertion, got: %v", err)
	}
}

func TestVerifySteamOpenIDCallbackRejectedByServer(t *testing.T) {
	withServer(t, &steamOpenIDLoginURL, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:false\n"))
	})

	err := requireVerifyError(t, newValidSteamCallbackQuery())
	if !errors.Is(err, ErrInvalidAssertion) {
		t.Errorf("expected ErrInvalidAssertion when Steam reports is_valid:false, got: %v", err)
	}
}

// TestVerifySteamOpenIDCallbackNonOKStatus is the regression test for the
// error-category split: a Steam-side outage/error must NOT be
// ErrInvalidAssertion, so callers map it to a 5xx (their fault, not the
// caller's), unlike a genuinely rejected assertion.
func TestVerifySteamOpenIDCallbackNonOKStatus(t *testing.T) {
	withServer(t, &steamOpenIDLoginURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := requireVerifyError(t, newValidSteamCallbackQuery())
	if errors.Is(err, ErrInvalidAssertion) {
		t.Errorf("expected a non-2xx response from Steam to NOT be ErrInvalidAssertion, got: %v", err)
	}
}

// TestVerifySteamOpenIDCallbackNetworkErrorIsNotInvalidAssertion covers the
// other outage shape - Steam unreachable entirely, rather than reachable but
// erroring - which must also fall outside ErrInvalidAssertion.
func TestVerifySteamOpenIDCallbackNetworkErrorIsNotInvalidAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close() // closed immediately, so the client's request will fail to connect

	original := steamOpenIDLoginURL
	steamOpenIDLoginURL = unreachableURL
	t.Cleanup(func() { steamOpenIDLoginURL = original })

	err := requireVerifyError(t, newValidSteamCallbackQuery())
	if errors.Is(err, ErrInvalidAssertion) {
		t.Errorf("expected a network failure reaching Steam to NOT be ErrInvalidAssertion, got: %v", err)
	}
}

// requireVerifyError calls VerifySteamOpenIDCallback and fails the test if
// it doesn't return an error, returning that error for further assertions.
func requireVerifyError(t *testing.T, query url.Values) error {
	t.Helper()
	_, err := VerifySteamOpenIDCallback(query)
	if err == nil {
		t.Fatal("expected VerifySteamOpenIDCallback to return an error")
	}
	return err
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

func TestGetSteamUserMismatchedSteamIDRejected(t *testing.T) {
	withServer(t, &steamPlayerSummaryURL, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"response":{"players":[{"steamid":"99999999999999999","personaname":"WrongPlayer"}]}}`))
	})
	t.Cleanup(setSteamAPIKey(t, "test-key"))

	if _, err := GetSteamUser("76561198000000000"); err == nil {
		t.Fatal("expected an error when the returned player's steamid doesn't match the requested one")
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
