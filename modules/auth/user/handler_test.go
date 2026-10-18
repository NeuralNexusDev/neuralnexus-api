package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/link"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil/authtest"
)

type stubUserService struct {
	user    *User
	userErr error

	listed     []*User
	listErr    error
	listCalls  int
	listLimit  int
	listOffset int

	permissions    []string
	permissionsErr error

	updateUserErr error
	updatedUserID string
	updatedUpdate *UserUpdate

	updatedUser              *User
	updateFromPlatformErr    error
	updateFromPlatformCalled bool

	deleteUserErr error

	settings       *account.AccountSettings
	getSettingsErr error

	setPasswordAuthErr   error
	setPasswordAuthCalls []bool
}

var _ Service = (*stubUserService)(nil)

func (s *stubUserService) GetUser(string) (*User, error) { return s.user, s.userErr }

func (s *stubUserService) GetUserFromPlatform(link.Platform, string) (*User, error) {
	return s.user, s.userErr
}

func (s *stubUserService) ListUsers(limit, offset int) ([]*User, error) {
	s.listCalls++
	s.listLimit, s.listOffset = limit, offset
	return s.listed, s.listErr
}

func (s *stubUserService) GetUserPermissions(string) ([]string, error) {
	return s.permissions, s.permissionsErr
}

func (s *stubUserService) UpdateUser(userID string, update *UserUpdate) error {
	s.updatedUserID = userID
	s.updatedUpdate = update
	return s.updateUserErr
}

func (s *stubUserService) UpdateUserFromPlatform(link.Platform, string, link.PlatformData) (*User, error) {
	s.updateFromPlatformCalled = true
	return s.updatedUser, s.updateFromPlatformErr
}

func (s *stubUserService) DeleteUser(string) error { return s.deleteUserErr }

func (s *stubUserService) GetAccountSettings(string) (*account.AccountSettings, error) {
	return s.settings, s.getSettingsErr
}

func (s *stubUserService) SetPasswordAuthEnabled(_ string, enabled bool) error {
	s.setPasswordAuthCalls = append(s.setPasswordAuthCalls, enabled)
	return s.setPasswordAuthErr
}

func TestUR01GetUserHandlerSelfHappyPath(t *testing.T) {
	svc := &stubUserService{user: &User{UserID: "u1"}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-01_GetUserSelfHappyPath", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR02GetUserHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{user: &User{UserID: "someone-else"}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.AdminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-02_GetUserAdminHappyPath", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR03GetUserHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-03_GetUserForbidden", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToGetUser)
	})
}

func TestUR04GetUserHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{userErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-04_GetUserServiceErrorMapsTo500", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetUser)
	})
}

func TestUR05GetUserFromPlatformHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{user: &User{UserID: "u1"}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.AdminUsersSession("admin1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("UR-05_GetUserFromPlatformAdminHappyPath", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR06GetUserFromPlatformHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("UR-06_GetUserFromPlatformForbidden", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToGetUsers)
	})
}

func TestUR07GetUserFromPlatformHandlerServiceErrorMapsTo404(t *testing.T) {
	svc := &stubUserService{userErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.AdminUsersSession("admin1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("UR-07_GetUserFromPlatformServiceErrorMapsTo404", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgUserNotFound)
	})
}

func TestUR08GetUserPermissionsHandlerSelfHappyPath(t *testing.T) {
	svc := &stubUserService{permissions: []string{"users.admin"}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-08_GetUserPermissionsSelfHappyPath", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR09GetUserPermissionsHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{permissions: []string{"users.admin"}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.AdminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-09_GetUserPermissionsAdminHappyPath", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR10GetUserPermissionsHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-10_GetUserPermissionsForbidden", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToGetUserPermissions)
	})
}

func TestUR11GetUserPermissionsHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{permissionsErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-11_GetUserPermissionsServiceErrorMapsTo500", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetUser)
	})
}

