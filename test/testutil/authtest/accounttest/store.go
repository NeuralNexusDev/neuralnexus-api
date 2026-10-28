// Package accounttest holds fakes of the account package's interfaces. It imports account, so only tests of packages that import account may use it.
package accounttest

import (
	"context"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
)

// FakeStore is an account.Store that returns configured results and records its calls.
type FakeStore struct {
	// AddErr is returned by AddAccountToDB.
	AddErr error
	// AddCalls counts AddAccountToDB calls.
	AddCalls int
	// AddLastAccount is the account passed to the last AddAccountToDB call.
	AddLastAccount *account.Account
	// AddCtxErr is the ctx.Err() seen by the last AddAccountToDB call.
	AddCtxErr error
	// GetByIDAccount is returned by GetAccountByID.
	GetByIDAccount *account.Account
	// GetByIDErr is returned by GetAccountByID.
	GetByIDErr error
	// GetByIDCalls counts GetAccountByID calls.
	GetByIDCalls int
	// GetByIDLastUserID is the user ID passed to the last GetAccountByID call.
	GetByIDLastUserID string
	// ListResult is returned by ListAccounts.
	ListResult []*account.Account
	// ListErr is returned by ListAccounts.
	ListErr error
	// ListCalls counts ListAccounts calls.
	ListCalls int
	// ListLastLimit is the limit passed to the last ListAccounts call.
	ListLastLimit int
	// ListLastOffset is the offset passed to the last ListAccounts call.
	ListLastOffset int
	// UpdateErr is returned by UpdateAccountInDB.
	UpdateErr error
	// UpdateCalls counts UpdateAccountInDB calls.
	UpdateCalls int
	// UpdateLastAccount is the account passed to the last UpdateAccountInDB call.
	UpdateLastAccount *account.Account
	// DeleteErr is returned by DeleteAccountFromDB.
	DeleteErr error
	// DeleteCalls counts DeleteAccountFromDB calls.
	DeleteCalls int
	// DeleteLastUserID is the user ID passed to the last DeleteAccountFromDB call.
	DeleteLastUserID string
	// DeleteCtxErr is the ctx.Err() seen by the last DeleteAccountFromDB call.
	DeleteCtxErr error
}

var _ account.Store = (*FakeStore)(nil)

// AddAccountToDB records the call and returns AddErr.
func (f *FakeStore) AddAccountToDB(ctx context.Context, a *account.Account) error {
	f.AddCtxErr = ctx.Err()
	f.AddCalls++
	f.AddLastAccount = a
	return f.AddErr
}

// GetAccountByID records the call and returns GetByIDAccount and GetByIDErr.
func (f *FakeStore) GetAccountByID(_ context.Context, userID string) (*account.Account, error) {
	f.GetByIDCalls++
	f.GetByIDLastUserID = userID
	return f.GetByIDAccount, f.GetByIDErr
}

// GetAccountByUsername returns account.ErrNotFound.
func (f *FakeStore) GetAccountByUsername(_ context.Context, _ string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

// GetAccountByEmail returns account.ErrNotFound.
func (f *FakeStore) GetAccountByEmail(_ context.Context, _ string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

// ListAccounts records the call and returns ListResult and ListErr.
func (f *FakeStore) ListAccounts(_ context.Context, limit, offset int) ([]*account.Account, error) {
	f.ListCalls++
	f.ListLastLimit, f.ListLastOffset = limit, offset
	return f.ListResult, f.ListErr
}

// UpdateAccountInDB records the call and returns UpdateErr.
func (f *FakeStore) UpdateAccountInDB(_ context.Context, a *account.Account) error {
	f.UpdateCalls++
	f.UpdateLastAccount = a
	return f.UpdateErr
}

// DeleteAccountFromDB records the call and returns DeleteErr.
func (f *FakeStore) DeleteAccountFromDB(ctx context.Context, userID string) error {
	f.DeleteCtxErr = ctx.Err()
	f.DeleteCalls++
	f.DeleteLastUserID = userID
	return f.DeleteErr
}
