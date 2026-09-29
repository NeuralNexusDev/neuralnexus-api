package linking

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
)

type stMockAccountService struct {
	accounts        map[string]*auth.Account
	addAccountErr   error
	getByIDErr      error
	deleteErr       error
	deleteCalls     int
	addAccountCalls int
}

var _ auth.AccountService = (*stMockAccountService)(nil)

func newSTMockAccountService() *stMockAccountService {
	return &stMockAccountService{accounts: map[string]*auth.Account{}}
}

func (m *stMockAccountService) AddAccount(a *auth.Account) error {
	m.addAccountCalls++
	if m.addAccountErr != nil {
		return m.addAccountErr
	}
	m.accounts[a.UserID] = a
	return nil
}
func (m *stMockAccountService) GetAccountByID(userID string) (*auth.Account, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	a, ok := m.accounts[userID]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return a, nil
}
func (m *stMockAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *stMockAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *stMockAccountService) UpdateAccount(a *auth.Account) error {
	m.accounts[a.UserID] = a
	return nil
}
func (m *stMockAccountService) DeleteAccount(userID string) error {
	m.deleteCalls++
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.accounts, userID)
	return nil
}
func (m *stMockAccountService) IsPasswordAuthEnabled(string) (bool, error) { return false, nil }

type stMockLinkAccountStore struct {
	byPlatformID map[auth.Platform]map[string]*auth.LinkedAccount
	getErr       error
	addErr       error
	addCalls     int
}

var _ auth.LinkAccountStore = (*stMockLinkAccountStore)(nil)

func newSTMockLinkAccountStore() *stMockLinkAccountStore {
	return &stMockLinkAccountStore{byPlatformID: map[auth.Platform]map[string]*auth.LinkedAccount{}}
}

func (m *stMockLinkAccountStore) AddLinkedAccountToDB(la *auth.LinkedAccount) error {
	m.addCalls++
	if m.addErr != nil {
		return m.addErr
	}
	if m.byPlatformID[la.Platform] == nil {
		m.byPlatformID[la.Platform] = map[string]*auth.LinkedAccount{}
	}
	m.byPlatformID[la.Platform][la.PlatformID] = la
	return nil
}
func (m *stMockLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error { return nil }
func (m *stMockLinkAccountStore) GetLinkedAccountByPlatformID(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	la, ok := m.byPlatformID[platform][platformID]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return la, nil
}
func (m *stMockLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *stMockLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}
func (m *stMockLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*auth.LinkedAccount, error) {
	return nil, nil
}
func (m *stMockLinkAccountStore) DeleteLinkedAccount(string, auth.Platform) error { return nil }
func (m *stMockLinkAccountStore) SetLinkedAccountLoginEnabled(string, auth.Platform, bool) error {
	return nil
}

type stMockSessionService struct {
	addSessionErr error
	added         []*auth.Session
}

var _ auth.SessionService = (*stMockSessionService)(nil)

func (m *stMockSessionService) AddSession(session *auth.Session) error {
	if m.addSessionErr != nil {
		return m.addSessionErr
	}
	m.added = append(m.added, session)
	return nil
}
func (m *stMockSessionService) GetSession(string) (*auth.Session, error) {
	return nil, auth.ErrNotFound
}
func (m *stMockSessionService) UpdateSession(*auth.Session) error       { return nil }
func (m *stMockSessionService) DeleteSession(string) error              { return nil }
func (m *stMockSessionService) CreateJWT(*auth.Session) (string, error) { return "", nil }
func (m *stMockSessionService) ReadJWT(string) (*auth.Session, error)   { return nil, auth.ErrNotFound }

func stRequestWithSession(session *auth.Session) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if session == nil {
		return req
	}
	ctx := context.WithValue(req.Context(), mw.SessionKey, session)
	return req.WithContext(ctx)
}

