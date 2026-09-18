package auth

import (
	"errors"
	"testing"
)

// mockAccountStore implements AccountStore for unit testing userService.
type mockAccountStore struct {
	accounts  map[string]*Account
	addErr    error
	deleteErr error

	deletedIDs []string
}

func newMockAccountStore() *mockAccountStore {
	return &mockAccountStore{accounts: make(map[string]*Account)}
}

func (m *mockAccountStore) AddAccountToDB(a *Account) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.accounts[a.UserID] = a
	return nil
}

func (m *mockAccountStore) GetAccountByID(userID string) (*Account, error) {
	if a, ok := m.accounts[userID]; ok {
		return a, nil
	}
	return nil, ErrNotFound
}

func (m *mockAccountStore) GetAccountByUsername(string) (*Account, error) { return nil, ErrNotFound }
func (m *mockAccountStore) GetAccountByEmail(string) (*Account, error)    { return nil, ErrNotFound }

func (m *mockAccountStore) UpdateAccountInDB(a *Account) error {
	m.accounts[a.UserID] = a
	return nil
}

func (m *mockAccountStore) DeleteAccountFromDB(userID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.accounts, userID)
	m.deletedIDs = append(m.deletedIDs, userID)
	return nil
}

// mockLinkAccountStore implements LinkAccountStore for unit testing userService.
type mockLinkAccountStore struct {
	getByPlatformIDFunc func(platform Platform, platformID string) (*LinkedAccount, error)
	addFunc             func(la *LinkedAccount) error
	updateErr           error

	addCalls    []*LinkedAccount
	updateCalls []*LinkedAccount
	getCalls    int
}

func (m *mockLinkAccountStore) AddLinkedAccountToDB(la *LinkedAccount) error {
	m.addCalls = append(m.addCalls, la)
	if m.addFunc != nil {
		return m.addFunc(la)
	}
	return nil
}

func (m *mockLinkAccountStore) UpdateLinkedAccount(la *LinkedAccount) error {
	m.updateCalls = append(m.updateCalls, la)
	return m.updateErr
}

func (m *mockLinkAccountStore) GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error) {
	m.getCalls++
	return m.getByPlatformIDFunc(platform, platformID)
}

func (m *mockLinkAccountStore) GetLinkedAccountByPlatformName(Platform, string) (*LinkedAccount, error) {
	return nil, ErrNotFound
}

func (m *mockLinkAccountStore) GetLinkedAccountByUserID(string, Platform) (*LinkedAccount, error) {
	return nil, ErrNotFound
}

func TestUserServiceUpdateUserFromPlatformCreatesNewAccount(t *testing.T) {
	as := newMockAccountStore()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
			return nil, ErrNotFound
		},
	}
	svc := &userService{as: as, als: als}

	account, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil)
	if err != nil {
		t.Fatalf("UpdateUserFromPlatform returned error: %v", err)
	}

	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	if account.UserID == "" {
		t.Error("expected the returned account to have a UserID")
	}
	if len(als.addCalls) != 1 {
		t.Fatalf("expected AddLinkedAccountToDB to be called once, got %d", len(als.addCalls))
	}
	if als.addCalls[0].PlatformID != "pid1" || als.addCalls[0].UserID != account.UserID {
		t.Errorf("linked account not associated with the new account: %+v", als.addCalls[0])
	}
	if len(als.updateCalls) != 1 {
		t.Errorf("expected UpdateLinkedAccount to be called once to persist the platform data, got %d", len(als.updateCalls))
	}
}

func TestUserServiceUpdateUserFromPlatformUsesExistingLinkedAccount(t *testing.T) {
	as := newMockAccountStore()
	as.accounts["existing123"] = &Account{UserID: "existing123"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
			return &LinkedAccount{UserID: "existing123", PlatformID: "pid1"}, nil
		},
	}
	svc := &userService{as: as, als: als}

	account, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil)
	if err != nil {
		t.Fatalf("UpdateUserFromPlatform returned error: %v", err)
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

func TestUserServiceUpdateUserFromPlatformUpdateLinkedAccountErrorPropagates(t *testing.T) {
	as := newMockAccountStore()
	as.accounts["existing123"] = &Account{UserID: "existing123"}
	wantErr := errors.New("db down")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
			return &LinkedAccount{UserID: "existing123", PlatformID: "pid1"}, nil
		},
		updateErr: wantErr,
	}
	svc := &userService{as: as, als: als}

	if _, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil); !errors.Is(err, wantErr) {
		t.Errorf("expected the UpdateLinkedAccount error to propagate, got: %v", err)
	}
}

