package auth

import (
	"errors"
	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
)

type usFakePlatformData struct {
	username string
}

func (f usFakePlatformData) GetID() string { return "pid" }

func (f usFakePlatformData) GetEmail() string { return "" }

func (f usFakePlatformData) GetUsername() string { return f.username }

func (f usFakePlatformData) GetData() string { return "{}" }

func (f usFakePlatformData) CreateLinkedAccount(_ string) *linking.LinkedAccount {
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

func (f *usFakeAccountStore) ListAccounts(limit, offset int) ([]*authaccount.Account, error) {
	f.listCalls++
	f.listLastLimit, f.listLastOffset = limit, offset
	return f.listAccounts, f.listErr
}

func (f *usFakeAccountStore) UpdateAccountInDB(a *authaccount.Account) error {
	f.updateCalls++
	f.updateLastAccount = a
	return f.updateErr
}

func (f *usFakeAccountStore) DeleteAccountFromDB(userID string) error {
	f.deleteCalls++
	f.deleteLastUserID = userID
	return f.deleteErr
}

type usLinkResult struct {
	la  *linking.LinkedAccount
	err error
}

type usFakeLinkAccountStore struct {
	getByPlatformIDResults []usLinkResult
	getByPlatformIDCalls   int

	addErr               error
	addCalls             int
	addLastLinkedAccount *linking.LinkedAccount

	updateErr               error
	updateCalls             int
	updateLastLinkedAccount *linking.LinkedAccount

	getsByUserID    []*linking.LinkedAccount
	getsByUserIDErr error

	deleteErr   error
	deleteCalls int

	setLoginEnabledErr   error
	setLoginEnabledCalls int
}

func (f *usFakeLinkAccountStore) AddLinkedAccountToDB(la *linking.LinkedAccount) error {
	f.addCalls++
	f.addLastLinkedAccount = la
	return f.addErr
}

func (f *usFakeLinkAccountStore) UpdateLinkedAccount(la *linking.LinkedAccount) error {
	f.updateCalls++
	f.updateLastLinkedAccount = la
	return f.updateErr
}

func (f *usFakeLinkAccountStore) GetLinkedAccountByPlatformID(_ linking.Platform, _ string) (*linking.LinkedAccount, error) {
	i := f.getByPlatformIDCalls
	f.getByPlatformIDCalls++
	if i < len(f.getByPlatformIDResults) {
		r := f.getByPlatformIDResults[i]
		return r.la, r.err
	}
	last := f.getByPlatformIDResults[len(f.getByPlatformIDResults)-1]
	return last.la, last.err
}

func (f *usFakeLinkAccountStore) GetLinkedAccountByPlatformName(_ linking.Platform, _ string) (*linking.LinkedAccount, error) {
	return nil, authaccount.ErrNotFound
}

func (f *usFakeLinkAccountStore) GetLinkedAccountByUserID(_ string, _ linking.Platform) (*linking.LinkedAccount, error) {
	return nil, authaccount.ErrNotFound
}

func (f *usFakeLinkAccountStore) GetLinkedAccountsByUserID(_ string) ([]*linking.LinkedAccount, error) {
	return f.getsByUserID, f.getsByUserIDErr
}

func (f *usFakeLinkAccountStore) DeleteLinkedAccount(_ string, _ linking.Platform) error {
	f.deleteCalls++
	return f.deleteErr
}

func (f *usFakeLinkAccountStore) SetLinkedAccountLoginEnabled(_ string, _ linking.Platform, _ bool) error {
	f.setLoginEnabledCalls++
	return f.setLoginEnabledErr
}

type usFakeAccountSettingsStore struct {
	settings *authaccount.AccountSettings
	getErr   error
	setErr   error
	setCalls int
}

func (f *usFakeAccountSettingsStore) GetAccountSettings(_ string) (*authaccount.AccountSettings, error) {
	return f.settings, f.getErr
}

func (f *usFakeAccountSettingsStore) SetPasswordAuthEnabled(_ string, _ bool) error {
	f.setCalls++
	return f.setErr
}

func usNewService(as *usFakeAccountStore, als *usFakeLinkAccountStore, ass *usFakeAccountSettingsStore) UserService {
	return NewUserService(as, als, linking.NewService(als, as), ass, rsDefaultRoleStore())
}

func TestUS01NewUserService(t *testing.T) {
	t.Run("US-01_WiresGivenSubStores", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1"}}
		als := &usFakeLinkAccountStore{}
		ass := &usFakeAccountSettingsStore{}
		svc := usNewService(as, als, ass)

		if _, err := svc.GetUser("u1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if as.getByIDCalls != 1 {
			t.Errorf("expected GetUser to delegate to the AccountStore passed to NewUserService")
		}
	})
}