func TestST01to04SteamDataAccessors(t *testing.T) {
	s := &SteamData{SteamID64: "76500000000000001", PersonaName: "bob", ProfileURL: "https://steamcommunity.com/id/bob", AvatarURL: "https://avatar.test/bob.png"}

	t.Run("ST-01_GetID", func(t *testing.T) {
		if got := s.GetID(); got != "76500000000000001" {
			t.Errorf("GetID() = %q, want %q", got, "76500000000000001")
		}
	})
	t.Run("ST-02_GetUsername", func(t *testing.T) {
		if got := s.GetUsername(); got != "bob" {
			t.Errorf("GetUsername() = %q, want %q", got, "bob")
		}
	})
	t.Run("ST-03_GetEmail", func(t *testing.T) {
		if got := s.GetEmail(); got != "" {
			t.Errorf("GetEmail() = %q, want empty string", got)
		}
	})
	t.Run("ST-04_GetData", func(t *testing.T) {
		data := s.GetData()
		var roundTripped SteamData
		if err := json.Unmarshal([]byte(data), &roundTripped); err != nil {
			t.Fatalf("GetData() did not round-trip as JSON: %v", err)
		}
		if roundTripped != *s {
			t.Errorf("GetData() round-trip mismatch: got %+v, want %+v", roundTripped, *s)
		}
	})
}

func TestST05CreateLinkedAccount(t *testing.T) {
	t.Run("ST-05_CreateLinkedAccount", func(t *testing.T) {
		s := &SteamData{SteamID64: "76500000000000001", PersonaName: "bob"}
		la := s.CreateLinkedAccount("user-1")

		if la.UserID != "user-1" || la.Platform != auth.PlatformSteam || la.PlatformUsername != "bob" || la.PlatformID != "76500000000000001" {
			t.Errorf("CreateLinkedAccount() = %+v, want UserID=user-1 Platform=steam PlatformUsername=bob PlatformID=76500000000000001", la)
		}
		if !la.Verified || !la.LoginEnabled {
			t.Errorf("expected Verified and LoginEnabled true, got Verified=%v LoginEnabled=%v", la.Verified, la.LoginEnabled)
		}
	})
}

func validSteamQuery(claimedID string) url.Values {
	q := url.Values{}
	q.Set("openid.mode", "id_res")
	q.Set("openid.claimed_id", "https://steamcommunity.com/openid/id/"+claimedID)
	q.Set("openid.identity", "https://steamcommunity.com/openid/id/"+claimedID)
	q.Set("openid.signed", "op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle")
	q.Set("openid.sig", "deadbeef")
	q.Set("openid.ns", "http://specs.openid.net/auth/2.0")
	return q
}

func TestST06to12VerifySteamOpenIDCallback(t *testing.T) {
	t.Run("ST-06_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("failed to parse check_authentication body: %v", err)
			}
			if got := r.PostForm.Get("openid.mode"); got != "check_authentication" {
				t.Errorf("openid.mode sent to Steam = %q, want check_authentication", got)
			}
			_, _ = w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n"))
		}))
		defer ts.Close()
		restore := setSteamURLs(ts.URL, "")
		defer restore()

		got, err := VerifySteamOpenIDCallback(validSteamQuery("76500000000000001"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "76500000000000001" {
			t.Errorf("VerifySteamOpenIDCallback() = %q, want %q", got, "76500000000000001")
		}
	})

	t.Run("ST-07_WrongMode", func(t *testing.T) {
		q := validSteamQuery("76500000000000001")
		q.Set("openid.mode", "cancel")
		_, err := VerifySteamOpenIDCallback(q)
		if !errors.Is(err, ErrInvalidAssertion) {
			t.Fatalf("expected ErrInvalidAssertion, got %v", err)
		}
	})

	t.Run("ST-08_InvalidClaimedID", func(t *testing.T) {
		q := validSteamQuery("76500000000000001")
		q.Set("openid.claimed_id", "not-a-steam-url")
		_, err := VerifySteamOpenIDCallback(q)
		if !errors.Is(err, ErrInvalidAssertion) {
			t.Fatalf("expected ErrInvalidAssertion, got %v", err)
		}
	})

	t.Run("ST-09_SignedDoesNotCoverClaimedID", func(t *testing.T) {
		q := validSteamQuery("76500000000000001")
		q.Set("openid.signed", "op_endpoint,identity,return_to")
		_, err := VerifySteamOpenIDCallback(q)
		if !errors.Is(err, ErrInvalidAssertion) {
			t.Fatalf("expected ErrInvalidAssertion, got %v", err)
		}
	})

	t.Run("ST-10_NetworkErrorIsNotInvalidAssertion", func(t *testing.T) {
		restore := setSteamURLs("http://127.0.0.1:1", "")
		defer restore()

		_, err := VerifySteamOpenIDCallback(validSteamQuery("76500000000000001"))
		if err == nil {
			t.Fatal("expected an error")
		}
		if errors.Is(err, ErrInvalidAssertion) {
			t.Errorf("network failure should not be classified as ErrInvalidAssertion, got %v", err)
		}
	})

	t.Run("ST-11_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		restore := setSteamURLs(ts.URL, "")
		defer restore()

		_, err := VerifySteamOpenIDCallback(validSteamQuery("76500000000000001"))
		if err == nil {
			t.Fatal("expected an error")
		}
		if errors.Is(err, ErrInvalidAssertion) {
			t.Errorf("non-OK status should not be classified as ErrInvalidAssertion, got %v", err)
		}
	})

	t.Run("ST-12_RejectedByServer", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("ns:http://specs.openid.net/auth/2.0\nis_valid:false\n"))
		}))
		defer ts.Close()
		restore := setSteamURLs(ts.URL, "")
		defer restore()

		_, err := VerifySteamOpenIDCallback(validSteamQuery("76500000000000001"))
		if !errors.Is(err, ErrInvalidAssertion) {
			t.Fatalf("expected ErrInvalidAssertion, got %v", err)
		}
	})
}

