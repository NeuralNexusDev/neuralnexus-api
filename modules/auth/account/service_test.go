package account

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

type acMarkerKey struct{}

type acFakeAccountStore struct {
	getByIDAccount    *Account
	getByIDErr        error
	getByUserAccount  *Account
	getByUserErr      error
	getByUserCalls    int
	getByEmailAccount *Account
	getByEmailErr     error
	getByEmailCalls   int
	addErr            error
	addCalls          int
	addCtxErr         error
	addMarker         any
	updateErr         error
	updateCalls       int
	deleteErr         error
	deleteCalls       int
}

func (f *acFakeAccountStore) AddAccountToDB(ctx context.Context, _ *Account) error {
	f.addCalls++
	f.addCtxErr = ctx.Err()
	f.addMarker = ctx.Value(acMarkerKey{})
	return f.addErr
}
func (f *acFakeAccountStore) GetAccountByID(_ context.Context, _ string) (*Account, error) {
	return f.getByIDAccount, f.getByIDErr
}
func (f *acFakeAccountStore) GetAccountByUsername(_ context.Context, _ string) (*Account, error) {
	f.getByUserCalls++
	return f.getByUserAccount, f.getByUserErr
}
func (f *acFakeAccountStore) GetAccountByEmail(_ context.Context, _ string) (*Account, error) {
	f.getByEmailCalls++
	return f.getByEmailAccount, f.getByEmailErr
}
func (f *acFakeAccountStore) ListAccounts(_ context.Context, _, _ int) ([]*Account, error) {
	return nil, nil
}
func (f *acFakeAccountStore) UpdateAccountInDB(_ context.Context, _ *Account) error {
	f.updateCalls++
	return f.updateErr
}
func (f *acFakeAccountStore) DeleteAccountFromDB(_ context.Context, _ string) error {
	f.deleteCalls++
	return f.deleteErr
}

type acFakeAccountSettingsStore struct {
	settings *AccountSettings
	err      error
	calls    int
}

func (f *acFakeAccountSettingsStore) GetAccountSettings(_ context.Context, _ string) (*AccountSettings, error) {
	f.calls++
	return f.settings, f.err
}
func (f *acFakeAccountSettingsStore) SetPasswordAuthEnabled(_ context.Context, _ string, _ bool) error {
	return errors.New("acFakeAccountSettingsStore: SetPasswordAuthEnabled not implemented")
}

