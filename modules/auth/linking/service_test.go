package linking

import (
	"errors"
	"sync"
	"testing"

	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
)

type usLinkResult struct {
	la  *LinkedAccount
	err error
}

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
	return nil, authaccount.ErrNotFound
}

func (f *usFakeLinkAccountStore) GetLinkedAccountByUserID(_ string, _ Platform) (*LinkedAccount, error) {
	return nil, authaccount.ErrNotFound
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

func TestUS28to29GetUserLinkedAccounts(t *testing.T) {
	t.Run("US-28_Success", func(t *testing.T) {
		want := []*LinkedAccount{{UserID: "u1"}}
		als := &usFakeLinkAccountStore{getsByUserID: want}
		svc := NewService(als, nil)

		got, err := svc.GetUserLinkedAccounts("u1")
		if err != nil || len(got) != 1 || got[0] != want[0] {
			t.Errorf("GetUserLinkedAccounts() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-29_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		als := &usFakeLinkAccountStore{getsByUserIDErr: wantErr}
		svc := NewService(als, nil)

		_, err := svc.GetUserLinkedAccounts("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetUserLinkedAccounts() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS30to31UnlinkPlatform(t *testing.T) {
	t.Run("US-30_Success", func(t *testing.T) {
		als := &usFakeLinkAccountStore{}
		svc := NewService(als, nil)

		if err := svc.UnlinkPlatform("u1", PlatformDiscord); err != nil {
			t.Errorf("UnlinkPlatform() = %v, want nil", err)
		}
		if als.deleteCalls != 1 {
			t.Errorf("expected DeleteLinkedAccount to be called once, got %d", als.deleteCalls)
		}
	})

	t.Run("US-31_StoreError", func(t *testing.T) {
		als := &usFakeLinkAccountStore{deleteErr: authaccount.ErrWouldLockAccount}
		svc := NewService(als, nil)

		err := svc.UnlinkPlatform("u1", PlatformDiscord)
		if !errors.Is(err, authaccount.ErrWouldLockAccount) {
			t.Errorf("UnlinkPlatform() err = %v, want ErrWouldLockAccount", err)
		}
	})
}

func TestUS32to33SetPlatformLoginEnabled(t *testing.T) {
	t.Run("US-32_Success", func(t *testing.T) {
		als := &usFakeLinkAccountStore{}
		svc := NewService(als, nil)

		if err := svc.SetPlatformLoginEnabled("u1", PlatformDiscord, true); err != nil {
			t.Errorf("SetPlatformLoginEnabled() = %v, want nil", err)
		}
		if als.setLoginEnabledCalls != 1 {
			t.Errorf("expected SetLinkedAccountLoginEnabled to be called once, got %d", als.setLoginEnabledCalls)
		}
	})

	t.Run("US-33_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		als := &usFakeLinkAccountStore{setLoginEnabledErr: wantErr}
		svc := NewService(als, nil)

		err := svc.SetPlatformLoginEnabled("u1", PlatformDiscord, true)
		if !errors.Is(err, wantErr) {
			t.Errorf("SetPlatformLoginEnabled() err = %v, want %v", err, wantErr)
		}
	})
}

type usFakePlatformData struct {
	username string
}

func (f usFakePlatformData) GetID() string { return "pid" }

func (f usFakePlatformData) GetEmail() string { return "" }

func (f usFakePlatformData) GetUsername() string { return f.username }

func (f usFakePlatformData) GetData() string { return "{}" }

func (f usFakePlatformData) CreateLinkedAccount(_ string) *LinkedAccount {
	return nil
}

type usFakeAccountStore struct {
	addErr         error
	addCalls       int
	addLastAccount *authaccount.Account

	getByIDAccount    *authaccount.Account
	getByIDErr        error
	getByIDCalls      int
	getByIDLastUserID string

	listAccounts   []*authaccount.Account
	listErr        error
	listCalls      int
	listLastLimit  int
	listLastOffset int

	updateErr         error
	updateCalls       int
	updateLastAccount *authaccount.Account

	deleteErr        error
	deleteCalls      int
	deleteLastUserID string
}

func (f *usFakeAccountStore) AddAccountToDB(a *authaccount.Account) error {
	f.addCalls++
	f.addLastAccount = a
	return f.addErr
}

func (f *usFakeAccountStore) DeleteAccountFromDB(userID string) error {
	f.deleteCalls++
	f.deleteLastUserID = userID
	return f.deleteErr
}

func (f *usFakeAccountStore) GetAccountByID(userID string) (*authaccount.Account, error) {
	f.getByIDCalls++
	f.getByIDLastUserID = userID
	return f.getByIDAccount, f.getByIDErr
}

func (f *usFakeAccountStore) GetAccountByUsername(_ string) (*authaccount.Account, error) {
	return nil, authaccount.ErrNotFound
}

func (f *usFakeAccountStore) GetAccountByEmail(_ string) (*authaccount.Account, error) {
	return nil, authaccount.ErrNotFound
}

func (f *usFakeAccountStore) ListAccounts(_, _ int) ([]*authaccount.Account, error) {
	f.listCalls++
	return f.listAccounts, f.listErr
}

func (f *usFakeAccountStore) UpdateAccountInDB(a *authaccount.Account) error {
	f.updateCalls++
	f.updateLastAccount = a
	return f.updateErr
}

type usConcurrentStore struct {
	mu       sync.Mutex
	accounts map[string]*authaccount.Account
	links    map[string]*LinkedAccount

	barrierMu    sync.Mutex
	barrierWG    sync.WaitGroup
	barrierN     int
	barrierCount int
}

func newUSConcurrentStore(barrierN int) *usConcurrentStore {
	s := &usConcurrentStore{accounts: map[string]*authaccount.Account{}, links: map[string]*LinkedAccount{}, barrierN: barrierN}
	s.barrierWG.Add(barrierN)
	return s
}

func usLinkKey(platform Platform, platformID string) string {
	return string(platform) + "|" + platformID
}

func (s *usConcurrentStore) AddAccountToDB(a *authaccount.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[a.UserID] = a
	return nil
}

func (s *usConcurrentStore) GetAccountByID(userID string) (*authaccount.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[userID]
	if !ok {
		return nil, authaccount.ErrNotFound
	}
	return a, nil
}

func (s *usConcurrentStore) GetAccountByUsername(_ string) (*authaccount.Account, error) {
	return nil, authaccount.ErrNotFound
}

func (s *usConcurrentStore) GetAccountByEmail(_ string) (*authaccount.Account, error) {
	return nil, authaccount.ErrNotFound
}

func (s *usConcurrentStore) ListAccounts(_, _ int) ([]*authaccount.Account, error) { return nil, nil }

func (s *usConcurrentStore) UpdateAccountInDB(_ *authaccount.Account) error { return nil }

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
		s.barrierWG.Wait()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	la, ok := s.links[usLinkKey(platform, platformID)]
	if !ok {
		return nil, authaccount.ErrNotFound
	}
	cp := *la
	return &cp, nil
}

func (s *usConcurrentStore) GetLinkedAccountByPlatformName(_ Platform, _ string) (*LinkedAccount, error) {
	return nil, authaccount.ErrNotFound
}

func (s *usConcurrentStore) GetLinkedAccountByUserID(_ string, _ Platform) (*LinkedAccount, error) {
	return nil, authaccount.ErrNotFound
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

func TestUS15to23and38UpdateFromPlatform(t *testing.T) {
	t.Run("US-15_NewIdentityCreatesAccount", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "resolved"}}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}}}
		svc := NewService(als, as)

		got, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{username: "alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != as.addLastAccount.UserID {
			t.Errorf("UpdateFromPlatform() = %v, want %v", got, as.addLastAccount.UserID)
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
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "existing-user"}}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{existing, nil}}}
		svc := NewService(als, as)

		got, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{username: "alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if as.addCalls != 0 {
			t.Error("expected AddAccountToDB to never be called for an already-linked identity")
		}
		if als.updateCalls != 1 || als.updateLastLinkedAccount.UserID != "existing-user" {
			t.Errorf("expected UpdateLinkedAccount to be called once for the existing link, got calls=%d last=%+v", als.updateCalls, als.updateLastLinkedAccount)
		}
		if got != "existing-user" {
			t.Errorf("UpdateFromPlatform() = %v, want %v", got, "existing-user")
		}
	})
	t.Run("US-17_InitialLookupUnexpectedError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, wantErr}}}
		svc := NewService(als, as)

		_, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateFromPlatform() err = %v, want %v", err, wantErr)
		}
		if as.addCalls != 0 {
			t.Error("expected no account creation attempt on an unexpected lookup error")
		}
	})
	t.Run("US-18_AddAccountFails", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{addErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}}}
		svc := NewService(als, as)

		_, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateFromPlatform() err = %v, want %v", err, wantErr)
		}
		if als.addCalls != 0 {
			t.Error("expected AddLinkedAccountToDB to never be called when AddAccountToDB fails")
		}
	})
	t.Run("US-19_AddLinkFailsCleansUpPlaceholder", func(t *testing.T) {
		wantErr := testerrors.ErrInsertFailed
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}}, addErr: wantErr}
		svc := NewService(als, as)

		_, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateFromPlatform() err = %v, want %v", err, wantErr)
		}
		if as.deleteCalls != 1 || as.deleteLastUserID != as.addLastAccount.UserID {
			t.Errorf("expected DeleteAccountFromDB to clean up the orphaned placeholder %q, got calls=%d last=%q", as.addLastAccount.UserID, as.deleteCalls, as.deleteLastUserID)
		}
	})
	t.Run("US-20_AddLinkFailsAndCleanupFailsReturnsTheCleanupError", func(t *testing.T) {
		linkErr := testerrors.ErrInsertFailed
		deleteErr := testerrors.ErrDBDown
		as := &usFakeAccountStore{deleteErr: deleteErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}}, addErr: linkErr}
		svc := NewService(als, as)

		_, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if err == nil {
			t.Fatal("expected a non-nil error")
		}
		if !errors.Is(err, deleteErr) {
			t.Errorf("UpdateFromPlatform() err = %v, want the cleanup error %v", err, deleteErr)
		}
	})
	t.Run("US-21_LostRaceUsesWinner", func(t *testing.T) {
		winner := &LinkedAccount{UserID: "winner-user", Platform: PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "winner-user"}}
		als := &usFakeLinkAccountStore{
			getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}, {winner, nil}},
			addErr:                 ErrAlreadyLinked,
		}
		svc := NewService(als, as)

		got, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "winner-user" {
			t.Errorf("UpdateFromPlatform() = %v, want the winner's user ID %v", got, "winner-user")
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
		wantErr := testerrors.ErrDBDown
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{
			getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}, {nil, wantErr}},
			addErr:                 ErrAlreadyLinked,
		}
		svc := NewService(als, as)

		_, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateFromPlatform() err = %v, want %v", err, wantErr)
		}
	})
	t.Run("US-23_UpdateLinkedAccountFails", func(t *testing.T) {
		wantErr := testerrors.ErrDBDown
		existing := &LinkedAccount{UserID: "u1", Platform: PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{existing, nil}}, updateErr: wantErr}
		svc := NewService(als, as)

		_, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateFromPlatform() err = %v, want %v", err, wantErr)
		}
	})
	t.Run("US-38_AddLinkAndCleanupFailReturnsTheCleanupError", func(t *testing.T) {
		as := &usFakeAccountStore{deleteErr: testerrors.ErrBoom}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}}, addErr: testerrors.ErrInsertFailed}
		svc := NewService(als, as)

		_, err := svc.UpdateFromPlatform(PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("UpdateFromPlatform() err = %v, want the cleanup error %v", err, testerrors.ErrBoom)
		}
	})
}