func TestUserServiceUpdateUserFromPlatformFinalGetAccountErrorPropagates(t *testing.T) {
	as := newMockAccountStore() // deliberately empty: the linked account points at an account that doesn't exist
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
			return &LinkedAccount{UserID: "missing123", PlatformID: "pid1"}, nil
		},
	}
	svc := &userService{as: as, als: als}

	if _, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected the final GetAccountByID error to propagate, got: %v", err)
	}
}

// Regression test: ProcessOAuthLogin's sibling logic previously never re-fetched
// the winner's account/linked account after losing this race, leaving a dangling
// reference to the account that was just deleted. This locks in the fixed cleanup path.
func TestUserServiceUpdateUserFromPlatformRaceLostCleansUpAndUsesWinner(t *testing.T) {
	as := newMockAccountStore()
	as.accounts["winner1"] = &Account{UserID: "winner1"}

	// Use a counter closure so the first lookup misses (triggering account
	// creation) and the post-race-loss re-fetch returns the winner's row.
	lookupCall := 0
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform Platform, platformID string) (*LinkedAccount, error) {
			lookupCall++
			if lookupCall == 1 {
				return nil, ErrNotFound
			}
			return &LinkedAccount{UserID: "winner1", Platform: platform, PlatformID: platformID}, nil
		},
		addFunc: func(*LinkedAccount) error {
			return ErrAlreadyLinked
		},
	}
	svc := &userService{as: as, als: als}

	account, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil)
	if err != nil {
		t.Fatalf("UpdateUserFromPlatform returned error: %v", err)
	}

	if account.UserID != "winner1" {
		t.Errorf("expected the winner's account to be returned, got UserID %q", account.UserID)
	}
	if len(as.deletedIDs) != 1 {
		t.Fatalf("expected the losing account to be cleaned up, deletedIDs: %v", as.deletedIDs)
	}
	if as.deletedIDs[0] == "winner1" {
		t.Error("cleanup deleted the winner's account instead of the losing one")
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected only the winner's account to remain, got %d accounts: %v", len(as.accounts), as.accounts)
	}
	if lookupCall != 2 {
		t.Errorf("expected GetLinkedAccountByPlatformID to be called twice (initial miss + post-race re-fetch), got %d", lookupCall)
	}
}

func TestUserServiceUpdateUserFromPlatformUnexpectedLookupErrorPropagates(t *testing.T) {
	as := newMockAccountStore()
	wantErr := errors.New("db exploded")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
			return nil, wantErr
		},
	}
	svc := &userService{as: as, als: als}

	if _, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil); !errors.Is(err, wantErr) {
		t.Errorf("expected the lookup error to propagate, got: %v", err)
	}
	if len(as.accounts) != 0 {
		t.Error("no account should be created when the initial lookup fails for an unexpected reason")
	}
}

func TestUserServiceUpdateUserFromPlatformUnexpectedAddLinkedAccountErrorPropagates(t *testing.T) {
	as := newMockAccountStore()
	wantErr := errors.New("constraint violation, but not the one we auto-recover from")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
			return nil, ErrNotFound
		},
		addFunc: func(*LinkedAccount) error {
			return wantErr
		},
	}
	svc := &userService{as: as, als: als}

	if _, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil); !errors.Is(err, wantErr) {
		t.Errorf("expected the AddLinkedAccountToDB error to propagate, got: %v", err)
	}
	if len(as.accounts) != 0 {
		t.Errorf("expected the placeholder account to be cleaned up, got %d accounts remaining: %v", len(as.accounts), as.accounts)
	}
	if len(as.deletedIDs) != 1 {
		t.Errorf("expected DeleteAccountFromDB to be called once to clean up the orphaned account, got: %v", as.deletedIDs)
	}
}

func TestUserServiceUpdateUserFromPlatformCleanupFailureWrapsBothErrors(t *testing.T) {
	as := newMockAccountStore()
	addErr := errors.New("constraint violation, but not the one we auto-recover from")
	deleteErr := errors.New("db unreachable")
	as.deleteErr = deleteErr
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(Platform, string) (*LinkedAccount, error) {
			return nil, ErrNotFound
		},
		addFunc: func(*LinkedAccount) error {
			return addErr
		},
	}
	svc := &userService{as: as, als: als}

	_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "pid1", nil)
	if !errors.Is(err, addErr) {
		t.Errorf("expected the original AddLinkedAccountToDB error to be wrapped, got: %v", err)
	}
	if !errors.Is(err, deleteErr) {
		t.Errorf("expected the cleanup failure to be wrapped rather than swallowed, got: %v", err)
	}
}