func setSteamURLs(openIDLoginURL, playerSummaryURL string) func() {
	origLogin, origSummary := steamOpenIDLoginURL, steamPlayerSummaryURL
	if openIDLoginURL != "" {
		steamOpenIDLoginURL = openIDLoginURL
	}
	if playerSummaryURL != "" {
		steamPlayerSummaryURL = playerSummaryURL
	}
	return func() {
		steamOpenIDLoginURL = origLogin
		steamPlayerSummaryURL = origSummary
	}
}

func TestST13to15ResponseIsValid(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"ST-13_IsValidTrue", "ns:foo\nis_valid:true\n", true},
		{"ST-14_IsValidFalseOrAbsent", "ns:foo\nis_valid:false\n", false},
		{"ST-15_EmptyBody", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := responseIsValid([]byte(tc.body)); got != tc.want {
				t.Errorf("responseIsValid(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestST16to22GetSteamUser(t *testing.T) {
	t.Run("ST-16_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"players":[{"steamid":"76500000000000001","personaname":"bob","profileurl":"https://steamcommunity.com/id/bob","avatarfull":"https://avatar.test/bob.png"}]}}`))
		}))
		defer ts.Close()
		restore := setSteamURLs("", ts.URL+"/")
		restoreKey := setSteamAPIKeyForTest(t, "test-key")
		defer restore()
		defer restoreKey()

		got, err := GetSteamUser("76500000000000001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.SteamID64 != "76500000000000001" || got.PersonaName != "bob" {
			t.Errorf("GetSteamUser() = %+v, want steamid=76500000000000001 personaname=bob", got)
		}
	})

	t.Run("ST-17_MissingAPIKey", func(t *testing.T) {
		restoreKey := setSteamAPIKeyForTest(t, "")
		defer restoreKey()

		_, err := GetSteamUser("76500000000000001")
		if !errors.Is(err, errSteamAPIKeyUnset) {
			t.Fatalf("GetSteamUser() error = %v, want %v", err, errSteamAPIKeyUnset)
		}
	})

	t.Run("ST-18_NetworkError", func(t *testing.T) {
		restore := setSteamURLs("", "http://127.0.0.1:1/")
		restoreKey := setSteamAPIKeyForTest(t, "test-key")
		defer restore()
		defer restoreKey()

		_, err := GetSteamUser("76500000000000001")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("ST-19_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		restore := setSteamURLs("", ts.URL+"/")
		restoreKey := setSteamAPIKeyForTest(t, "test-key")
		defer restore()
		defer restoreKey()

		_, err := GetSteamUser("76500000000000001")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("ST-20_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		restore := setSteamURLs("", ts.URL+"/")
		restoreKey := setSteamAPIKeyForTest(t, "test-key")
		defer restore()
		defer restoreKey()

		_, err := GetSteamUser("76500000000000001")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("ST-21_NoPlayersReturned", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"players":[]}}`))
		}))
		defer ts.Close()
		restore := setSteamURLs("", ts.URL+"/")
		restoreKey := setSteamAPIKeyForTest(t, "test-key")
		defer restore()
		defer restoreKey()

		_, err := GetSteamUser("76500000000000001")
		if !errors.Is(err, errSteamNoPlayers) {
			t.Fatalf("GetSteamUser() error = %v, want %v", err, errSteamNoPlayers)
		}
	})

	t.Run("ST-22_MismatchedSteamID", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"players":[{"steamid":"76500000000000099","personaname":"bob"}]}}`))
		}))
		defer ts.Close()
		restore := setSteamURLs("", ts.URL+"/")
		restoreKey := setSteamAPIKeyForTest(t, "test-key")
		defer restore()
		defer restoreKey()

		_, err := GetSteamUser("76500000000000001")
		if err == nil {
			t.Fatal("expected a steamid-mismatch error")
		}
	})
}

