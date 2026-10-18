package user

import (
	"encoding/json"
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/link"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
)

var usUpdatedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// usFullAccount is an account with every field set, including the ones the API must not return.
func usFullAccount() *account.Account {
	email := "u1@example.com"
	return &account.Account{
		UserID:       "u1",
		Username:     "name",
		Email:        &email,
		HashedSecret: []byte("hash"),
		Salt:         []byte("salt"),
		Roles:        []string{"1", "2"},
		UpdatedAt:    usUpdatedAt,
	}
}

func usMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() err = %v", err)
	}
	return string(b)
}

type usFakePlatformData struct {
	username string
}

func (f usFakePlatformData) GetID() string { return "pid" }

func (f usFakePlatformData) GetEmail() string { return "" }

func (f usFakePlatformData) GetUsername() string { return f.username }

func (f usFakePlatformData) GetData() string { return "{}" }

func (f usFakePlatformData) CreateLinkedAccount(_ string) *link.LinkedAccount {
	return nil
}

type usFakeAccountStore struct {
	addErr         error
	addCalls       int
	addLastAccount *account.Account

	getByIDAccount    *account.Account
	getByIDErr        error
	getByIDCalls      int
	getByIDLastUserID string

	listAccounts   []*account.Account
	listErr        error
	listCalls      int
	listLastLimit  int
	listLastOffset int

	updateErr         error
	updateCalls       int
	updateLastAccount *account.Account

	deleteErr        error
	deleteCalls      int
	deleteLastUserID string
}

func (f *usFakeAccountStore) AddAccountToDB(a *account.Account) error {
	f.addCalls++
	f.addLastAccount = a
	return f.addErr
}

func (f *usFakeAccountStore) GetAccountByID(userID string) (*account.Account, error) {
	f.getByIDCalls++
	f.getByIDLastUserID = userID
	return f.getByIDAccount, f.getByIDErr
}