func TestUR12UpdateUserHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{user: &User{UserID: "u1", Username: "stored-name"}}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"user_id":"someone-else","username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("UR-12_UpdateUserHappyPath", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
		if svc.updatedUserID != "u1" {
			t.Errorf("UpdateUser got user ID %q, want the path user_id u1", svc.updatedUserID)
		}
		if !strings.Contains(w.Body.String(), `"user_id":"u1"`) || !strings.Contains(w.Body.String(), `"username":"stored-name"`) {
			t.Errorf("expected the response to be the stored account for the path user, got %s", w.Body.String())
		}
	})
}

func TestUR13UpdateUserHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("UR-13_UpdateUserForbidden", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdateUsers)
	})
}

func TestUR14UpdateUserHandlerMalformedBody(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `not json`)
	w := httptest.NewRecorder()

	t.Run("UR-14_UpdateUserMalformedBody", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidRequestBody)
	})
}

func TestUR15UpdateUserHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{updateUserErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("UR-15_UpdateUserServiceErrorMapsTo500", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToUpdateUser)
	})
}

func TestUR16to20UpdateUserFromPlatformHandlerPlatformHappyPaths(t *testing.T) {
	tests := []struct {
		id       string
		name     string
		platform string
		body     string
	}{
		{"UR-16", "Discord", "discord", `{"id":"123","username":"discorduser"}`},
		{"UR-17", "Minecraft", "minecraft", `{"id":"00000000-0000-0000-0000-000000000000","username":"mcuser","skins":[],"capes":[]}`},
		{"UR-18", "Twitch", "twitch", `{"id":"123","login":"twitchuser"}`},
		{"UR-19", "XboxLive", "xboxlive", `{"xuid":"123","gamertag":"xboxuser"}`},
		{"UR-20", "Microsoft", "microsoft", `{"sub":"123","name":"msuser"}`},
	}
	for _, tc := range tests {
		t.Run(tc.id+"_UpdateUserFromPlatform"+tc.name+"HappyPath", func(t *testing.T) {
			svc := &stubUserService{updatedUser: &User{UserID: "u1"}}
			r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", tc.platform, tc.body)
			r.SetPathValue("platform_id", "123")
			w := httptest.NewRecorder()

			UpdateUserFromPlatformHandler(svc)(w, r)

			testutil.ExpectStatus(t, w, http.StatusOK)
			if !svc.updateFromPlatformCalled {
				t.Error("expected UpdateUserFromPlatform to be called")
			}
		})
	}
}

func TestUR21UpdateUserFromPlatformHandlerUnsupportedPlatform(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", "bogus-platform", `{}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("UR-21_UpdateUserFromPlatformUnsupportedPlatform", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgUnsupportedPlatform)
		if svc.updateFromPlatformCalled {
			t.Error("expected UpdateUserFromPlatform to never be called for an unsupported platform")
		}
	})
}

func TestUR22UpdateUserFromPlatformHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("UR-22_UpdateUserFromPlatformForbidden", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdateUsers)
		if svc.updateFromPlatformCalled {
			t.Error("expected UpdateUserFromPlatform to never be called for a forbidden request")
		}
	})
}

func TestUR23UpdateUserFromPlatformHandlerMalformedBody(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", "discord", `not json`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("UR-23_UpdateUserFromPlatformMalformedBody", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidRequestBody)
		if svc.updateFromPlatformCalled {
			t.Error("expected UpdateUserFromPlatform to never be called for a malformed body")
		}
	})
}