func TestAC02to03GetAccountByID(t *testing.T) {
	t.Run("AC-02_Success", func(t *testing.T) {
		want := &Account{UserID: "u1"}
		as := &acFakeAccountStore{getByIDAccount: want}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		got, err := svc.GetAccountByID(context.Background(), "u1")
		if err != nil || got != want {
			t.Errorf("GetAccountByID() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("AC-03_StoreError", func(t *testing.T) {
		as := &acFakeAccountStore{getByIDErr: ErrNotFound}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		_, err := svc.GetAccountByID(context.Background(), "u1")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("GetAccountByID() err = %v, want ErrNotFound", err)
		}
	})
}

func TestAC04to05GetAccountByUsername(t *testing.T) {
	t.Run("AC-04_Success", func(t *testing.T) {
		want := &Account{UserID: "u1", Username: "alice"}
		as := &acFakeAccountStore{getByUserAccount: want}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		got, err := svc.GetAccountByUsername(context.Background(), "alice")
		if err != nil || got != want {
			t.Errorf("GetAccountByUsername() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("AC-05_StoreError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		as := &acFakeAccountStore{getByUserErr: wantErr}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		_, err := svc.GetAccountByUsername(context.Background(), "alice")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetAccountByUsername() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC06to07GetAccountByEmail(t *testing.T) {
	t.Run("AC-06_Success", func(t *testing.T) {
		want := &Account{UserID: "u1"}
		as := &acFakeAccountStore{getByEmailAccount: want}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		got, err := svc.GetAccountByEmail(context.Background(), "a@b.com")
		if err != nil || got != want {
			t.Errorf("GetAccountByEmail() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("AC-07_StoreError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		as := &acFakeAccountStore{getByEmailErr: wantErr}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		_, err := svc.GetAccountByEmail(context.Background(), "a@b.com")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetAccountByEmail() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC08to09AddAccount(t *testing.T) {
	t.Run("AC-08_Success", func(t *testing.T) {
		as := &acFakeAccountStore{}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		if err := svc.AddAccount(context.Background(), &Account{UserID: "u1"}); err != nil {
			t.Errorf("AddAccount() = %v, want nil", err)
		}
		if as.addCalls != 1 {
			t.Errorf("expected AddAccountToDB to be called once, got %d", as.addCalls)
		}
	})

	t.Run("AC-09_StoreError", func(t *testing.T) {
		as := &acFakeAccountStore{addErr: ErrUsernameAlreadyExists}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		err := svc.AddAccount(context.Background(), &Account{UserID: "u1"})
		if !errors.Is(err, ErrUsernameAlreadyExists) {
			t.Errorf("AddAccount() err = %v, want ErrUsernameAlreadyExists", err)
		}
	})
}

func TestAC24AddAccountIgnoresCancelledContext(t *testing.T) {
	t.Run("AC-24_AddAccountToDBRunsOnAnUncancelledContext", func(t *testing.T) {
		as := &acFakeAccountStore{}
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), acMarkerKey{}, "m1"))
		cancel()

		if err := NewService(as, &acFakeAccountSettingsStore{}, nil).AddAccount(ctx, &Account{UserID: "u1"}); err != nil {
			t.Fatalf("AddAccount() = %v, want nil", err)
		}
		if as.addCtxErr != nil {
			t.Errorf("AddAccountToDB context Err() = %v, want nil", as.addCtxErr)
		}
		if as.addMarker != "m1" {
			t.Errorf("AddAccountToDB context marker = %v, want %q", as.addMarker, "m1")
		}
	})
}

func TestAC10to11UpdateAccount(t *testing.T) {
	t.Run("AC-10_Success", func(t *testing.T) {
		as := &acFakeAccountStore{}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		if err := svc.UpdateAccount(context.Background(), &Account{UserID: "u1"}); err != nil {
			t.Errorf("UpdateAccount() = %v, want nil", err)
		}
		if as.updateCalls != 1 {
			t.Errorf("expected UpdateAccountInDB to be called once, got %d", as.updateCalls)
		}
	})

	t.Run("AC-11_StoreError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		as := &acFakeAccountStore{updateErr: wantErr}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		err := svc.UpdateAccount(context.Background(), &Account{UserID: "u1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateAccount() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC12to13DeleteAccount(t *testing.T) {
	t.Run("AC-12_Success", func(t *testing.T) {
		as := &acFakeAccountStore{}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		if err := svc.DeleteAccount(context.Background(), "u1"); err != nil {
			t.Errorf("DeleteAccount() = %v, want nil", err)
		}
		if as.deleteCalls != 1 {
			t.Errorf("expected DeleteAccountFromDB to be called once, got %d", as.deleteCalls)
		}
	})

	t.Run("AC-13_StoreError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		as := &acFakeAccountStore{deleteErr: wantErr}
		svc := NewService(as, &acFakeAccountSettingsStore{}, nil)

		err := svc.DeleteAccount(context.Background(), "u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("DeleteAccount() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC14to16IsPasswordAuthEnabled(t *testing.T) {
	t.Run("AC-14_Enabled", func(t *testing.T) {
		ass := &acFakeAccountSettingsStore{settings: &AccountSettings{UserID: "u1", PasswordAuthEnabled: true}}
		svc := NewService(&acFakeAccountStore{}, ass, nil)

		got, err := svc.IsPasswordAuthEnabled(context.Background(), "u1")
		if err != nil || got != true {
			t.Errorf("IsPasswordAuthEnabled() = (%v, %v), want (true, nil)", got, err)
		}
	})

	t.Run("AC-15_Disabled", func(t *testing.T) {
		ass := &acFakeAccountSettingsStore{settings: &AccountSettings{UserID: "u1", PasswordAuthEnabled: false}}
		svc := NewService(&acFakeAccountStore{}, ass, nil)

		got, err := svc.IsPasswordAuthEnabled(context.Background(), "u1")
		if err != nil || got != false {
			t.Errorf("IsPasswordAuthEnabled() = (%v, %v), want (false, nil)", got, err)
		}
	})

	t.Run("AC-16_SettingsLookupError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		ass := &acFakeAccountSettingsStore{err: wantErr}
		svc := NewService(&acFakeAccountStore{}, ass, nil)

		got, err := svc.IsPasswordAuthEnabled(context.Background(), "u1")
		if got != false || !errors.Is(err, wantErr) {
			t.Errorf("IsPasswordAuthEnabled() = (%v, %v), want (false, %v)", got, err, wantErr)
		}
	})
}

func TestAC17to21NewSession(t *testing.T) {
	newService := func(rs RoleStore) Service {
		return NewService(&acFakeAccountStore{}, &acFakeAccountSettingsStore{}, rs)
	}

	t.Run("AC-17_UsesTheRoleStorePermissions", func(t *testing.T) {
		rs := testutil.DefaultRoleStore()
		svc := newService(rs)

		s, err := svc.NewSession(context.Background(), &Account{UserID: "u1", Roles: []string{"1"}}, 12345)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.UserID != "u1" || s.ExpiresAt != 12345 {
			t.Errorf("session = %+v, want UserID u1 and ExpiresAt 12345", s)
		}
		if _, err := strconv.ParseInt(s.ID, 10, 64); err != nil {
			t.Errorf("session ID %q is not a snowflake: %v", s.ID, err)
		}
		if now := time.Now().Unix(); s.IssuedAt < now-5 || s.IssuedAt > now+5 || s.LastUsedAt != s.IssuedAt {
			t.Errorf("IssuedAt = %d and LastUsedAt = %d, want both set to now (%d)", s.IssuedAt, s.LastUsedAt, now)
		}
		want := []string{"beenamegenerator.admin", "petpictures.admin", "ratelimit:1000"}
		if len(s.Permissions) != len(want) {
			t.Fatalf("Permissions = %v, want %v", s.Permissions, want)
		}
		for i := range want {
			if s.Permissions[i] != want[i] {
				t.Errorf("Permissions[%d] = %q, want %q", i, s.Permissions[i], want[i])
			}
		}
		if len(rs.Calls) != 1 || len(rs.Calls[0]) != 1 || rs.Calls[0][0] != "1" {
			t.Errorf("role store calls = %v, want one call with the account's role IDs", rs.Calls)
		}
	})

	t.Run("AC-18_AllTheAccountsRolesAreResolvedTogether", func(t *testing.T) {
		rs := testutil.DefaultRoleStore()
		svc := newService(rs)

		s, err := svc.NewSession(context.Background(), &Account{UserID: "u1", Roles: []string{"1", "2"}}, 1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s.Permissions) != 5 {
			t.Errorf("Permissions = %v, want the permissions of both roles", s.Permissions)
		}
		if len(rs.Calls) != 1 || len(rs.Calls[0]) != 2 {
			t.Errorf("role store calls = %v, want a single call with both role IDs", rs.Calls)
		}
	})

	t.Run("AC-19_NoRolesLooksNothingUp", func(t *testing.T) {
		rs := testutil.DefaultRoleStore()
		svc := newService(rs)

		s, err := svc.NewSession(context.Background(), &Account{UserID: "u1"}, 1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Permissions == nil || len(s.Permissions) != 0 || len(rs.Calls) != 0 {
			t.Errorf("Permissions = %#v, calls = %v, want empty non-nil permissions and no role lookup", s.Permissions, rs.Calls)
		}
	})

	t.Run("AC-21_RolesWithoutPermissionsGiveEmptyNonNilPermissions", func(t *testing.T) {
		svc := newService(testutil.DefaultRoleStore())

		s, err := svc.NewSession(context.Background(), &Account{UserID: "u1", Roles: []string{"99"}}, 1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Permissions == nil || len(s.Permissions) != 0 {
			t.Errorf("Permissions = %#v, want empty and non-nil", s.Permissions)
		}
	})

	t.Run("AC-20_RoleStoreFailureFailsTheSession", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		svc := newService(&testutil.FakeRoleStore{Err: wantErr})

		s, err := svc.NewSession(context.Background(), &Account{UserID: "u1", Roles: []string{"1"}}, 1)

		if s != nil || !errors.Is(err, wantErr) {
			t.Errorf("NewSession() = (%v, %v), want (nil, %v)", s, err, wantErr)
		}
	})
}
