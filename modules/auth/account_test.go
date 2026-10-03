package auth

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
)

type acFakeAccountStore struct {
	getByIDAccount    *Account
	getByIDErr        error
	getByIDCalls      int
	getByUserAccount  *Account
	getByUserErr      error
	getByUserCalls    int
	getByEmailAccount *Account
	getByEmailErr     error
	getByEmailCalls   int
	addErr            error
	addCalls          int
	updateErr         error
	updateCalls       int
	deleteErr         error
	deleteCalls       int
}

func (f *acFakeAccountStore) AddAccountToDB(_ *Account) error {
	f.addCalls++
	return f.addErr
}
func (f *acFakeAccountStore) GetAccountByID(_ string) (*Account, error) {
	f.getByIDCalls++
	return f.getByIDAccount, f.getByIDErr
}
func (f *acFakeAccountStore) GetAccountByUsername(_ string) (*Account, error) {
	f.getByUserCalls++
	return f.getByUserAccount, f.getByUserErr
}
func (f *acFakeAccountStore) GetAccountByEmail(_ string) (*Account, error) {
	f.getByEmailCalls++
	return f.getByEmailAccount, f.getByEmailErr
}
func (f *acFakeAccountStore) UpdateAccountInDB(_ *Account) error {
	f.updateCalls++
	return f.updateErr
}
func (f *acFakeAccountStore) DeleteAccountFromDB(_ string) error {
	f.deleteCalls++
	return f.deleteErr
}

type acFakeAccountSettingsStore struct {
	settings *AccountSettings
	err      error
	calls    int
}

func (f *acFakeAccountSettingsStore) GetAccountSettings(_ string) (*AccountSettings, error) {
	f.calls++
	return f.settings, f.err
}
func (f *acFakeAccountSettingsStore) SetPasswordAuthEnabled(_ string, _ bool) error {
	return errors.New("acFakeAccountSettingsStore: SetPasswordAuthEnabled not implemented")
}

type acFakeStore struct {
	as  AccountStore
	ass AccountSettingsStore
}

func (f *acFakeStore) Account() AccountStore                 { return f.as }
func (f *acFakeStore) AccountSettings() AccountSettingsStore { return f.ass }
func (f *acFakeStore) Session() SessionStore                 { panic("acFakeStore: Session not implemented") }
func (f *acFakeStore) LinkAccount() LinkAccountStore {
	panic("acFakeStore: LinkAccount not implemented")
}
func (f *acFakeStore) RateLimit() RateLimitStore   { panic("acFakeStore: RateLimit not implemented") }
func (f *acFakeStore) OAuthToken() OAuthTokenStore { panic("acFakeStore: OAuthToken not implemented") }

func TestAC01NewAccountService(t *testing.T) {
	t.Run("AC-01_WiresGivenSubStores", func(t *testing.T) {
		as := &acFakeAccountStore{getByIDAccount: &Account{UserID: "u1"}}
		ass := &acFakeAccountSettingsStore{}
		svc := NewAccountService(&acFakeStore{as: as, ass: ass}, nil)

		got, err := svc.GetAccountByID("u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != as.getByIDAccount {
			t.Errorf("GetAccountByID did not delegate to the store.Account() instance passed to NewAccountService")
		}
		if as.getByIDCalls != 1 {
			t.Errorf("expected the fake AccountStore to be called once, got %d", as.getByIDCalls)
		}
	})
}