func TestUR56UpdateUserFromPlatformHandlerMalformedBodyOtherPlatforms(t *testing.T) {
	for _, platform := range []link.Platform{link.PlatformMinecraft, link.PlatformTwitch, link.PlatformXboxLive, link.PlatformMicrosoft} {
		t.Run("UR-56_MalformedBody_"+string(platform), func(t *testing.T) {
			svc := &stubUserService{}
			r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", string(platform), `not json`)
			r.SetPathValue("platform_id", "123")
			w := httptest.NewRecorder()

			UpdateUserFromPlatformHandler(svc)(w, r)

			testutil.ExpectStatus(t, w, http.StatusBadRequest)
			testutil.ExpectDetail(t, w, msgInvalidRequestBody)
			if svc.updateFromPlatformCalled {
				t.Error("expected UpdateUserFromPlatform to never be called for a malformed body")
			}
		})
	}
}

func TestUR24UpdateUserFromPlatformHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("UR-24_UpdateUserFromPlatformServiceErrorMapsTo500", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToUpdateUser)
	})
}

func TestUR25DeleteUserHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.AdminUsersSession("admin1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-25_DeleteUserHappyPath", func(t *testing.T) {
		DeleteUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNoContent)
	})
}

func TestUR26DeleteUserHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-26_DeleteUserForbidden", func(t *testing.T) {
		DeleteUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToDeleteUsers)
	})
}

func TestUR27DeleteUserHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{deleteUserErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodDelete, authtest.AdminUsersSession("admin1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-27_DeleteUserServiceErrorMapsTo500", func(t *testing.T) {
		DeleteUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToDeleteUser)
	})
}

func TestUR45GetAccountSettingsHandlerSelfHappyPath(t *testing.T) {
	svc := &stubUserService{settings: &account.AccountSettings{UserID: "u1", PasswordAuthEnabled: true}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-45_GetAccountSettingsSelfHappyPath", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR46GetAccountSettingsHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{settings: &account.AccountSettings{UserID: "someone-else", PasswordAuthEnabled: true}}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.AdminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-46_GetAccountSettingsAdminHappyPath", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusOK)
	})
}

func TestUR47GetAccountSettingsHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-47_GetAccountSettingsForbidden", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToViewSettings)
	})
}

func TestUR48GetAccountSettingsHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{getSettingsErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-48_GetAccountSettingsServiceErrorMapsTo500", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetAccountSettings)
	})
}

func TestUR49UpdateAccountSettingsHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-49_UpdateAccountSettingsHappyPath", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNoContent)
		if len(svc.setPasswordAuthCalls) != 1 || svc.setPasswordAuthCalls[0] != true {
			t.Errorf("expected SetPasswordAuthEnabled(true), got %v", svc.setPasswordAuthCalls)
		}
	})
}

func TestUR50UpdateAccountSettingsHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "someone-else", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-50_UpdateAccountSettingsForbidden", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToUpdateSettings)
		if len(svc.setPasswordAuthCalls) != 0 {
			t.Error("expected SetPasswordAuthEnabled to never be called for a forbidden request")
		}
	})
}

func TestUR51UpdateAccountSettingsHandlerMalformedBody(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `not json`)
	w := httptest.NewRecorder()

	t.Run("UR-51_UpdateAccountSettingsMalformedBody", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidRequestBody)
	})
}

func TestUR52UpdateAccountSettingsHandlerMissingField(t *testing.T) {
	svc := &stubUserService{}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `{}`)
	w := httptest.NewRecorder()

	t.Run("UR-52_UpdateAccountSettingsMissingField", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setPasswordAuthCalls) != 0 {
			t.Error("expected SetPasswordAuthEnabled to never be called when password_auth is omitted")
		}
	})
}

func TestUR53UpdateAccountSettingsHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubUserService{setPasswordAuthErr: account.ErrWouldLockAccount}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `{"password_auth":false}`)
	w := httptest.NewRecorder()

	t.Run("UR-53_UpdateAccountSettingsWouldLockAccountMapsTo400", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgEnableLoginMethodBeforeDisablingPassword)
	})
}

func TestUR54UpdateAccountSettingsHandlerNoPasswordSetMapsTo400(t *testing.T) {
	svc := &stubUserService{setPasswordAuthErr: account.ErrNoPasswordSet}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-54_UpdateAccountSettingsNoPasswordSetMapsTo400", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgSetPasswordBeforeEnablingPasswordLogin)
	})
}