func setSteamAPIKeyForTest(t *testing.T, value string) func() {
	t.Helper()
	original := STEAM_API_KEY
	STEAM_API_KEY = value
	return func() {
		STEAM_API_KEY = original
	}
}

func TestST23to25ProcessSteamLogin(t *testing.T) {
	user := &SteamData{SteamID64: "76500000000000001", PersonaName: "bob"}

	t.Run("ST-23_CreatesAccount", func(t *testing.T) {
		as := newSTMockAccountService()
		las := newSTMockLinkAccountStore()
		ss := &stMockSessionService{}

		session, err := ProcessSteamLogin(as, las, ss, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session == nil {
			t.Fatal("expected a non-nil session")
		}
		if len(ss.added) != 1 {
			t.Errorf("ss.AddSession called %d times, want 1", len(ss.added))
		}
		if as.addAccountCalls != 1 {
			t.Errorf("as.AddAccount called %d times, want 1", as.addAccountCalls)
		}
	})

	t.Run("ST-24_AccountResolutionFails", func(t *testing.T) {
		as := newSTMockAccountService()
		as.addAccountErr = testerrors.ErrDBDown
		las := newSTMockLinkAccountStore()
		ss := &stMockSessionService{}

		_, err := ProcessSteamLogin(as, las, ss, user)
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("ST-25_AddSessionFails", func(t *testing.T) {
		as := newSTMockAccountService()
		las := newSTMockLinkAccountStore()
		ss := &stMockSessionService{addSessionErr: errSessionStoreDown}

		_, err := ProcessSteamLogin(as, las, ss, user)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestST26to29ProcessSteamLink(t *testing.T) {
	user := &SteamData{SteamID64: "76500000000000001", PersonaName: "bob"}

	t.Run("ST-26_LinksToSession", func(t *testing.T) {
		las := newSTMockLinkAccountStore()
		session := &auth.Session{ID: "sess-1", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
		req := stRequestWithSession(session)

		got, err := ProcessSteamLink(req, las, user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != session {
			t.Errorf("ProcessSteamLink() returned a different session than the one supplied")
		}
		if las.addCalls != 1 {
			t.Errorf("AddLinkedAccountToDB called %d times, want 1", las.addCalls)
		}
	})

	t.Run("ST-27_NoSessionRejected", func(t *testing.T) {
		las := newSTMockLinkAccountStore()
		req := stRequestWithSession(nil)

		_, err := ProcessSteamLink(req, las, user)
		if !errors.Is(err, errSessionNotFound) {
			t.Fatalf("ProcessSteamLink() error = %v, want %v", err, errSessionNotFound)
		}
	})

	t.Run("ST-28_ExpiredSessionRejected", func(t *testing.T) {
		las := newSTMockLinkAccountStore()
		session := &auth.Session{ID: "sess-1", UserID: "user-1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		req := stRequestWithSession(session)

		_, err := ProcessSteamLink(req, las, user)
		if !errors.Is(err, errSessionExpired) {
			t.Fatalf("ProcessSteamLink() error = %v, want %v", err, errSessionExpired)
		}
	})

	t.Run("ST-29_AlreadyLinkedToDifferentAccountRejected", func(t *testing.T) {
		las := newSTMockLinkAccountStore()
		las.byPlatformID[auth.PlatformSteam] = map[string]*auth.LinkedAccount{
			user.SteamID64: {UserID: "other-user", Platform: auth.PlatformSteam, PlatformID: user.SteamID64},
		}
		session := &auth.Session{ID: "sess-1", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
		req := stRequestWithSession(session)

		_, err := ProcessSteamLink(req, las, user)
		if !errors.Is(err, errPlatformAlreadyLinkedToDifferentAccount) {
			t.Fatalf("ProcessSteamLink() error = %v, want errPlatformAlreadyLinkedToDifferentAccount", err)
		}
	})
}
