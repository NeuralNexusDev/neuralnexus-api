package linking

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
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
	// Mirrors the real accounts.username UNIQUE constraint (a genuine,
	// non-empty duplicate only - see store.go's NULLIF/COALESCE handling)
	// so tests can exercise a platform-supplied username that collides
	// with an existing account's username.
	if a.Username != "" {
		for _, existing := range m.accounts {
			if existing.Username == a.Username {
				return auth.ErrUsernameAlreadyExists
			}
		}
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

func (m *mockLinkAccountStore) GetLinkedAccountsByUserID(string) ([]*auth.LinkedAccount, error) {
	return nil, nil
}

func (m *mockLinkAccountStore) DeleteLinkedAccount(string, auth.Platform) error {
	return nil
}

func (m *mockLinkAccountStore) SetLinkedAccountLoginEnabled(string, auth.Platform, bool) error {
	return nil
}

// concurrentAccountService is a goroutine-safe variant of mockAccountService.
// The plain-map mock above is intentionally unsynchronized (it's only ever
// driven sequentially by the other tests); reusing it under concurrent
// goroutines would trip -race on the test double itself rather than on
// resolveOrCreateAccountForPlatformUser.
type concurrentAccountService struct {
	mu       sync.Mutex
	accounts map[string]*auth.Account
	deleted  []string
}

var _ auth.AccountService = (*concurrentAccountService)(nil)

func newConcurrentAccountService() *concurrentAccountService {
	return &concurrentAccountService{accounts: make(map[string]*auth.Account)}
}

func (m *concurrentAccountService) AddAccount(a *auth.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.UserID] = a
	return nil
}

func (m *concurrentAccountService) GetAccountByID(userID string) (*auth.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.accounts[userID]; ok {
		return a, nil
	}
	return nil, auth.ErrNotFound
}

func (m *concurrentAccountService) GetAccountByUsername(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}

func (m *concurrentAccountService) GetAccountByEmail(string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}

func (m *concurrentAccountService) UpdateAccount(a *auth.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.UserID] = a
	return nil
}

func (m *concurrentAccountService) DeleteAccount(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.accounts, userID)
	m.deleted = append(m.deleted, userID)
	return nil
}

func (m *concurrentAccountService) remaining() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.accounts)
}

// concurrentLinkAccountStore enforces a unique-constraint-like guarantee on
// (platform, platformID) atomically under a mutex, mirroring the real
// Postgres unique index this logic depends on for correctness.
type concurrentLinkAccountStore struct {
	mu    sync.Mutex
	byKey map[string]*auth.LinkedAccount
}

var _ auth.LinkAccountStore = (*concurrentLinkAccountStore)(nil)

func newConcurrentLinkAccountStore() *concurrentLinkAccountStore {
	return &concurrentLinkAccountStore{byKey: make(map[string]*auth.LinkedAccount)}
}

func linkKey(platform auth.Platform, platformID string) string {
	return string(platform) + ":" + platformID
}

func (m *concurrentLinkAccountStore) AddLinkedAccountToDB(la *auth.LinkedAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := linkKey(la.Platform, la.PlatformID)
	if _, exists := m.byKey[key]; exists {
		return auth.ErrAlreadyLinked
	}
	m.byKey[key] = la
	return nil
}

func (m *concurrentLinkAccountStore) UpdateLinkedAccount(*auth.LinkedAccount) error { return nil }

func (m *concurrentLinkAccountStore) GetLinkedAccountByPlatformID(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if la, ok := m.byKey[linkKey(platform, platformID)]; ok {
		return la, nil
	}
	return nil, auth.ErrNotFound
}

func (m *concurrentLinkAccountStore) GetLinkedAccountByPlatformName(auth.Platform, string) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}

func (m *concurrentLinkAccountStore) GetLinkedAccountByUserID(string, auth.Platform) (*auth.LinkedAccount, error) {
	return nil, auth.ErrNotFound
}

func (m *concurrentLinkAccountStore) GetLinkedAccountsByUserID(userID string) ([]*auth.LinkedAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*auth.LinkedAccount
	for _, la := range m.byKey {
		if la.UserID == userID {
			result = append(result, la)
		}
	}
	return result, nil
}

// hasOtherUsableLoginMethodLocked mirrors the real store's guard predicate
// (minus the password check - none of these tests use a passworded
// account). Callers must already hold m.mu.
func hasOtherUsableLoginMethodLocked(byKey map[string]*auth.LinkedAccount, userID string, excludePlatform auth.Platform) bool {
	for _, la := range byKey {
		if la.UserID == userID && la.Platform != excludePlatform && la.Verified && la.LoginEnabled {
			return true
		}
	}
	return false
}