func TestUR55UpdateAccountSettingsHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{setPasswordAuthErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-55_UpdateAccountSettingsUnclassifiedErrorMapsTo500", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToUpdateUserSettings)
	})
}

func TestUR57GetUserFromPlatformHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{userErr: testerrors.ErrDBDown}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.AdminUsersSession("admin1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("UR-57_GetUserFromPlatformServiceErrorMapsTo500", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetUser)
	})
}

func TestUR58GetUserHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{userErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-58_GetUserNotFoundMapsTo404", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgUserNotFound)
	})
}

func TestUR59GetUserPermissionsHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{permissionsErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-59_GetUserPermissionsNotFoundMapsTo404", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgUserNotFound)
	})
}

func TestUR60UpdateUserHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{updateUserErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("UR-60_UpdateUserNotFoundMapsTo404", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgUserNotFound)
	})
}

func TestUR61UpdateUserHandlerEmailExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateUserErr: account.ErrEmailAlreadyExists}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("UR-61_UpdateUserEmailExistsMapsTo409", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusConflict)
		testutil.ExpectDetail(t, w, msgEmailAlreadyExists)
	})
}

func TestUR62UpdateUserHandlerUsernameExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateUserErr: account.ErrUsernameAlreadyExists}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("UR-62_UpdateUserUsernameExistsMapsTo409", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusConflict)
		testutil.ExpectDetail(t, w, msgUsernameAlreadyExists)
	})
}

func TestUR63UpdateUserFromPlatformHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("UR-63_UpdateUserFromPlatformNotFoundMapsTo404", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgUserNotFound)
	})
}

func TestUR64UpdateUserFromPlatformHandlerEmailExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: account.ErrEmailAlreadyExists}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("UR-64_UpdateUserFromPlatformEmailExistsMapsTo409", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusConflict)
		testutil.ExpectDetail(t, w, msgEmailAlreadyExists)
	})
}

func TestUR65UpdateUserFromPlatformHandlerUsernameExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: account.ErrUsernameAlreadyExists}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("UR-65_UpdateUserFromPlatformUsernameExistsMapsTo409", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusConflict)
		testutil.ExpectDetail(t, w, msgUsernameAlreadyExists)
	})
}

func TestUR66GetAccountSettingsHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{getSettingsErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodGet, authtest.SelfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("UR-66_GetAccountSettingsNotFoundMapsTo404", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgUserNotFound)
	})
}

func TestUR67UpdateAccountSettingsHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{setPasswordAuthErr: account.ErrNotFound}
	r := authtest.NewSessionRequest(http.MethodPatch, authtest.SelfSession("u1"), "u1", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("UR-67_UpdateAccountSettingsNotFoundMapsTo404", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgUserNotFound)
	})
}

