package link

import (
	"context"
	"sync"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
)

type inertLinkStore struct{}

func (inertLinkStore) UpdateLinkedAccount(context.Context, *LinkedAccount) error { return nil }

func (inertLinkStore) GetLinkedAccountByPlatformName(context.Context, Platform, string) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (inertLinkStore) GetLinkedAccountByUserID(context.Context, string, Platform) (*LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (inertLinkStore) GetLinkedAccountsByUserID(context.Context, string) ([]*LinkedAccount, error) {
	return nil, nil
}

func (inertLinkStore) DeleteLinkedAccount(context.Context, string, Platform) error { return nil }

func (inertLinkStore) SetLinkedAccountLoginEnabled(context.Context, string, Platform, bool) error {
	return nil
}

func linkKey(platform Platform, platformID string) string {
	return string(platform) + "|" + platformID
}

// concurrentLinkStore enforces the (platform, platform_id) uniqueness
// constraint atomically under a mutex, so goroutines launched against it
// race for real. GetLinkedAccountByPlatformID holds the first barrierN callers
// at a barrier until all have read, so the check-then-act window overlaps
// deterministically instead of only occasionally.
type concurrentLinkStore struct {
	inertLinkStore
	mu    sync.Mutex
	byKey map[string]*LinkedAccount

	barrierMu    sync.Mutex
	barrierWG    sync.WaitGroup
	barrierN     int
	barrierCount int
}

func newConcurrentLinkStore(barrierN int) *concurrentLinkStore {
	s := &concurrentLinkStore{byKey: map[string]*LinkedAccount{}, barrierN: barrierN}
	s.barrierWG.Add(barrierN)
	return s
}

func (s *concurrentLinkStore) AddLinkedAccountToDB(_ context.Context, la *LinkedAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := linkKey(la.Platform, la.PlatformID)
	if _, exists := s.byKey[key]; exists {
		return ErrAlreadyLinked
	}
	cp := *la
	s.byKey[key] = &cp
	return nil
}

func (s *concurrentLinkStore) UpdateLinkedAccount(_ context.Context, la *LinkedAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *la
	s.byKey[linkKey(la.Platform, la.PlatformID)] = &cp
	return nil
}

func (s *concurrentLinkStore) GetLinkedAccountByPlatformID(_ context.Context, platform Platform, platformID string) (*LinkedAccount, error) {
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
	la, ok := s.byKey[linkKey(platform, platformID)]
	if !ok {
		return nil, account.ErrNotFound
	}
	cp := *la
	return &cp, nil
}

type inertAccountService struct{}

func (inertAccountService) GetAccountByUsername(context.Context, string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

func (inertAccountService) GetAccountByEmail(context.Context, string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

func (inertAccountService) IsPasswordAuthEnabled(context.Context, string) (bool, error) {
	return false, nil
}

func (inertAccountService) NewSession(_ context.Context, a *account.Account, expiresAt int64) (*session.Session, error) {
	return &session.Session{ID: "stub-session", UserID: a.UserID, Permissions: []string{"stub:perm"}, ExpiresAt: expiresAt}, nil
}

type fakePlatformData struct {
	id, email, username, data string
}

func (f *fakePlatformData) GetID() string       { return f.id }
func (f *fakePlatformData) GetEmail() string    { return f.email }
func (f *fakePlatformData) GetUsername() string { return f.username }
func (f *fakePlatformData) GetData() string {
	if f.data != "" {
		return f.data
	}
	return "{}"
}
func (f *fakePlatformData) CreateLinkedAccount(userID string) *LinkedAccount {
	return NewLinkedAccount(userID, PlatformDiscord, f.username, f.id, f)
}
