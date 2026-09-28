package auth

import (
	"errors"
	"sync"
	"testing"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

// usFakePlatformData is a minimal PlatformData double.
type usFakePlatformData struct {
	username string
}

func (f usFakePlatformData) GetID() string       { return "pid" }
func (f usFakePlatformData) GetEmail() string    { return "" }
func (f usFakePlatformData) GetUsername() string { return f.username }
func (f usFakePlatformData) GetData() string     { return "{}" }
func (f usFakePlatformData) CreateLinkedAccount(_ string) *LinkedAccount {
	return nil
}

// usFakeAccountStore is a controllable AccountStore double for userService
// tests.
type usFakeAccountStore struct {
	addErr         error
	addCalls       int
	addLastAccount *Account

	getByIDAccount    *Account
	getByIDErr        error
	getByIDCalls      int
	getByIDLastUserID string

	updateErr         error
	updateCalls       int
	updateLastAccount *Account

	deleteErr        error
	deleteCalls      int
	deleteLastUserID string
}

func (f *usFakeAccountStore) AddAccountToDB(a *Account) error {
	f.addCalls++
	f.addLastAccount = a
	return f.addErr
}
func (f *usFakeAccountStore) GetAccountByID(userID string) (*Account, error) {
	f.getByIDCalls++
	f.getByIDLastUserID = userID
	return f.getByIDAccount, f.getByIDErr
}
func (f *usFakeAccountStore) GetAccountByUsername(_ string) (*Account, error) {
	return nil, ErrNotFound
}
func (f *usFakeAccountStore) GetAccountByEmail(_ string) (*Account, error) {
	return nil, ErrNotFound
}
func (f *usFakeAccountStore) UpdateAccountInDB(a *Account) error {
	f.updateCalls++
	f.updateLastAccount = a
	return f.updateErr
}
func (f *usFakeAccountStore) DeleteAccountFromDB(userID string) error {
	f.deleteCalls++
	f.deleteLastUserID = userID
	return f.deleteErr
}

// usLinkResult is one queued result for usFakeLinkAccountStore's
// GetLinkedAccountByPlatformID, so a test can script different answers
// across successive calls (e.g. "not found" then "found the race winner").
type usLinkResult struct {
	la  *LinkedAccount
	err error
}

// usFakeLinkAccountStore is a controllable LinkAccountStore double.
type usFakeLinkAccountStore struct {
	getByPlatformIDResults []usLinkResult
	getByPlatformIDCalls   int

	addErr               error
	addCalls             int
	addLastLinkedAccount *LinkedAccount

	updateErr               error
	updateCalls             int
	updateLastLinkedAccount *LinkedAccount

	getsByUserID    []*LinkedAccount
	getsByUserIDErr error

	deleteErr   error
	deleteCalls int

	setLoginEnabledErr   error
	setLoginEnabledCalls int
}

func (f *usFakeLinkAccountStore) AddLinkedAccountToDB(la *LinkedAccount) error {
	f.addCalls++
	f.addLastLinkedAccount = la
	return f.addErr
}
func (f *usFakeLinkAccountStore) UpdateLinkedAccount(la *LinkedAccount) error {
	f.updateCalls++
	f.updateLastLinkedAccount = la
	return f.updateErr
}
func (f *usFakeLinkAccountStore) GetLinkedAccountByPlatformID(_ Platform, _ string) (*LinkedAccount, error) {
	i := f.getByPlatformIDCalls
	f.getByPlatformIDCalls++
	if i < len(f.getByPlatformIDResults) {
		r := f.getByPlatformIDResults[i]
		return r.la, r.err
	}
	last := f.getByPlatformIDResults[len(f.getByPlatformIDResults)-1]
	return last.la, last.err
}
func (f *usFakeLinkAccountStore) GetLinkedAccountByPlatformName(_ Platform, _ string) (*LinkedAccount, error) {
	return nil, ErrNotFound
}
func (f *usFakeLinkAccountStore) GetLinkedAccountByUserID(_ string, _ Platform) (*LinkedAccount, error) {
	return nil, ErrNotFound
}
func (f *usFakeLinkAccountStore) GetLinkedAccountsByUserID(_ string) ([]*LinkedAccount, error) {
	return f.getsByUserID, f.getsByUserIDErr
}
func (f *usFakeLinkAccountStore) DeleteLinkedAccount(_ string, _ Platform) error {
	f.deleteCalls++
	return f.deleteErr
}
func (f *usFakeLinkAccountStore) SetLinkedAccountLoginEnabled(_ string, _ Platform, _ bool) error {
	f.setLoginEnabledCalls++
	return f.setLoginEnabledErr
}

// usFakeAccountSettingsStore is a controllable AccountSettingsStore double.
type usFakeAccountSettingsStore struct {
	settings *AccountSettings
	getErr   error
	setErr   error
	setCalls int
}

func (f *usFakeAccountSettingsStore) GetAccountSettings(_ string) (*AccountSettings, error) {
	return f.settings, f.getErr
}
func (f *usFakeAccountSettingsStore) SetPasswordAuthEnabled(_ string, _ bool) error {
	f.setCalls++
	return f.setErr
}

// usFakeStore is a minimal Store double wiring Account()/LinkAccount()/
// AccountSettings(); Session/RateLimit/OAuthToken panic if reached, since
// NewUserService never calls them.
type usFakeStore struct {
	as  AccountStore
	als LinkAccountStore
	ass AccountSettingsStore
}

func (f *usFakeStore) Account() AccountStore                 { return f.as }
func (f *usFakeStore) AccountSettings() AccountSettingsStore { return f.ass }
func (f *usFakeStore) Session() SessionStore                 { panic("usFakeStore: Session not implemented") }
func (f *usFakeStore) LinkAccount() LinkAccountStore         { return f.als }
func (f *usFakeStore) RateLimit() RateLimitStore             { panic("usFakeStore: RateLimit not implemented") }
func (f *usFakeStore) OAuthToken() OAuthTokenStore           { panic("usFakeStore: OAuthToken not implemented") }

func usNewService(as *usFakeAccountStore, als *usFakeLinkAccountStore, ass *usFakeAccountSettingsStore) UserService {
	return NewUserService(&usFakeStore{as: as, als: als, ass: ass})
}

func TestUS01NewUserService(t *testing.T) {
	t.Run("US-01_WiresGivenSubStores", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "u1"}}
		als := &usFakeLinkAccountStore{}
		ass := &usFakeAccountSettingsStore{}
		svc := usNewService(as, als, ass)

		if _, err := svc.GetUser("u1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if as.getByIDCalls != 1 {
			t.Errorf("expected GetUser to delegate to the store.Account() instance passed to NewUserService")
		}
	})
}

