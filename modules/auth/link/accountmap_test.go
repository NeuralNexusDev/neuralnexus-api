package link

import (
	"context"
	"sync"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
)

type accountMap struct {
	mu       sync.Mutex
	accounts map[string]*account.Account
}

func newAccountMap() *accountMap {
	return &accountMap{accounts: map[string]*account.Account{}}
}

func (m *accountMap) AddAccount(_ context.Context, a *account.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.UserID] = a
	return nil
}

func (m *accountMap) UpdateAccount(ctx context.Context, a *account.Account) error {
	return m.AddAccount(ctx, a)
}

func (m *accountMap) GetAccountByID(_ context.Context, userID string) (*account.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[userID]
	if !ok {
		return nil, account.ErrNotFound
	}
	return a, nil
}

func (m *accountMap) DeleteAccount(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.accounts, userID)
	return nil
}

func (m *accountMap) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.accounts)
}
