package linking

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// -------------- Fakes / Mocks --------------

// fakePlatformData is a minimal auth.PlatformData implementation for tests,
// patterned after DiscordData in discord.go.
type fakePlatformData struct {
	id       string
	email    string
	username string
}

func (f *fakePlatformData) GetID() string       { return f.id }
func (f *fakePlatformData) GetEmail() string    { return f.email }
func (f *fakePlatformData) GetUsername() string { return f.username }
func (f *fakePlatformData) GetData() string     { return "{}" }
func (f *fakePlatformData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformDiscord, f.username, f.id, f)
}

// mockAccountService implements auth.AccountService for unit testing
// resolveOrCreateAccountForPlatformUser / ProcessOAuthLogin.
type mockAccountService struct {
	accounts  map[string]*auth.Account
	addErr    error
	deleteErr error

	deletedIDs []string
}

var _ auth.AccountService = (*mockAccountService)(nil)

func newMockAccountService() *mockAccountService {
	return &mockAccountService{accounts: make(map[string]*auth.Account)}
}

func (m *mockAccountService) AddAccount(a *auth.Account) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.accounts[a.UserID] = a
	return nil
}

func (m *mockAccountService) GetAccountByID(userID string) (*auth.Account, error) {
	if a, ok := m.accounts[userID]; ok {
		return a, nil
	}
	return nil, auth.ErrNotFound
}

func (m *mockAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}

func (m *mockAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}

func (m *mockAccountService) UpdateAccount(a *auth.Account) error {
	m.accounts[a.UserID] = a
	return nil
}

func (m *mockAccountService) DeleteAccount(userID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.accounts, userID)
	m.deletedIDs = append(m.deletedIDs, userID)
	return nil
}

// mockLinkAccountStore implements auth.LinkAccountStore for unit testing
// resolveOrCreateAccountForPlatformUser / ProcessOAuthLogin.
type mockLinkAccountStore struct {
	getByPlatformIDFunc func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error)
	addFunc             func(la *auth.LinkedAccount) error

	addCalls []*auth.LinkedAccount
	getCalls int
}

var _ auth.LinkAccountStore = (*mockLinkAccountStore)(nil)

func (m *mockLinkAccountStore) AddLinkedAccountToDB(la *auth.LinkedAccount) error {
	m.addCalls = append(m.addCalls, la)
	if m.addFunc != nil {
		return m.addFunc(la)
	}
	return nil
}

func (m *mockLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error {
	return nil
}

func (m *mockLinkAccountStore) GetLinkedAccountByPlatformID(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
	m.getCalls++
	return m.getByPlatformIDFunc(platform, platformID)
}

func (m *mockLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}

func (m *mockLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}

// -------------- resolveOrCreateAccountForPlatformUser tests --------------

func TestResolveOrCreateAccountForPlatformUserCreatesNewAccount(t *testing.T) {
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser", email: "someuser@example.com"}

	account, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForPlatformUser returned error: %v", err)
	}

	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	if account.UserID == "" {
		t.Error("expected the returned account to have a UserID")
	}
	if account.Username != "someuser" || account.Email != "someuser@example.com" {
		t.Errorf("expected the new account to be seeded from the platform data, got: %+v", account)
	}
	if len(als.addCalls) != 1 {
		t.Fatalf("expected AddLinkedAccountToDB to be called once, got %d", len(als.addCalls))
	}
	if als.addCalls[0].PlatformID != "pid1" || als.addCalls[0].UserID != account.UserID {
		t.Errorf("linked account not associated with the new account: %+v", als.addCalls[0])
	}
}

func TestResolveOrCreateAccountForPlatformUserUsesExistingLinkedAccount(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing123"] = &auth.Account{UserID: "existing123", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing123", PlatformID: "pid1"}, nil
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	account, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForPlatformUser returned error: %v", err)
	}

	if account.UserID != "existing123" {
		t.Errorf("expected the existing account to be returned, got UserID %q", account.UserID)
	}
	if len(als.addCalls) != 0 {
		t.Error("expected no new linked account to be created when one already exists")
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d accounts", len(as.accounts))
	}
}