func (m *concurrentLinkAccountStore) DeleteLinkedAccount(userID string, platform auth.Platform) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key, ok := "", false
	for k, la := range m.byKey {
		if la.UserID == userID && la.Platform == platform {
			key, ok = k, true
			break
		}
	}
	if !ok {
		return auth.ErrNotFound
	}
	if !hasOtherUsableLoginMethodLocked(m.byKey, userID, platform) {
		return auth.ErrWouldLockAccount
	}
	delete(m.byKey, key)
	return nil
}

func (m *concurrentLinkAccountStore) SetLinkedAccountLoginEnabled(userID string, platform auth.Platform, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var target *auth.LinkedAccount
	for _, la := range m.byKey {
		if la.UserID == userID && la.Platform == platform {
			target = la
			break
		}
	}
	if target == nil {
		return auth.ErrNotFound
	}
	if enabled {
		if !target.Verified {
			return auth.ErrLinkedAccountUnverified
		}
		target.LoginEnabled = true
		return nil
	}
	if !hasOtherUsableLoginMethodLocked(m.byKey, userID, platform) {
		return auth.ErrWouldLockAccount
	}
	target.LoginEnabled = false
	return nil
}

// TestResolveOrCreateAccountForPlatformUserConcurrentRaceExactlyOneWinner
// drives many goroutines through resolveOrCreateAccountForPlatformUser at
// once for the *same* platform identity, mirroring two clients starting an
// OAuth login for the same external account at nearly the same time. Unlike
// the sequential race-simulation test above (which scripts a single retry
// via canned mock return values), this exercises the real
// AddLinkedAccountToDB/DeleteAccount/GetLinkedAccountByPlatformID call
// sequence under actual goroutine contention against a store that enforces
// the uniqueness guarantee the real Postgres constraint provides.
func TestResolveOrCreateAccountForPlatformUserConcurrentRaceExactlyOneWinner(t *testing.T) {
	const n = 20
	as := newConcurrentAccountService()
	als := newConcurrentLinkAccountStore()
	user := &fakePlatformData{id: "pid-race", username: "racer"}

	var wg sync.WaitGroup
	accounts := make([]*auth.Account, n)
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			accounts[i], errs[i] = resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: resolveOrCreateAccountForPlatformUser returned error: %v", i, err)
		}
	}

	winner := accounts[0].UserID
	for i, a := range accounts {
		if a.UserID != winner {
			t.Errorf("goroutine %d returned a different account (%q) than goroutine 0 (%q); all callers linking the same platform identity concurrently must converge on one account", i, a.UserID, winner)
		}
	}

	if got := as.remaining(); got != 1 {
		t.Errorf("expected exactly one account to survive the race (all losing placeholders cleaned up), got %d", got)
	}
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
	if account.Username != "someuser" || account.Email == nil || *account.Email != "someuser@example.com" {
		t.Errorf("expected the new account to be seeded from the platform data, got: %+v", account)
	}
	if len(als.addCalls) != 1 {
		t.Fatalf("expected AddLinkedAccountToDB to be called once, got %d", len(als.addCalls))
	}
	if als.addCalls[0].PlatformID != "pid1" || als.addCalls[0].UserID != account.UserID {
		t.Errorf("linked account not associated with the new account: %+v", als.addCalls[0])
	}
}

// TestResolveOrCreateAccountForPlatformUserUsernameCollisionPropagatesCleanly
// covers a scenario from the "platform data collides with an existing
// account" family: a brand-new platform identity (no linked_accounts row
// yet) reports a username that collides with a completely unrelated,
// already-existing account's username. AddAccount fails before any linked
// account row is ever inserted, so there is nothing to clean up - this
// pins down that the error surfaces as the ErrUsernameAlreadyExists
// sentinel (not a raw DB error) and that no linked account or extra
// account is left behind.
func TestResolveOrCreateAccountForPlatformUserUsernameCollisionPropagatesCleanly(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing123"] = &auth.Account{UserID: "existing123", Username: "taken-username"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	user := &fakePlatformData{id: "pid-new", username: "taken-username", email: "newperson@example.com"}

	_, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if !errors.Is(err, auth.ErrUsernameAlreadyExists) {
		t.Errorf("expected auth.ErrUsernameAlreadyExists, got: %v", err)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected only the original account to remain, got %d: %v", len(as.accounts), as.accounts)
	}
	if len(as.deletedIDs) != 0 {
		t.Errorf("nothing should need cleanup since AddAccount never created anything, got deletedIDs: %v", as.deletedIDs)
	}
	if len(als.addCalls) != 0 {
		t.Errorf("AddLinkedAccountToDB should never be called when AddAccount fails first, got %d calls", len(als.addCalls))
	}
}