func TestUS25UpdateFromPlatformConcurrentRace(t *testing.T) {
	t.Run("US-25_ConcurrentCallersConvergeOnSameAccount", func(t *testing.T) {
		const trials = 50
		const n = 8
		for trial := 0; trial < trials; trial++ {
			cs := newUSConcurrentStore(n)
			svc := NewService(cs, cs)

			var wg sync.WaitGroup
			results := make([]string, n)
			errs := make([]error, n)
			wg.Add(n)
			for i := 0; i < n; i++ {
				i := i
				go func() {
					defer wg.Done()
					results[i], errs[i] = svc.UpdateFromPlatform(PlatformDiscord, "shared-platform-id", usFakePlatformData{username: "alice"})
				}()
			}
			wg.Wait()

			for i, err := range errs {
				if err != nil {
					t.Fatalf("trial %d, goroutine %d: unexpected error: %v", trial, i, err)
				}
			}
			first := results[0]
			for i, id := range results {
				if id != first {
					t.Fatalf("trial %d: goroutine %d resolved to user %q, want all goroutines to converge on %q", trial, i, id, first)
				}
			}
			if got := cs.accountCount(); got != 1 {
				t.Fatalf("trial %d: expected exactly one account to exist after the race, got %d", trial, got)
			}
		}
	})
}
