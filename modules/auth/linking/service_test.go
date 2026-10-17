package linking

import (
	"errors"
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