func TestAC02to03GetAccountByID(t *testing.T) {
	t.Run("AC-02_Success", func(t *testing.T) {
		want := &Account{UserID: "u1"}
		as := &acFakeAccountStore{getByIDAccount: want}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		got, err := svc.GetAccountByID("u1")
		if err != nil || got != want {
			t.Errorf("GetAccountByID() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("AC-03_StoreError", func(t *testing.T) {
		as := &acFakeAccountStore{getByIDErr: ErrNotFound}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		_, err := svc.GetAccountByID("u1")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("GetAccountByID() err = %v, want ErrNotFound", err)
		}
	})
}

func TestAC04to05GetAccountByUsername(t *testing.T) {
	t.Run("AC-04_Success", func(t *testing.T) {
		want := &Account{UserID: "u1", Username: "alice"}
		as := &acFakeAccountStore{getByUserAccount: want}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		got, err := svc.GetAccountByUsername("alice")
		if err != nil || got != want {
			t.Errorf("GetAccountByUsername() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("AC-05_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &acFakeAccountStore{getByUserErr: wantErr}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		_, err := svc.GetAccountByUsername("alice")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetAccountByUsername() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC06to07GetAccountByEmail(t *testing.T) {
	t.Run("AC-06_Success", func(t *testing.T) {
		want := &Account{UserID: "u1"}
		as := &acFakeAccountStore{getByEmailAccount: want}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		got, err := svc.GetAccountByEmail("a@b.com")
		if err != nil || got != want {
			t.Errorf("GetAccountByEmail() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("AC-07_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &acFakeAccountStore{getByEmailErr: wantErr}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		_, err := svc.GetAccountByEmail("a@b.com")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetAccountByEmail() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC08to09AddAccount(t *testing.T) {
	t.Run("AC-08_Success", func(t *testing.T) {
		as := &acFakeAccountStore{}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		if err := svc.AddAccount(&Account{UserID: "u1"}); err != nil {
			t.Errorf("AddAccount() = %v, want nil", err)
		}
		if as.addCalls != 1 {
			t.Errorf("expected AddAccountToDB to be called once, got %d", as.addCalls)
		}
	})

	t.Run("AC-09_StoreError", func(t *testing.T) {
		as := &acFakeAccountStore{addErr: ErrUsernameAlreadyExists}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		err := svc.AddAccount(&Account{UserID: "u1"})
		if !errors.Is(err, ErrUsernameAlreadyExists) {
			t.Errorf("AddAccount() err = %v, want ErrUsernameAlreadyExists", err)
		}
	})
}

func TestAC10to11UpdateAccount(t *testing.T) {
	t.Run("AC-10_Success", func(t *testing.T) {
		as := &acFakeAccountStore{}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		if err := svc.UpdateAccount(&Account{UserID: "u1"}); err != nil {
			t.Errorf("UpdateAccount() = %v, want nil", err)
		}
		if as.updateCalls != 1 {
			t.Errorf("expected UpdateAccountInDB to be called once, got %d", as.updateCalls)
		}
	})

	t.Run("AC-11_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &acFakeAccountStore{updateErr: wantErr}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		err := svc.UpdateAccount(&Account{UserID: "u1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateAccount() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC12to13DeleteAccount(t *testing.T) {
	t.Run("AC-12_Success", func(t *testing.T) {
		as := &acFakeAccountStore{}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		if err := svc.DeleteAccount("u1"); err != nil {
			t.Errorf("DeleteAccount() = %v, want nil", err)
		}
		if as.deleteCalls != 1 {
			t.Errorf("expected DeleteAccountFromDB to be called once, got %d", as.deleteCalls)
		}
	})

	t.Run("AC-13_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &acFakeAccountStore{deleteErr: wantErr}
		svc := NewAccountService(&acFakeStore{as: as, ass: &acFakeAccountSettingsStore{}}, nil)

		err := svc.DeleteAccount("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("DeleteAccount() err = %v, want %v", err, wantErr)
		}
	})
}

func TestAC14to16IsPasswordAuthEnabled(t *testing.T) {
	t.Run("AC-14_Enabled", func(t *testing.T) {
		ass := &acFakeAccountSettingsStore{settings: &AccountSettings{UserID: "u1", PasswordAuthEnabled: true}}
		svc := NewAccountService(&acFakeStore{as: &acFakeAccountStore{}, ass: ass}, nil)

		got, err := svc.IsPasswordAuthEnabled("u1")
		if err != nil || got != true {
			t.Errorf("IsPasswordAuthEnabled() = (%v, %v), want (true, nil)", got, err)
		}
	})

	t.Run("AC-15_Disabled", func(t *testing.T) {
		ass := &acFakeAccountSettingsStore{settings: &AccountSettings{UserID: "u1", PasswordAuthEnabled: false}}
		svc := NewAccountService(&acFakeStore{as: &acFakeAccountStore{}, ass: ass}, nil)

		got, err := svc.IsPasswordAuthEnabled("u1")
		if err != nil || got != false {
			t.Errorf("IsPasswordAuthEnabled() = (%v, %v), want (false, nil)", got, err)
		}
	})

	t.Run("AC-16_SettingsLookupError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		ass := &acFakeAccountSettingsStore{err: wantErr}
		svc := NewAccountService(&acFakeStore{as: &acFakeAccountStore{}, ass: ass}, nil)

		got, err := svc.IsPasswordAuthEnabled("u1")
		if got != false || !errors.Is(err, wantErr) {
			t.Errorf("IsPasswordAuthEnabled() = (%v, %v), want (false, %v)", got, err, wantErr)
		}
	})
}

const acBrokenSnowflakeEnv = "AC22_BROKEN_SNOWFLAKE"

func TestAC22NewSessionSnowflakeFails(t *testing.T) {
	if os.Getenv(acBrokenSnowflakeEnv) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAC22NewSessionSnowflakeFails$", "-test.v")
		cmd.Env = append(os.Environ(), acBrokenSnowflakeEnv+"=1", "SNOWFLAKE_NODE_ID=99")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("child test run failed: %v\n%s", err, out)
		}
		return
	}

	t.Run("AC-22_SnowflakeFailureFailsTheSession", func(t *testing.T) {
		svc := NewAccountService(&acFakeStore{as: &acFakeAccountStore{}, ass: &acFakeAccountSettingsStore{}}, rsDefaultRoleStore())

		s, err := svc.NewSession(&Account{UserID: "u1"}, 1)

		if s != nil || err == nil {
			t.Errorf("NewSession() = (%v, %v), want (nil, an error)", s, err)
		}
	})
}

func TestAC17to21NewSession(t *testing.T) {
	newService := func(rs RoleStore) AccountService {
		return NewAccountService(&acFakeStore{as: &acFakeAccountStore{}, ass: &acFakeAccountSettingsStore{}}, rs)
	}

	t.Run("AC-17_UsesTheRoleStorePermissions", func(t *testing.T) {
		rs := rsDefaultRoleStore()
		svc := newService(rs)

		s, err := svc.NewSession(&Account{UserID: "u1", Roles: []string{"1"}}, 12345)

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
		if len(rs.calls) != 1 || len(rs.calls[0]) != 1 || rs.calls[0][0] != "1" {
			t.Errorf("role store calls = %v, want one call with the account's role IDs", rs.calls)
		}
	})

	t.Run("AC-18_AllTheAccountsRolesAreResolvedTogether", func(t *testing.T) {
		rs := rsDefaultRoleStore()
		svc := newService(rs)

		s, err := svc.NewSession(&Account{UserID: "u1", Roles: []string{"1", "2"}}, 1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s.Permissions) != 5 {
			t.Errorf("Permissions = %v, want the permissions of both roles", s.Permissions)
		}
		if len(rs.calls) != 1 || len(rs.calls[0]) != 2 {
			t.Errorf("role store calls = %v, want a single call with both role IDs", rs.calls)
		}
	})

	t.Run("AC-19_NoRolesLooksNothingUp", func(t *testing.T) {
		rs := rsDefaultRoleStore()
		svc := newService(rs)

		s, err := svc.NewSession(&Account{UserID: "u1"}, 1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Permissions == nil || len(s.Permissions) != 0 || len(rs.calls) != 0 {
			t.Errorf("Permissions = %#v, calls = %v, want empty non-nil permissions and no role lookup", s.Permissions, rs.calls)
		}
	})

	t.Run("AC-21_RolesWithoutPermissionsGiveEmptyNonNilPermissions", func(t *testing.T) {
		svc := newService(rsDefaultRoleStore())

		s, err := svc.NewSession(&Account{UserID: "u1", Roles: []string{"99"}}, 1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Permissions == nil || len(s.Permissions) != 0 {
			t.Errorf("Permissions = %#v, want empty and non-nil", s.Permissions)
		}
	})

	t.Run("AC-20_RoleStoreFailureFailsTheSession", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		svc := newService(&rsFakeRoleStore{err: wantErr})

		s, err := svc.NewSession(&Account{UserID: "u1", Roles: []string{"1"}}, 1)

		if s != nil || !errors.Is(err, wantErr) {
			t.Errorf("NewSession() = (%v, %v), want (nil, %v)", s, err, wantErr)
		}
	})
}