func TestResolveOrCreateAccountForPlatformUserUsesExistingLinkedAccount(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing123"] = &auth.Account{UserID: "existing123", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing123", PlatformID: "pid1", Verified: true, LoginEnabled: true}, nil
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

// TestResolveOrCreateAccountForPlatformUserLoginDisabledRejected is the
// regression test for the login_enabled/verified infrastructure: a platform
// identity that's already linked to a real account, but not eligible for
// login, must be rejected outright - never silently treated as unlinked
// (which would create a second, duplicate account for someone who already
// has one) and never silently logged in anyway (which would defeat
// disabling it in the first place).
func TestResolveOrCreateAccountForPlatformUserLoginDisabledRejected(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing123"] = &auth.Account{UserID: "existing123", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing123", PlatformID: "pid1", Verified: true, LoginEnabled: false}, nil
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if !errors.Is(err, errPlatformLoginDisabled) {
		t.Fatalf("expected errPlatformLoginDisabled, got: %v", err)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d accounts", len(as.accounts))
	}
	if len(als.addCalls) != 0 {
		t.Error("expected no linking attempt for a login-disabled identity")
	}
}

// TestResolveOrCreateAccountForPlatformUserUnverifiedRejected mirrors the
// login-disabled case for an unverified row - defense in depth, since
// nothing in this package can construct one today, but the login path must
// still never trust one if it ever exists.
func TestResolveOrCreateAccountForPlatformUserUnverifiedRejected(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing123"] = &auth.Account{UserID: "existing123", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing123", PlatformID: "pid1", Verified: false, LoginEnabled: true}, nil
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := resolveOrCreateAccountForPlatformUser(as, als, auth.PlatformDiscord, user)
	if !errors.Is(err, errPlatformLoginDisabled) {
		t.Fatalf("expected errPlatformLoginDisabled, got: %v", err)
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

// -------------- linkPlatformUserToSession tests --------------

func TestLinkPlatformUserToSessionNotYetLinked(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	got, err := linkPlatformUserToSession(als, session, auth.PlatformDiscord, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 1 {
		t.Fatalf("expected AddLinkedAccountToDB to be called once, got %d calls", len(als.addCalls))
	}
	if als.addCalls[0].UserID != session.UserID {
		t.Errorf("expected the new link to be created for the session's user %q, got %q", session.UserID, als.addCalls[0].UserID)
	}
}

func TestLinkPlatformUserToSessionAlreadyLinkedToSameAccount(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "u1"}, nil
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	got, err := linkPlatformUserToSession(als, session, auth.PlatformDiscord, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != session {
		t.Error("expected the same session to be returned")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("re-linking a platform account already linked to the same account should be a no-op, but AddLinkedAccountToDB was called %d time(s)", len(als.addCalls))
	}
}

func TestLinkPlatformUserToSessionAlreadyLinkedToDifferentAccount(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := linkPlatformUserToSession(als, session, auth.PlatformDiscord, user)
	if err == nil {
		t.Fatal("expected an error when the platform account is already linked to a different account")
	}
	if err.Error() != "this platform account is already linked to a different account; log in with it directly if you want to use that account, or unlink it there first" {
		t.Errorf("expected the already-linked-to-a-different-account message, got: %v", err)
	}
	if len(als.addCalls) != 0 {
		t.Errorf("AddLinkedAccountToDB should not be called when the platform account belongs to a different account, got %d calls", len(als.addCalls))
	}
}

func TestLinkPlatformUserToSessionAddFails(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
		addFunc: func(*auth.LinkedAccount) error {
			return errors.New("db exploded")
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := linkPlatformUserToSession(als, session, auth.PlatformDiscord, user)
	if err == nil {
		t.Fatal("expected an error when AddLinkedAccountToDB fails")
	}
	if err.Error() != "failed to link account" {
		t.Errorf("expected \"failed to link account\", got: %v", err)
	}
}

func TestLinkPlatformUserToSessionLookupError(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	wantErr := errors.New("db exploded")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, wantErr
		},
	}
	user := &fakePlatformData{id: "pid1", username: "someuser"}

	_, err := linkPlatformUserToSession(als, session, auth.PlatformDiscord, user)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the lookup error to propagate, got: %v", err)
	}
	if len(als.addCalls) != 0 {
		t.Error("AddLinkedAccountToDB should not be called when the initial lookup fails for an unexpected reason")
	}
}