func (f *usFakeAccountStore) GetAccountByUsername(_ string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

func (f *usFakeAccountStore) GetAccountByEmail(_ string) (*account.Account, error) {
	return nil, account.ErrNotFound
}

func (f *usFakeAccountStore) ListAccounts(limit, offset int) ([]*account.Account, error) {
	f.listCalls++
	f.listLastLimit, f.listLastOffset = limit, offset
	return f.listAccounts, f.listErr
}

func (f *usFakeAccountStore) UpdateAccountInDB(a *account.Account) error {
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
	la  *link.LinkedAccount
	err error
}

type usFakeLinkAccountStore struct {
	getByPlatformIDResults []usLinkResult
	getByPlatformIDCalls   int

	addErr               error
	addCalls             int
	addLastLinkedAccount *link.LinkedAccount

	updateErr               error
	updateCalls             int
	updateLastLinkedAccount *link.LinkedAccount

	getsByUserID    []*link.LinkedAccount
	getsByUserIDErr error

	deleteErr   error
	deleteCalls int

	setLoginEnabledErr   error
	setLoginEnabledCalls int
}

func (f *usFakeLinkAccountStore) AddLinkedAccountToDB(la *link.LinkedAccount) error {
	f.addCalls++
	f.addLastLinkedAccount = la
	return f.addErr
}

func (f *usFakeLinkAccountStore) UpdateLinkedAccount(la *link.LinkedAccount) error {
	f.updateCalls++
	f.updateLastLinkedAccount = la
	return f.updateErr
}

func (f *usFakeLinkAccountStore) GetLinkedAccountByPlatformID(_ link.Platform, _ string) (*link.LinkedAccount, error) {
	i := f.getByPlatformIDCalls
	f.getByPlatformIDCalls++
	if i < len(f.getByPlatformIDResults) {
		r := f.getByPlatformIDResults[i]
		return r.la, r.err
	}
	last := f.getByPlatformIDResults[len(f.getByPlatformIDResults)-1]
	return last.la, last.err
}

func (f *usFakeLinkAccountStore) GetLinkedAccountByPlatformName(_ link.Platform, _ string) (*link.LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (f *usFakeLinkAccountStore) GetLinkedAccountByUserID(_ string, _ link.Platform) (*link.LinkedAccount, error) {
	return nil, account.ErrNotFound
}

func (f *usFakeLinkAccountStore) GetLinkedAccountsByUserID(_ string) ([]*link.LinkedAccount, error) {
	return f.getsByUserID, f.getsByUserIDErr
}

func (f *usFakeLinkAccountStore) DeleteLinkedAccount(_ string, _ link.Platform) error {
	f.deleteCalls++
	return f.deleteErr
}

func (f *usFakeLinkAccountStore) SetLinkedAccountLoginEnabled(_ string, _ link.Platform, _ bool) error {
	f.setLoginEnabledCalls++
	return f.setLoginEnabledErr
}

type usFakeAccountSettingsStore struct {
	settings *account.AccountSettings
	getErr   error
	setErr   error
	setCalls int
}

func (f *usFakeAccountSettingsStore) GetAccountSettings(_ string) (*account.AccountSettings, error) {
	return f.settings, f.getErr
}

func (f *usFakeAccountSettingsStore) SetPasswordAuthEnabled(_ string, _ bool) error {
	f.setCalls++
	return f.setErr
}

func usNewService(as *usFakeAccountStore, als *usFakeLinkAccountStore, ass *usFakeAccountSettingsStore) Service {
	return NewService(as, als, link.NewService(als, as), ass, rsDefaultRoleStore())
}

func TestUS01NewService(t *testing.T) {
	t.Run("US-01_WiresGivenSubStores", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1"}}
		als := &usFakeLinkAccountStore{}
		ass := &usFakeAccountSettingsStore{}
		svc := usNewService(as, als, ass)

		if _, err := svc.GetUser("u1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if as.getByIDCalls != 1 {
			t.Errorf("expected GetUser to delegate to the Store passed to NewService")
		}
	})
}

func TestUS02to03GetUser(t *testing.T) {
	t.Run("US-02_Success", func(t *testing.T) {
		svc := usNewService(&usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1", Username: "name"}}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUser("u1")
		if err != nil || got == nil || got.UserID != "u1" || got.Username != "name" {
			t.Errorf("GetUser() = (%+v, %v), want the u1 user, nil", got, err)
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
		want := []*account.Account{{UserID: "u1", Username: "one", Roles: []string{"1"}, UpdatedAt: usUpdatedAt}, {UserID: "u2", Username: "two"}}
		as := &usFakeAccountStore{listAccounts: want}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.ListUsers(25, 50)
		if err != nil || usMarshal(t, got) != usMarshal(t, want) {
			t.Errorf("ListUsers() = (%s, %v), want the JSON of %s, nil", usMarshal(t, got), err, usMarshal(t, want))
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
		want := usFullAccount()
		as := &usFakeAccountStore{getByIDAccount: want}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{&link.LinkedAccount{UserID: "u1"}, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		got, err := svc.GetUserFromPlatform(link.PlatformDiscord, "p1")
		if err != nil || usMarshal(t, got) != usMarshal(t, want) {
			t.Errorf("GetUserFromPlatform() = (%s, %v), want the JSON of %s, nil", usMarshal(t, got), err, usMarshal(t, want))
		}
	})

	t.Run("US-05_LookupFails", func(t *testing.T) {
		as := &usFakeAccountStore{}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{nil, account.ErrNotFound}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.GetUserFromPlatform(link.PlatformDiscord, "p1")
		if !errors.Is(err, account.ErrNotFound) {
			t.Errorf("GetUserFromPlatform() err = %v, want ErrNotFound", err)
		}
		if as.getByIDCalls != 0 {
			t.Error("expected GetAccountByID to never be called when the link lookup fails")
		}
	})

	t.Run("US-06_ResolvedAccountLookupFails", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{getByIDErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{&link.LinkedAccount{UserID: "u1"}, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.GetUserFromPlatform(link.PlatformDiscord, "p1")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetUserFromPlatform() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS07to10GetUserPermissions(t *testing.T) {
	t.Run("US-07_Success", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1", Roles: []string{"1"}}}
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
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1", Roles: []string{"1", "2"}}}
		rs := rsDefaultRoleStore()
		svc := NewService(as, &usFakeLinkAccountStore{}, link.NewService(&usFakeLinkAccountStore{}, as), &usFakeAccountSettingsStore{}, rs)

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
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1", Roles: []string{"1"}}}
		svc := NewService(as, &usFakeLinkAccountStore{}, link.NewService(&usFakeLinkAccountStore{}, as), &usFakeAccountSettingsStore{}, &rsFakeRoleStore{err: wantErr})

		got, err := svc.GetUserPermissions("u1")

		if got != nil || !errors.Is(err, wantErr) {
			t.Errorf("GetUserPermissions() = (%v, %v), want (nil, %v)", got, err, wantErr)
		}
	})

	t.Run("US-10_NoRoles", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1"}}
		rs := rsDefaultRoleStore()
		svc := NewService(as, &usFakeLinkAccountStore{}, link.NewService(&usFakeLinkAccountStore{}, as), &usFakeAccountSettingsStore{}, rs)

		got, err := svc.GetUserPermissions("u1")
		if err != nil || got == nil || len(got) != 0 || len(rs.calls) != 0 {
			t.Errorf("GetUserPermissions() = (%#v, %v) with calls %v, want (empty non-nil, nil) and no role lookup", got, err, rs.calls)
		}
	})

	t.Run("US-42_RolesWithoutPermissionsGiveEmptyNonNil", func(t *testing.T) {
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1", Roles: []string{"99"}}}
		als := &usFakeLinkAccountStore{}
		svc := NewService(as, als, link.NewService(als, as), &usFakeAccountSettingsStore{}, rsDefaultRoleStore())

		got, err := svc.GetUserPermissions("u1")

		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("GetUserPermissions() = (%#v, %v), want (empty non-nil, nil)", got, err)
		}
	})
}

func TestUS11to14UpdateUser(t *testing.T) {
	t.Run("US-11_MergesNonZeroFields", func(t *testing.T) {
		email := "old@b.com"
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1", Username: "old", Email: &email, Roles: []string{"old"}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser("u1", &UserUpdate{Username: "new", Roles: []string{"new"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if as.updateCalls != 1 {
			t.Fatalf("expected UpdateAccountInDB to be called once, got %d", as.updateCalls)
		}
		got := as.updateLastAccount
		if got.Username != "new" || len(got.Roles) != 1 || got.Roles[0] != "new" {
			t.Errorf("UpdateAccountInDB called with %+v, want username and roles overwritten", got)
		}
	})

	t.Run("US-12_ZeroValueFieldsPreserveExisting", func(t *testing.T) {
		email := "old@b.com"
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1", Username: "old", Email: &email, Roles: []string{"old"}}}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser("u1", &UserUpdate{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := as.updateLastAccount
		if got.Username != "old" || got.Email == nil || *got.Email != email || len(got.Roles) != 1 || got.Roles[0] != "old" {
			t.Errorf("UpdateAccountInDB called with %+v, want the original fields preserved when the patch supplies zero values", got)
		}
	})

	t.Run("US-13_GetAccountFails", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{getByIDErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser("u1", &UserUpdate{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUser() err = %v, want %v", err, wantErr)
		}
		if as.updateCalls != 0 {
			t.Error("expected UpdateAccountInDB to never be called when the initial lookup fails")
		}
	})

	t.Run("US-14_UpdateFails", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{getByIDAccount: &account.Account{UserID: "u1"}, updateErr: wantErr}
		svc := usNewService(as, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		err := svc.UpdateUser("u1", &UserUpdate{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUser() err = %v, want %v", err, wantErr)
		}
	})
}

func TestUS24UpdateUserFromPlatformFinalGetAccountFails(t *testing.T) {
	t.Run("US-24_FinalGetAccountFails", func(t *testing.T) {
		wantErr := testerrors.ErrDBDown
		existing := &link.LinkedAccount{UserID: "u1", Platform: link.PlatformDiscord, PlatformID: "p1"}
		as := &usFakeAccountStore{getByIDErr: wantErr}
		als := &usFakeLinkAccountStore{getByPlatformIDResults: []usLinkResult{{existing, nil}}}
		svc := usNewService(as, als, &usFakeAccountSettingsStore{})

		_, err := svc.UpdateUserFromPlatform(link.PlatformDiscord, "p1", usFakePlatformData{})
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
		want := &account.AccountSettings{UserID: "u1", PasswordAuthEnabled: true}
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
		ass := &usFakeAccountSettingsStore{getErr: account.ErrNotFound}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		_, err := svc.GetAccountSettings("u1")
		if !errors.Is(err, account.ErrNotFound) {
			t.Errorf("GetAccountSettings() err = %v, want %v", err, account.ErrNotFound)
		}
	})

	t.Run("US-40_SetPasswordAuthEnabledNotFound", func(t *testing.T) {
		ass := &usFakeAccountSettingsStore{setErr: account.ErrNotFound}
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, ass)

		err := svc.SetPasswordAuthEnabled("u1", true)
		if !errors.Is(err, account.ErrNotFound) {
			t.Errorf("SetPasswordAuthEnabled() err = %v, want %v", err, account.ErrNotFound)
		}
	})
}

type usStubLinkingService struct {
	userID string
	err    error
}

func (s *usStubLinkingService) GetUserLinkedAccounts(string) ([]*link.LinkedAccount, error) {
	return nil, nil
}
func (s *usStubLinkingService) UnlinkPlatform(string, link.Platform) error { return nil }
func (s *usStubLinkingService) SetPlatformLoginEnabled(string, link.Platform, bool) error {
	return nil
}
func (s *usStubLinkingService) UpdateFromPlatform(link.Platform, string, link.PlatformData) (string, error) {
	return s.userID, s.err
}

func TestUS85UserFromAccountKeepsOnlyPublicFields(t *testing.T) {
	t.Run("US-85_GetUserMarshalsLikeTheAccount", func(t *testing.T) {
		want := usFullAccount()
		svc := usNewService(&usFakeAccountStore{getByIDAccount: want}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUser("u1")
		if err != nil || usMarshal(t, got) != usMarshal(t, want) {
			t.Errorf("GetUser() = (%s, %v), want the JSON of the account %s, nil", usMarshal(t, got), err, usMarshal(t, want))
		}
	})

	t.Run("US-86_NilAccountGivesNilUser", func(t *testing.T) {
		svc := usNewService(&usFakeAccountStore{}, &usFakeLinkAccountStore{}, &usFakeAccountSettingsStore{})

		got, err := svc.GetUser("u1")
		if got != nil || err != nil {
			t.Errorf("GetUser() = (%+v, %v), want (nil, nil)", got, err)
		}
	})
}

func TestUS83UpdateUserFromPlatformReturnsAccountForLinkedUser(t *testing.T) {
	t.Run("US-83_ReturnsAccountForTheLinkUserID", func(t *testing.T) {
		want := &account.Account{UserID: "resolved", Username: "r", Roles: []string{"1"}, UpdatedAt: usUpdatedAt}
		as := &usFakeAccountStore{getByIDAccount: want}
		svc := NewService(as, &usFakeLinkAccountStore{}, &usStubLinkingService{userID: "resolved"}, &usFakeAccountSettingsStore{}, rsDefaultRoleStore())

		got, err := svc.UpdateUserFromPlatform(link.PlatformDiscord, "p1", usFakePlatformData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usMarshal(t, got) != usMarshal(t, want) {
			t.Errorf("UpdateUserFromPlatform() = %s, want the JSON of %s", usMarshal(t, got), usMarshal(t, want))
		}
		if as.getByIDLastUserID != "resolved" {
			t.Errorf("expected GetAccountByID to be called with the link service's user ID, got %q", as.getByIDLastUserID)
		}
	})
}

func TestUS84UpdateUserFromPlatformLinkErrorSkipsAccountLookup(t *testing.T) {
	t.Run("US-84_LinkErrorPassesThroughWithoutLookup", func(t *testing.T) {
		wantErr := testerrors.ErrBoom
		as := &usFakeAccountStore{}
		svc := NewService(as, &usFakeLinkAccountStore{}, &usStubLinkingService{err: wantErr}, &usFakeAccountSettingsStore{}, rsDefaultRoleStore())

		_, err := svc.UpdateUserFromPlatform(link.PlatformDiscord, "p1", usFakePlatformData{})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateUserFromPlatform() err = %v, want %v", err, wantErr)
		}
		if as.getByIDCalls != 0 {
			t.Errorf("expected GetAccountByID never to be called, got %d calls", as.getByIDCalls)
		}
	})
}

type rsFakeRoleStore struct {
	permissions map[string][]string
	err         error
	calls       [][]string
}

func (f *rsFakeRoleStore) GetPermissionsForRoles(roleIDs []string) ([]string, error) {
	f.calls = append(f.calls, roleIDs)
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, id := range roleIDs {
		out = append(out, f.permissions[id]...)
	}
	return out, nil
}

func rsDefaultRoleStore() *rsFakeRoleStore {
	return &rsFakeRoleStore{permissions: map[string][]string{
		"1": {"beenamegenerator.admin", "petpictures.admin", "ratelimit:1000"},
		"2": {"users.admin", "datastore.admin"},
	}}
}