// Under the pre-fix behavior (cleanup only ran when the AddLinkedAccountToDB
// error was auth.ErrAlreadyLinked, and the winner's account was never
// re-fetched) this would either fail to clean up or return the deleted
// placeholder account. resolveOrCreateAccountForPlatformUser must clean up
// the placeholder and hand back the winner's account instead.
func TestResolveOrCreateAccountForPlatformUserRaceLostCleansUpAndUsesWinner(t *testing.T) {
	as := newMockAccountService()
	as.accounts["winner1"] = &auth.Account{UserID: "winner1"}

	lookupCall := 0
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			lookupCall++
			if lookupCall == 1 {
				return nil, auth.ErrNotFound
			}
			return &auth.LinkedAccount{UserID: "winner1", Platform: platform, PlatformID: platformID}, nil
		},
		addFunc: func(*auth.LinkedAccount) error {
			return auth.ErrAlreadyLinked
		},
	}
	user := &fakePlatformData{id: "pid1", username: "loser"}

	account, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForPlatformUser returned error: %v", err)
	}

	if account.UserID != "winner1" {
		t.Errorf("expected the winner's account to be returned, got UserID %q", account.UserID)
	}
	if len(as.deletedIDs) != 1 {
		t.Fatalf("expected the losing placeholder account to be cleaned up, deletedIDs: %v", as.deletedIDs)
	}
	if as.deletedIDs[0] == "winner1" {
		t.Error("cleanup deleted the winner's account instead of the placeholder")
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected only the winner's account to remain, got %d accounts: %v", len(as.accounts), as.accounts)
	}
	if lookupCall != 2 {
		t.Errorf("expected GetLinkedAccountByPlatformID to be called twice (initial miss + post-race re-fetch), got %d", lookupCall)
	}
}

// Under the pre-fix behavior, cleanup only happened when the
// AddLinkedAccountToDB error was auth.ErrAlreadyLinked, so an unexpected
// error would leave the placeholder account orphaned. This must clean it up
// and still propagate the original error.
func TestResolveOrCreateAccountForPlatformUserUnexpectedAddLinkedAccountErrorPropagatesAndCleansUp(t *testing.T) {
	as := newMockAccountService()
	wantErr := errors.New("constraint violation, but not the one we auto-recover from")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
		addFunc: func(*auth.LinkedAccount) error {
			return wantErr
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the AddLinkedAccountToDB error to propagate, got: %v", err)
	}
	if len(as.accounts) != 0 {
		t.Errorf("expected the placeholder account to be cleaned up, got %d accounts remaining: %v", len(as.accounts), as.accounts)
	}
	if len(as.deletedIDs) != 1 {
		t.Errorf("expected DeleteAccount to be called once to clean up the orphaned account, got: %v", as.deletedIDs)
	}
}

// If the cleanup delete itself also fails, both errors must be surfaced
// rather than one silently swallowing the other.
func TestResolveOrCreateAccountForPlatformUserCleanupFailureWrapsBothErrors(t *testing.T) {
	as := newMockAccountService()
	addErr := errors.New("constraint violation, but not the one we auto-recover from")
	deleteErr := errors.New("db unreachable")
	as.deleteErr = deleteErr
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
		addFunc: func(*auth.LinkedAccount) error {
			return addErr
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if !errors.Is(err, addErr) {
		t.Errorf("expected the original AddLinkedAccountToDB error to be wrapped, got: %v", err)
	}
	if !errors.Is(err, deleteErr) {
		t.Errorf("expected the cleanup failure to be wrapped rather than swallowed, got: %v", err)
	}
}

func TestResolveOrCreateAccountForPlatformUserUnexpectedLookupErrorPropagates(t *testing.T) {
	as := newMockAccountService()
	wantErr := errors.New("db exploded")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, wantErr
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the lookup error to propagate, got: %v", err)
	}
	if len(as.accounts) != 0 {
		t.Error("no placeholder account should be created when the initial lookup fails for an unexpected reason")
	}
	if len(als.addCalls) != 0 {
		t.Error("AddLinkedAccountToDB should not be called when the initial lookup fails for an unexpected reason")
	}
}

// -------------- ProcessOAuthLink cheap early-return tests --------------

func TestProcessOAuthLinkNoSessionInContext(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	als := &mockLinkAccountStore{}
	state := &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink}

	_, err := ProcessOAuthLink(r, als, "some-code", state)
	if err == nil {
		t.Fatal("expected an error when the request has no session in context")
	}
	if err.Error() != "session not found" {
		t.Errorf("expected \"session not found\", got: %v", err)
	}
}

func TestProcessOAuthLinkExpiredSession(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	als := &mockLinkAccountStore{}
	state := &OAuthState{Platform: auth.PlatformDiscord, Mode: ModeLink}

	_, err := ProcessOAuthLink(r, als, "some-code", state)
	if err == nil {
		t.Fatal("expected an error when the session in context is expired")
	}
	if err.Error() != "session expired" {
		t.Errorf("expected \"session expired\", got: %v", err)
	}
}