func TestUS02to03GetUser(t *testing.T) {
	t.Run("US-02_Success", func(t *testing.T) {
		want := &Account{UserID: "u1"}
		svc := usNewService(&usFakeAccountStore{getByIDAccount: want}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUser("u1")
		if err != nil || got != want {
			t.Errorf("GetUser() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-03_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		svc := usNewService(&usFakeAccountStore{getByIDErr: wantErr}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		_, err := svc.GetUser("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetUser() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS04to06GetUserFromPlatform(t *testing.T) {
	t.Run("US-04_Success", func(t *testing.T) {
		want := &Account{UserID: "u1"}
		as := &usFakeAccountStore{getByIDAccount: want}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{&LinkedAccount{UserID: "u1"}, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserFromPlatform(PlatformDiscord, "p1")
		if err != nil || got != want {
			t.Errorf("GetUserFromPlatform() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-05_LookupFails", func(t *testing.T) {
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, ErrNotFound}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.GetUserFromPlatform(PlatformDiscord, "p1")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("GetUserFromPlatform() err = %v, want ErrNotFound", err)
		}
		if as.getByIDCalls != 0 {
			t.Error("expected GetAccountByID to never be called when the link lookup fails")
		}
	})

	t.Run("US-06_ResolvedAccountLookupFails", func(t *testing.T) {
		wantErr := errors.New("boom")
		as := &usFakeAccountStore{getByIDErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{&LinkedAccount{UserID: "u1"}, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.GetUserFromPlatform(PlatformDiscord, "p1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetUserFromPlatform() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS07to10GetUserPermissions(t *testing.T) {
	t.Run("US-07_Success", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "u1", Roles: []string{perms.RoleSystem.Name}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserPermissions("u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := make([]string, 0, len(perms.RoleSystem.Permissions))
		for _, p := range perms.RoleSystem.Permissions {
			want = append(want, p.Name+"|"+p.Value)
		}
		if len(got) != len(want) {
			t.Fatalf("GetUserPermissions() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("permission[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("US-08_AccountLookupFails", func(t *testing.T) {
		wantErr := errors.New("boom")
		svc := usNewService(&usFakeAccountStore{getByIDErr: wantErr}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserPermissions("u1")
		if got != nil || !errors.Is(err, wantErr) {
			t.Errorf("GetUserPermissions() = (%v, %v), want (nil, %v)", got, err, wantErr)
		}
	})

	t.Run("US-09_SkipsUnknownRole", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "u1", Roles: []string{"not-a-role", perms.RoleOwner.Name}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserPermissions("u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != len(perms.RoleOwner.Permissions) {
			t.Errorf("GetUserPermissions() = %v, want just RoleOwner's %d permissions (unknown role skipped)", got, len(perms.RoleOwner.Permissions))
		}
	})

	t.Run("US-10_NoRoles", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "u1"}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserPermissions("u1")
		if err != nil || len(got) != 0 {
			t.Errorf("GetUserPermissions() = (%v, %v), want (empty, nil)", got, err)
		}
	})
}

func TestUS11to14UpdateUser(t *testing.T) {
	oldEmail := "old@b.com"
	newEmail := "new@b.com"

	t.Run("US-11_MergesNonZeroFields", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "u1", Username: "old", Email: &oldEmail, Roles: []string{"old"}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&Account{UserID: "u1", Username: "new", Email: &newEmail, Roles: []string{"new"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if as.updateCalls != 1 {
			t.Fatalf("expected UpdateAccountInDB to be called once, got %d", as.updateCalls)
		}
		got := as.updateLastAccount
		if got.Username != "new" || got.Email == nil || *got.Email != "new@b.com" || len(got.Roles) != 1 || got.Roles[0] != "new" {
			t.Errorf("UpdateAccountInDB called with %+v, want merged fields overwritten", got)
		}
	})

	t.Run("US-12_ZeroValueFieldsPreserveExisting", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "u1", Username: "old", Email: &oldEmail, Roles: []string{"old"}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&Account{UserID: "u1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := as.updateLastAccount
		if got.Username != "old" || got.Email == nil || *got.Email != oldEmail || len(got.Roles) != 1 || got.Roles[0] != "old" {
			t.Errorf("UpdateAccountInDB called with %+v, want the original fields preserved when the patch supplies zero values", got)
		}
	})

	t.Run("US-13_GetAccountFails", func(t *testing.T) {
		wantErr := errors.New("boom")
		as := &usFakeAccountStore{getByIDErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&Account{UserID: "u1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUser() err = %v, want %v", err, wantErr)
		}
		if as.updateCalls != 0 {
			t.Error("expected UpdateAccountInDB to never be called when the initial lookup fails")
		}
	})

	t.Run("US-14_UpdateFails", func(t *testing.T) {
		wantErr := errors.New("boom")
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "u1"}, updateErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&Account{UserID: "u1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUser() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS15to24UpdateUserFromPlatform(t *testing.T) {
	t.Run("US-15_NewIdentityCreatesAccount", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "resolved"}}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, ErrNotFound}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		got, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{username: "alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != as.getByIDAccount {
			t.Errorf("UpdateUserFromPlatform() = %v, want %v", got, as.getByIDAccount)
		}
		if as.addCalls != 1 || als.addCalls != 1 {
			t.Errorf("expected exactly one AddAccountToDB and one AddLinkedAccountToDB call, got %d/%d", as.addCalls, als.addCalls)
		}
		if als.addLastLinkedAccount.UserID != as.addLastAccount.UserID {
			t.Errorf("linked account UserID %q does not match the newly created placeholder account %q", als.addLastLinkedAccount.UserID, as.addLastAccount.UserID)
		}
	})

	t.Run("US-16_ExistingIdentitySkipsCreation", func(t *testing.T) {
		existing := &LinkedAccount{UserID: "existing-user", Platform: PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "existing-user"}}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{existing, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		got, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{username: "alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if as.addCalls != 0 {
			t.Error("expected AddAccountToDB to never be called for an already-linked identity")
		}
		if als.updateCalls != 1 || als.updateLastLinkedAccount.UserID != "existing-user" {
			t.Errorf("expected UpdateLinkedAccount to be called once for the existing link, got calls=%d last=%+v", als.updateCalls, als.updateLastLinkedAccount)
		}
		if got != as.getByIDAccount {
			t.Errorf("UpdateUserFromPlatform() = %v, want %v", got, as.getByIDAccount)
		}
	})

	t.Run("US-17_InitialLookupUnexpectedError", func(t *testing.T) {
		wantErr := errors.New("boom")
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, wantErr}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
		if as.addCalls != 0 {
			t.Error("expected no account creation attempt on an unexpected lookup error")
		}
	})

	t.Run("US-18_AddAccountFails", func(t *testing.T) {
		wantErr := errors.New("boom")
		as := &usFakeAccountStore{addErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, ErrNotFound}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
		if als.addCalls != 0 {
			t.Error("expected AddLinkedAccountToDB to never be called when AddAccountToDB fails")
		}
	})

	t.Run("US-19_AddLinkFailsCleansUpPlaceholder", func(t *testing.T) {
		wantErr := errors.New("insert failed")
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, ErrNotFound}}, addErr: wantErr}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
		if as.deleteCalls != 1 || as.deleteLastUserID != as.addLastAccount.UserID {
			t.Errorf("expected DeleteAccountFromDB to clean up the orphaned placeholder %q, got calls=%d last=%q", as.addLastAccount.UserID, as.deleteCalls, as.deleteLastUserID)
		}
	})

	t.Run("US-20_AddLinkFailsAndCleanupFailsWrapsBoth", func(t *testing.T) {
		linkErr := errors.New("insert failed")
		deleteErr := errors.New("delete failed")
		as := &usFakeAccountStore{deleteErr: deleteErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, ErrNotFound}}, addErr: linkErr}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if err == nil {
			t.Fatal("expected a non-nil error")
		}
		if !errors.Is(err, linkErr) || !errors.Is(err, deleteErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want it to wrap both %v and %v", err, linkErr, deleteErr)
		}
	})

	t.Run("US-21_LostRaceUsesWinner", func(t *testing.T) {
		winner := &LinkedAccount{UserID: "winner-user", Platform: PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{getByIDAccount: &Account{UserID: "winner-user"}}
		als := &usFakeLinkAccountStore{
			getByPlatformIDResults: []usLinkResult{{nil, ErrNotFound}, {winner, nil}},
			addErr:                 ErrAlreadyLinked,
		}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		got, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != as.getByIDAccount {
			t.Errorf("UpdateUserFromPlatform() = %v, want the winner's account %v", got, as.getByIDAccount)
		}
		if as.deleteCalls != 1 {
			t.Errorf("expected the loser's placeholder to be cleaned up, got %d DeleteAccountFromDB calls", as.deleteCalls)
		}
		if als.updateLastLinkedAccount.UserID != "winner-user" {
			t.Errorf("expected the winner's linked account to be updated, got UserID=%q", als.updateLastLinkedAccount.UserID)
		}
		if als.getByPlatformIDCalls != 2 {
			t.Errorf("expected the link to be looked up twice (initial miss + winner re-fetch), got %d", als.getByPlatformIDCalls)
		}
	})

	t.Run("US-22_LostRaceRefetchFails", func(t *testing.T) {
		wantErr := errors.New("refetch failed")
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{
			getByPlatformIDResults: []usLinkResult{{nil, ErrNotFound}, {nil, wantErr}},
			addErr:                 ErrAlreadyLinked,
		}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
	})

	t.Run("US-23_UpdateLinkedAccountFails", func(t *testing.T) {
		wantErr := errors.New("update failed")
		existing := &LinkedAccount{UserID: "u1", Platform: PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{existing, nil}}, updateErr: wantErr}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
	})

	t.Run("US-24_FinalGetAccountFails", func(t *testing.T) {
		wantErr := errors.New("get failed")
		existing := &LinkedAccount{UserID: "u1", Platform: PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{getByIDErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{existing, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
	})
}

// usConcurrentStore is a real, mutex-serialized in-memory AccountStore +
// LinkAccountStore double that enforces the actual (platform, platformID)
// uniqueness constraint AddLinkedAccountToDB depends on in production
// (Postgres's linked_accounts_platform_unique index), so two concurrent
// UpdateUserFromPlatform calls for the same identity exercise the same
// real race that a live database would produce.
type usConcurrentStore struct {
	mu       sync.Mutex
	accounts map[string]*Account
	links    map[string]*LinkedAccount

	// barrier forces the first barrierN calls to GetLinkedAccountByPlatformID
	// (one per racing goroutine, for its very first, pre-insert lookup) to
	// all complete before any of them returns. Without this, a fast
	// in-memory fake can let one goroutine run its entire
	// lookup-miss/create/insert sequence to completion before the next
	// goroutine is even scheduled, so the "lost the race" branch this test
	// exists to exercise would rarely, if ever, actually trigger.
	barrierMu    sync.Mutex
	barrierWG    sync.WaitGroup
	barrierN     int
	barrierCount int
}

func newUSConcurrentStore(barrierN int) *usConcurrentStore {
	s := &usConcurrentStore{accounts: map[string]*Account{}, links: map[string]*LinkedAccount{}, barrierN: barrierN}
	s.barrierWG.Add(barrierN)
	return s
}

func usLinkKey(platform Platform, platformID string) string {
	return string(platform) + "|" + platformID
}

func (s *usConcurrentStore) AddAccountToDB(a *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[a.UserID] = a
	return nil
}
func (s *usConcurrentStore) GetAccountByID(userID string) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[userID]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}
func (s *usConcurrentStore) GetAccountByUsername(_ string) (*Account, error) { return nil, ErrNotFound }
func (s *usConcurrentStore) GetAccountByEmail(_ string) (*Account, error)    { return nil, ErrNotFound }
func (s *usConcurrentStore) UpdateAccountInDB(_ *Account) error              { return nil }
func (s *usConcurrentStore) DeleteAccountFromDB(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.accounts, userID)
	return nil
}
func (s *usConcurrentStore) AddLinkedAccountToDB(la *LinkedAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := usLinkKey(la.Platform, la.PlatformID)
	if _, exists := s.links[key]; exists {
		return ErrAlreadyLinked
	}
	cp := *la
	s.links[key] = &cp
	return nil
}
func (s *usConcurrentStore) UpdateLinkedAccount(la *LinkedAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *la
	s.links[usLinkKey(la.Platform, la.PlatformID)] = &cp
	return nil
}
func (s *usConcurrentStore) GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error) {
	s.barrierMu.Lock()
	participates := s.barrierCount < s.barrierN
	if participates {
		s.barrierCount++
	}
	s.barrierMu.Unlock()
	if participates {
		s.barrierWG.Done()
		s.barrierWG.Wait() // rendezvous: hold every racer here until all have missed
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	la, ok := s.links[usLinkKey(platform, platformID)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *la
	return &cp, nil
}
func (s *usConcurrentStore) GetLinkedAccountByPlatformName(_ Platform, _ string) (*LinkedAccount, error) {
	return nil, ErrNotFound
}
func (s *usConcurrentStore) GetLinkedAccountByUserID(_ string, _ Platform) (*LinkedAccount, error) {
	return nil, ErrNotFound
}
func (s *usConcurrentStore) GetLinkedAccountsByUserID(_ string) ([]*LinkedAccount, error) {
	return nil, nil
}
func (s *usConcurrentStore) DeleteLinkedAccount(_ string, _ Platform) error { return nil }
func (s *usConcurrentStore) SetLinkedAccountLoginEnabled(_ string, _ Platform, _ bool) error {
	return nil
}
func (s *usConcurrentStore) accountCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.accounts)
}

// usConcurrentWrapperStore adapts usConcurrentStore to the Store interface
// NewUserService requires.
type usConcurrentWrapperStore struct {
	cs *usConcurrentStore
}

func (w *usConcurrentWrapperStore) Account() AccountStore { return w.cs }
func (w *usConcurrentWrapperStore) AccountSettings() AccountSettingsStore {
	return &usFakeAccountSettingsStore{}
}
func (w *usConcurrentWrapperStore) Session() SessionStore {
	panic("usConcurrentWrapperStore: Session not implemented")
}
func (w *usConcurrentWrapperStore) LinkAccount() LinkAccountStore { return w.cs }
func (w *usConcurrentWrapperStore) RateLimit() RateLimitStore {
	panic("usConcurrentWrapperStore: RateLimit not implemented")
}
func (w *usConcurrentWrapperStore) OAuthToken() OAuthTokenStore {
	panic("usConcurrentWrapperStore: OAuthToken not implemented")
}

// TestUS25UpdateUserFromPlatformConcurrentRace runs many trials, not one:
// the barrier makes a single trial likely to hit the "lost the race" branch,
// but not certain to - the Go scheduler can still let a racer finish its
// whole create+insert sequence before others resume and observe it as
// "found" rather than colliding. A broken recovery guard only fails a
// fraction of individual trials, so looping (mirroring the ST-* concurrency
// rows in store_test.go) is what actually makes this a reliable regression
// check rather than a coin flip.
func TestUS25UpdateUserFromPlatformConcurrentRace(t *testing.T) {
	t.Run("US-25_ConcurrentCallersConvergeOnSameAccount", func(t *testing.T) {
		const trials = 50
		const n = 8
		for trial := 0; trial < trials; trial++ {
			cs := newUSConcurrentStore(n)
			svc := NewUserService(&usConcurrentWrapperStore{cs: cs})

			var wg sync.WaitGroup
			results := make([]*Account, n)
			errs := make([]error, n)
			wg.Add(n)
			for i := 0; i < n; i++ {
				i := i
				go func() {
					defer wg.Done()
					results[i], errs[i] = svc.UpdateUserFromPlatform(PlatformDiscord, "shared-platform-id", usFakePlatformData{username: "alice"})
				}()
			}
			wg.Wait()

			for i, err := range errs {
				if err != nil {
					t.Fatalf("trial %d, goroutine %d: unexpected error: %v", trial, i, err)
				}
			}
			first := results[0].UserID
			for i, a := range results {
				if a.UserID != first {
					t.Fatalf("trial %d: goroutine %d resolved to account %q, want all goroutines to converge on %q", trial, i, a.UserID, first)
				}
			}
			if got := cs.accountCount(); got != 1 {
				t.Fatalf("trial %d: expected exactly one account to exist after the race, got %d", trial, got)
			}
		}
	})
}

func TestUS26to27DeleteUser(t *testing.T) {
	t.Run("US-26_Success", func(t *testing.T) {
		as := &usFakeAccountStore{}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		if err := svc.DeleteUser("u1"); err != nil {
			t.Errorf("DeleteUser() = %v, want nil", err)
		}
		if as.deleteCalls != 1 {
			t.Errorf("expected DeleteAccountFromDB to be called once, got %d", as.deleteCalls)
		}
	})

	t.Run("US-27_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		as := &usFakeAccountStore{deleteErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.DeleteUser("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("DeleteUser() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS28to29GetUserLinkedAccounts(t *testing.T) {
	t.Run("US-28_Success", func(t *testing.T) {
		want := []*LinkedAccount{{UserID: "u1"}}
		als := &usFakeLinkAccountStore{getsByUserID: want}
		svc := usNewService(&usFakeAccountStore{}, als, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserLinkedAccounts("u1")
		if err != nil || len(got) != 1 || got[0] != want[0] {
			t.Errorf("GetUserLinkedAccounts() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-29_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		als := &usFakeLinkAccountStore{getsByUserIDErr: wantErr}
		svc := usNewService(&usFakeAccountStore{}, als, &usFakeAccountSettingsStore{})

		_, err := svc.GetUserLinkedAccounts("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetUserLinkedAccounts() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS30to31UnlinkPlatform(t *testing.T) {
	t.Run("US-30_Success", func(t *testing.T) {
		als := &usFakeLinkAccountStore{}
		svc := usNewService(&usFakeAccountStore{}, als, &usFakeAccountSettingsStore{})

		if err := svc.UnlinkPlatform("u1", PlatformDiscord); err != nil {
			t.Errorf("UnlinkPlatform() = %v, want nil", err)
		}
		if als.deleteCalls != 1 {
			t.Errorf("expected DeleteLinkedAccount to be called once, got %d", als.deleteCalls)
		}
	})

	t.Run("US-31_StoreError", func(t *testing.T) {
		als := &usFakeLinkAccountStore{deleteErr: ErrWouldLockAccount}
		svc := usNewService(&usFakeAccountStore{}, als, &usFakeAccountSettingsStore{})

		err := svc.UnlinkPlatform("u1", PlatformDiscord)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Errorf("UnlinkPlatform() err = %v, want ErrWouldLockAccount", err)
		}
	})
}

func TestUS32to33SetPlatformLoginEnabled(t *testing.T) {
	t.Run("US-32_Success", func(t *testing.T) {
		als := &usFakeLinkAccountStore{}
		svc := usNewService(&usFakeAccountStore{}, als, &usFakeAccountSettingsStore{})

		if err := svc.SetPlatformLoginEnabled("u1", PlatformDiscord, true); err != nil {
			t.Errorf("SetPlatformLoginEnabled() = %v, want nil", err)
		}
		if als.setLoginEnabledCalls != 1 {
			t.Errorf("expected SetLinkedAccountLoginEnabled to be called once, got %d", als.setLoginEnabledCalls)
		}
	})

	t.Run("US-33_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		als := &usFakeLinkAccountStore{setLoginEnabledErr: wantErr}
		svc := usNewService(&usFakeAccountStore{}, als, &usFakeAccountSettingsStore{})

		err := svc.SetPlatformLoginEnabled("u1", PlatformDiscord, true)
		if !errors.Is(err, wantErr) {
			t.Errorf("SetPlatformLoginEnabled() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS34to35GetAccountSettings(t *testing.T) {
	t.Run("US-34_Success", func(t *testing.T) {
		want := &AccountSettings{UserID: "u1", PasswordAuthEnabled: true}
		ass := &usFakeAccountSettingsStore{settings: want}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		got, err := svc.GetAccountSettings("u1")
		if err != nil || got != want {
			t.Errorf("GetAccountSettings() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-35_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		ass := &usFakeAccountSettingsStore{getErr: wantErr}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		_, err := svc.GetAccountSettings("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetAccountSettings() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS36to37SetPasswordAuthEnabled(t *testing.T) {
	t.Run("US-36_Success", func(t *testing.T) {
		ass := &usFakeAccountSettingsStore{}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		if err := svc.SetPasswordAuthEnabled("u1", true); err != nil {
			t.Errorf("SetPasswordAuthEnabled() = %v, want nil", err)
		}
		if ass.setCalls != 1 {
			t.Errorf("expected SetPasswordAuthEnabled to be called once, got %d", ass.setCalls)
		}
	})

	t.Run("US-37_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		ass := &usFakeAccountSettingsStore{setErr: wantErr}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		err := svc.SetPasswordAuthEnabled("u1", true)
		if !errors.Is(err, wantErr) {
			t.Errorf("SetPasswordAuthEnabled() err = %v, want %v", err, wantErr)
		}
	})
}