func TestUS02to03GetUser(t *testing.T) {
	t.Run("US-02_Success", func(t *testing.T) {
		want := &authaccount.Account{UserID: "u1"}
		svc := usNewService(&usFakeAccountStore{getByIDAccount: want}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUser("u1")
		if err != nil || got != want {
			t.Errorf("GetUser() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-03_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		svc := usNewService(&usFakeAccountStore{getByIDErr: wantErr}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		_, err := svc.GetUser("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetUser() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS43to44ListUsers(t *testing.T) {
	t.Run("US-43_PassesThePageToTheStore", func(t *testing.T) {
		want := []*authaccount.Account{{UserID: "u1"}, {UserID: "u2"}}
		as := &usFakeAccountStore{listAccounts: want}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.ListUsers(25, 50)
		if err != nil || len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("ListUsers() = (%v, %v), want (%v, nil)", got, err, want)
		}
		if as.listCalls != 1 || as.listLastLimit != 25 || as.listLastOffset != 50 {
			t.Errorf("store called %d times with (%d, %d), want once with (25, 50)", as.listCalls, as.listLastLimit, as.listLastOffset)
		}
	})

	t.Run("US-44_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		svc := usNewService(&usFakeAccountStore{listErr: wantErr}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.ListUsers(10, 0)
		if !errors.Is(err, wantErr) || got != nil {
			t.Errorf("ListUsers() = (%v, %v), want (nil, %v)", got, err, wantErr)
		}
	})
}

func TestUS04to06GetUserFromPlatform(t *testing.T) {
	t.Run("US-04_Success", func(t *testing.T) {
		want := &authaccount.Account{UserID: "u1"}
		as := &usFakeAccountStore{getByIDAccount: want}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{&linking.LinkedAccount{UserID: "u1"}, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserFromPlatform(linking.PlatformDiscord, "p1")
		if err != nil || got != want {
			t.Errorf("GetUserFromPlatform() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-05_LookupFails", func(t *testing.T) {
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, authaccount.ErrNotFound}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.GetUserFromPlatform(linking.PlatformDiscord, "p1")
		if !errors.Is(err, authaccount.ErrNotFound) {
			t.Errorf("GetUserFromPlatform() err = %v, want ErrNotFound", err)
		}
		if as.getByIDCalls != 0 {
			t.Error("expected GetAccountByID to never be called when the link lookup fails")
		}
	})

	t.Run("US-06_ResolvedAccountLookupFails", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{getByIDErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{&linking.LinkedAccount{UserID: "u1"}, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.GetUserFromPlatform(linking.PlatformDiscord, "p1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetUserFromPlatform() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS07to10GetUserPermissions(t *testing.T) {
	t.Run("US-07_Success", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1", Roles: []string{"1"}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserPermissions("u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"beenamegenerator.admin", "petpictures.admin", "ratelimit:1000"}
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
		wantErr := testerrors.ErrBoom
		svc := usNewService(&usFakeAccountStore{getByIDErr: wantErr}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserPermissions("u1")
		if got != nil || !errors.Is(err, wantErr) {
			t.Errorf("GetUserPermissions() = (%v, %v), want (nil, %v)", got, err, wantErr)
		}
	})

	t.Run("US-09_ResolvesAllRolesTogether", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1", Roles: []string{"1", "2"}}}
		rs := rsDefaultRoleStore()
		svc := NewUserService(as, &usFakeLinkAccountStore{}, linking.NewService(&usFakeLinkAccountStore{}, as), &usFakeAccountSettingsStore{}, rs)

		got, err := svc.GetUserPermissions("u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 5 || len(rs.calls) != 1 || len(rs.calls[0]) != 2 {
			t.Errorf("GetUserPermissions() = %v with calls %v, want both roles' permissions from a single call", got, rs.calls)
		}
	})

	t.Run("US-41_RoleStoreFailureFailsTheLookup", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1", Roles: []string{"1"}}}
		svc := NewUserService(as, &usFakeLinkAccountStore{}, linking.NewService(&usFakeLinkAccountStore{}, as), &usFakeAccountSettingsStore{}, &rsFakeRoleStore{err: wantErr})

		got, err := svc.GetUserPermissions("u1")

		if got != nil || !errors.Is(err, wantErr) {
			t.Errorf("GetUserPermissions() = (%v, %v), want (nil, %v)", got, err, wantErr)
		}
	})

	t.Run("US-10_NoRoles", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1"}}
		rs := rsDefaultRoleStore()
		svc := NewUserService(as, &usFakeLinkAccountStore{}, linking.NewService(&usFakeLinkAccountStore{}, as), &usFakeAccountSettingsStore{}, rs)

		got, err := svc.GetUserPermissions("u1")
		if err != nil || got == nil || len(got) != 0 || len(rs.calls) != 0 {
			t.Errorf("GetUserPermissions() = (%#v, %v) with calls %v, want (empty non-nil, nil) and no role lookup", got, err, rs.calls)
		}
	})

	t.Run("US-42_RolesWithoutPermissionsGiveEmptyNonNil", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1", Roles: []string{"99"}}}
		als := &usFakeLinkAccountStore{}
		svc := NewUserService(as, als, linking.NewService(als, as), &usFakeAccountSettingsStore{}, rsDefaultRoleStore())

		got, err := svc.GetUserPermissions("u1")

		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("GetUserPermissions() = (%#v, %v), want (empty non-nil, nil)", got, err)
		}
	})
}

func TestUS11to14UpdateUser(t *testing.T) {
	oldEmail := "old@b.com"
	newEmail := "new@b.com"

	t.Run("US-11_MergesNonZeroFields", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1", Username: "old", Email: &oldEmail, Roles: []string{"old"}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&authaccount.Account{UserID: "u1", Username: "new", Email: &newEmail, Roles: []string{"new"}})
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
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1", Username: "old", Email: &oldEmail, Roles: []string{"old"}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&authaccount.Account{UserID: "u1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := as.updateLastAccount
		if got.Username != "old" || got.Email == nil || *got.Email != oldEmail || len(got.Roles) != 1 || got.Roles[0] != "old" {
			t.Errorf("UpdateAccountInDB called with %+v, want the original fields preserved when the patch supplies zero values", got)
		}
	})

	t.Run("US-13_GetAccountFails", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{getByIDErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&authaccount.Account{UserID: "u1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUser() err = %v, want %v", err, wantErr)
		}
		if as.updateCalls != 0 {
			t.Error("expected UpdateAccountInDB to never be called when the initial lookup fails")
		}
	})

	t.Run("US-14_UpdateFails", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "u1"}, updateErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser(&authaccount.Account{UserID: "u1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUser() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS24UpdateUserFromPlatformFinalGetAccountFails(t *testing.T) {
	t.Run("US-24_FinalGetAccountFails", func(t *testing.T) {
		wantErr := testerrors.ErrDBDown
		existing := &linking.LinkedAccount{UserID: "u1", Platform: linking.PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{getByIDErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{existing, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(linking.PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
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
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{deleteErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.DeleteUser("u1")
		if !errors.Is(err, wantErr) {
			t.Errorf("DeleteUser() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS34to35GetAccountSettings(t *testing.T) {
	t.Run("US-34_Success", func(t *testing.T) {
		want := &authaccount.AccountSettings{UserID: "u1", PasswordAuthEnabled: true}
		ass := &usFakeAccountSettingsStore{settings: want}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		got, err := svc.GetAccountSettings("u1")
		if err != nil || got != want {
			t.Errorf("GetAccountSettings() = (%v, %v), want (%v, nil)", got, err, want)
		}
	})

	t.Run("US-35_StoreError", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
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
		wantErr := testerrors.ErrBoom
		ass := &usFakeAccountSettingsStore{setErr: wantErr}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		err := svc.SetPasswordAuthEnabled("u1", true)
		if !errors.Is(err, wantErr) {
			t.Errorf("SetPasswordAuthEnabled() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS39to40_AccountSettingsNotFoundPassesThrough(t *testing.T) {
	t.Run("US-39_GetAccountSettingsNotFound", func(t *testing.T) {
		ass := &usFakeAccountSettingsStore{getErr: authaccount.ErrNotFound}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		_, err := svc.GetAccountSettings("u1")
		if !errors.Is(err, authaccount.ErrNotFound) {
			t.Errorf("GetAccountSettings() err = %v, want %v", err, authaccount.ErrNotFound)
		}
	})

	t.Run("US-40_SetPasswordAuthEnabledNotFound", func(t *testing.T) {
		ass := &usFakeAccountSettingsStore{setErr: authaccount.ErrNotFound}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		err := svc.SetPasswordAuthEnabled("u1", true)
		if !errors.Is(err, authaccount.ErrNotFound) {
			t.Errorf("SetPasswordAuthEnabled() err = %v, want %v", err, authaccount.ErrNotFound)
		}
	})
}

type usStubLinkingService struct {
	userID string
	err    error
}

func (s *usStubLinkingService) GetUserLinkedAccounts(string) ([]*linking.LinkedAccount, error) {
	return nil, nil
}
func (s *usStubLinkingService) UnlinkPlatform(string, linking.Platform) error { return nil }
func (s *usStubLinkingService) SetPlatformLoginEnabled(string, linking.Platform, bool) error {
	return nil
}
func (s *usStubLinkingService) UpdateFromPlatform(linking.Platform, string, linking.PlatformData) (string, error) {
	return s.userID, s.err
}

func TestUS83UpdateUserFromPlatformReturnsAccountForLinkedUser(t *testing.T) {
	t.Run("US-83_ReturnsAccountForTheLinkingUserID", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &authaccount.Account{UserID: "resolved"}}
		svc := NewUserService(as, &usFakeLinkAccountStore{}, &usStubLinkingService{userID: "resolved"}, &usFakeAccountSettingsStore{}, rsDefaultRoleStore())

		got, err := svc.UpdateUserFromPlatform(linking.PlatformDiscord, "p1", usFakePlatformData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != as.getByIDAccount {
			t.Errorf("UpdateUserFromPlatform() = %v, want %v", got, as.getByIDAccount)
		}
		if as.getByIDLastUserID != "resolved" {
			t.Errorf("expected GetAccountByID to be called with the linking service's user ID, got %q", as.getByIDLastUserID)
		}
	})
}

func TestUS84UpdateUserFromPlatformLinkErrorSkipsAccountLookup(t *testing.T) {
	t.Run("US-84_LinkingErrorPassesThroughWithoutLookup", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{}
		svc := NewUserService(as, &usFakeLinkAccountStore{}, &usStubLinkingService{err: wantErr}, &usFakeAccountSettingsStore{}, rsDefaultRoleStore())

		_, err := svc.UpdateUserFromPlatform(linking.PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
		if as.getByIDCalls != 0 {
			t.Errorf("expected GetAccountByID never to be called, got %d calls", as.getByIDCalls)
		}
	})
}