func TestUR68to71UpdateUserRoleIDFailures(t *testing.T) {
	cases := []struct {
		id  string
		err error
		msg string
	}{
		{"UR-68_UpdateUserInvalidRoleIDMapsTo400", account.ErrInvalidRoleID, msgInvalidRoleID},
		{"UR-69_UpdateUserUnknownRoleIDMapsTo400", account.ErrUnknownRoleID, msgUnknownRoleID},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			svc := &stubUserService{updateUserErr: c.err}
			r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"roles":["x"]}`)
			w := httptest.NewRecorder()
			UpdateUserHandler(svc)(w, r)
			testutil.ExpectStatus(t, w, http.StatusBadRequest)
			testutil.ExpectDetail(t, w, c.msg)
		})
	}
	platformCases := []struct {
		id  string
		err error
		msg string
	}{
		{"UR-70_UpdateUserFromPlatformInvalidRoleIDMapsTo400", account.ErrInvalidRoleID, msgInvalidRoleID},
		{"UR-71_UpdateUserFromPlatformUnknownRoleIDMapsTo400", account.ErrUnknownRoleID, msgUnknownRoleID},
	}
	for _, c := range platformCases {
		t.Run(c.id, func(t *testing.T) {
			svc := &stubUserService{updateFromPlatformErr: c.err}
			r := authtest.NewSessionRequest(http.MethodPut, authtest.AdminUsersSession("admin1"), "", "discord", `{"id":"1"}`)
			r.SetPathValue("platform_id", "12345")
			w := httptest.NewRecorder()
			UpdateUserFromPlatformHandler(svc)(w, r)
			testutil.ExpectStatus(t, w, http.StatusBadRequest)
			testutil.ExpectDetail(t, w, c.msg)
		})
	}
}

func TestUR72to73UpdateUserRespondsWithTheStoredAccount(t *testing.T) {
	t.Run("UR-72_UpdateUserResponseRolesAreTheStoredRoles", func(t *testing.T) {
		svc := &stubUserService{user: &User{UserID: "u1", Roles: []string{"1", "3"}}}
		r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"roles":["1","1","3"]}`)
		w := httptest.NewRecorder()

		UpdateUserHandler(svc)(w, r)

		testutil.ExpectStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"roles":["1","3"]`) || strings.Contains(w.Body.String(), `"1","1"`) {
			t.Errorf("response = %s, want the stored roles without the duplicate", w.Body.String())
		}
	})

	t.Run("UR-73_UpdateUserReadBackFailureMapsTo500", func(t *testing.T) {
		svc := &stubUserService{userErr: testerrors.ErrDBDown}
		r := authtest.NewSessionRequest(http.MethodPatch, authtest.AdminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
		w := httptest.NewRecorder()

		UpdateUserHandler(svc)(w, r)

		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToUpdateUser)
		if svc.updatedUpdate == nil {
			t.Error("UpdateUser was not called before the read-back")
		}
	})
}

func newListUsersRequest(session *session.Session, query string) *http.Request {
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	return httptest.NewRequest(http.MethodGet, "/api/v1/users"+query, nil).WithContext(ctx)
}

func TestUR74to82ListUsersHandler(t *testing.T) {
	t.Run("UR-74_ListsUsersWithTheDefaultPage", func(t *testing.T) {
		svc := &stubUserService{listed: []*User{{UserID: "u1", Username: "one", Roles: []string{"7"}}, {UserID: "u2", Username: "two", Roles: []string{}}}}
		w := httptest.NewRecorder()

		ListUsersHandler(svc)(w, newListUsersRequest(authtest.AdminUsersSession("admin1"), ""))

		testutil.ExpectStatus(t, w, http.StatusOK)
		if svc.listCalls != 1 || svc.listLimit != 50 || svc.listOffset != 0 {
			t.Errorf("ListUsers called %d times with (%d, %d), want once with (50, 0)", svc.listCalls, svc.listLimit, svc.listOffset)
		}
		var got []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode %q: %v", w.Body.String(), err)
		}
		if len(got) != 2 || got[0]["user_id"] != "u1" || got[0]["username"] != "one" || got[1]["user_id"] != "u2" {
			t.Errorf("body = %s, want the two users in order", w.Body.String())
		}
		for _, field := range []string{"email", "hashed_secret", "salt"} {
			if strings.Contains(w.Body.String(), field) {
				t.Errorf("body = %s, must not contain %q", w.Body.String(), field)
			}
		}
	})

	t.Run("UR-75_PassesTheLimitAndOffset", func(t *testing.T) {
		svc := &stubUserService{}
		w := httptest.NewRecorder()

		ListUsersHandler(svc)(w, newListUsersRequest(authtest.AdminUsersSession("admin1"), "?limit=200&offset=40"))

		testutil.ExpectStatus(t, w, http.StatusOK)
		if svc.listLimit != 200 || svc.listOffset != 40 {
			t.Errorf("ListUsers called with (%d, %d), want (200, 40)", svc.listLimit, svc.listOffset)
		}
	})

	t.Run("UR-76_RefusesWithoutUsersAdmin", func(t *testing.T) {
		svc := &stubUserService{listed: []*User{{UserID: "u1"}}}
		w := httptest.NewRecorder()

		ListUsersHandler(svc)(w, newListUsersRequest(authtest.SelfSession("u1"), ""))

		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToGetUsers)
		if svc.listCalls != 0 {
			t.Errorf("ListUsers was called %d times, want none", svc.listCalls)
		}
	})

	t.Run("UR-77_RefusesAnInvalidLimit", func(t *testing.T) {
		for _, query := range []string{"?limit=0", "?limit=-1", "?limit=201", "?limit=abc", "?limit=1.5", "?limit=%20"} {
			svc := &stubUserService{}
			w := httptest.NewRecorder()

			ListUsersHandler(svc)(w, newListUsersRequest(authtest.AdminUsersSession("admin1"), query))

			testutil.ExpectStatus(t, w, http.StatusBadRequest)
			testutil.ExpectDetail(t, w, msgInvalidLimit)
			if svc.listCalls != 0 {
				t.Errorf("%s: ListUsers was called %d times, want none", query, svc.listCalls)
			}
		}
	})

	t.Run("UR-78_RefusesAnInvalidOffset", func(t *testing.T) {
		for _, query := range []string{"?offset=-1", "?offset=abc", "?offset=1.5", "?offset=%20"} {
			svc := &stubUserService{}
			w := httptest.NewRecorder()

			ListUsersHandler(svc)(w, newListUsersRequest(authtest.AdminUsersSession("admin1"), query))

			testutil.ExpectStatus(t, w, http.StatusBadRequest)
			testutil.ExpectDetail(t, w, msgInvalidOffset)
			if svc.listCalls != 0 {
				t.Errorf("%s: ListUsers was called %d times, want none", query, svc.listCalls)
			}
		}
	})

	t.Run("UR-79_AnEmptyPageIsAnEmptyArray", func(t *testing.T) {
		svc := &stubUserService{}
		w := httptest.NewRecorder()

		ListUsersHandler(svc)(w, newListUsersRequest(authtest.AdminUsersSession("admin1"), "?offset=5000"))

		testutil.ExpectStatus(t, w, http.StatusOK)
		if strings.TrimSpace(w.Body.String()) != "[]" {
			t.Errorf("body = %q, want []", w.Body.String())
		}
	})

	t.Run("UR-80_ServiceFailureMapsTo500", func(t *testing.T) {
		svc := &stubUserService{listErr: testerrors.ErrDBDown}
		w := httptest.NewRecorder()

		ListUsersHandler(svc)(w, newListUsersRequest(authtest.AdminUsersSession("admin1"), ""))

		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToListUsers)
	})

	t.Run("UR-81_RefusesWithoutUsersAdminBeforeCheckingTheQuery", func(t *testing.T) {
		svc := &stubUserService{}
		w := httptest.NewRecorder()

		ListUsersHandler(svc)(w, newListUsersRequest(authtest.SelfSession("u1"), "?limit=0&offset=-1"))

		testutil.ExpectStatus(t, w, http.StatusForbidden)
		testutil.ExpectDetail(t, w, msgNoPermissionToGetUsers)
	})

	t.Run("UR-82_AcceptsTheSmallestLimit", func(t *testing.T) {
		svc := &stubUserService{}
		w := httptest.NewRecorder()

		ListUsersHandler(svc)(w, newListUsersRequest(authtest.AdminUsersSession("admin1"), "?limit=1"))

		testutil.ExpectStatus(t, w, http.StatusOK)
		if svc.listCalls != 1 || svc.listLimit != 1 {
			t.Errorf("ListUsers called %d times with limit %d, want once with limit 1", svc.listCalls, svc.listLimit)
		}
	})
}
