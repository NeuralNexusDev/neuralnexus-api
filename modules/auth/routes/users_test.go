package authroutes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

type stubUserService struct {
	user    *auth.Account
	userErr error

	permissions    []string
	permissionsErr error

	updateUserErr error

	updatedUser              *auth.Account
	updateFromPlatformErr    error
	updateFromPlatformCalled bool

	deleteUserErr error

	links    []*auth.LinkedAccount
	linksErr error

	unlinkErr   error
	unlinkCalls []auth.Platform

	setEnableErr   error
	setEnableCalls []bool

	settings       *auth.AccountSettings
	getSettingsErr error

	setPasswordAuthErr   error
	setPasswordAuthCalls []bool
}

var _ auth.UserService = (*stubUserService)(nil)

func (s *stubUserService) GetUser(string) (*auth.Account, error) { return s.user, s.userErr }
func (s *stubUserService) GetUserFromPlatform(auth.Platform, string) (*auth.Account, error) {
	return s.user, s.userErr
}
func (s *stubUserService) GetUserPermissions(string) ([]string, error) {
	return s.permissions, s.permissionsErr
}
func (s *stubUserService) UpdateUser(user *auth.Account) error { return s.updateUserErr }
func (s *stubUserService) UpdateUserFromPlatform(auth.Platform, string, auth.PlatformData) (*auth.Account, error) {
	s.updateFromPlatformCalled = true
	return s.updatedUser, s.updateFromPlatformErr
}
func (s *stubUserService) DeleteUser(string) error { return s.deleteUserErr }
func (s *stubUserService) GetUserLinkedAccounts(string) ([]*auth.LinkedAccount, error) {
	return s.links, s.linksErr
}
func (s *stubUserService) UnlinkPlatform(_ string, platform auth.Platform) error {
	s.unlinkCalls = append(s.unlinkCalls, platform)
	return s.unlinkErr
}
func (s *stubUserService) SetPlatformLoginEnabled(_ string, _ auth.Platform, enabled bool) error {
	s.setEnableCalls = append(s.setEnableCalls, enabled)
	return s.setEnableErr
}
func (s *stubUserService) GetAccountSettings(string) (*auth.AccountSettings, error) {
	return s.settings, s.getSettingsErr
}
func (s *stubUserService) SetPasswordAuthEnabled(_ string, enabled bool) error {
	s.setPasswordAuthCalls = append(s.setPasswordAuthCalls, enabled)
	return s.setPasswordAuthErr
}

func newSessionRequest(method string, session *auth.Session, userID, platform, body string) *http.Request {
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/", strings.NewReader(body)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, "/", nil).WithContext(ctx)
	}
	if userID != "" {
		r.SetPathValue("user_id", userID)
	}
	if platform != "" {
		r.SetPathValue("platform", platform)
	}
	return r
}

func selfSession(userID string) *auth.Session {
	return &auth.Session{UserID: userID}
}

func adminUsersSession(userID string) *auth.Session {
	return &auth.Session{UserID: userID, Permissions: []string{perms.ScopeAdminUsers.Name + "|" + perms.ScopeAdminUsers.Value}}
}

func expectStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("expected %d, got %d: %s", want, w.Code, w.Body.String())
	}
}

func expectDetail(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", w.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func TestUS01GetUserHandlerSelfHappyPath(t *testing.T) {
	svc := &stubUserService{user: &auth.Account{UserID: "u1"}}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-01_GetUserSelfHappyPath", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS02GetUserHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{user: &auth.Account{UserID: "someone-else"}}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-02_GetUserAdminHappyPath", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS03GetUserHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-03_GetUserForbidden", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToGetUser)
	})
}

func TestUS04GetUserHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{userErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-04_GetUserServiceErrorMapsTo500", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToGetUser)
	})
}

func TestUS05GetUserFromPlatformHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{user: &auth.Account{UserID: "u1"}}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("US-05_GetUserFromPlatformAdminHappyPath", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS06GetUserFromPlatformHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("US-06_GetUserFromPlatformForbidden", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToGetUsers)
	})
}

func TestUS07GetUserFromPlatformHandlerServiceErrorMapsTo404(t *testing.T) {
	svc := &stubUserService{userErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("US-07_GetUserFromPlatformServiceErrorMapsTo404", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgUserNotFound)
	})
}

func TestUS08GetUserPermissionsHandlerSelfHappyPath(t *testing.T) {
	svc := &stubUserService{permissions: []string{"users|*"}}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-08_GetUserPermissionsSelfHappyPath", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS09GetUserPermissionsHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{permissions: []string{"users|*"}}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-09_GetUserPermissionsAdminHappyPath", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS10GetUserPermissionsHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-10_GetUserPermissionsForbidden", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToGetUserPermissions)
	})
}

func TestUS11GetUserPermissionsHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{permissionsErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-11_GetUserPermissionsServiceErrorMapsTo500", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToGetUser)
	})
}

func TestUS12UpdateUserHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "u1", "", `{"user_id":"someone-else","username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("US-12_UpdateUserHappyPath", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"user_id":"u1"`) {
			t.Errorf("expected the response user_id to be overridden with the path value, got %s", w.Body.String())
		}
	})
}

func TestUS13UpdateUserHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("US-13_UpdateUserForbidden", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToUpdateUsers)
	})
}

func TestUS14UpdateUserHandlerMalformedBody(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "u1", "", `not json`)
	w := httptest.NewRecorder()

	t.Run("US-14_UpdateUserMalformedBody", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
	})
}

func TestUS15UpdateUserHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{updateUserErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("US-15_UpdateUserServiceErrorMapsTo500", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToUpdateUser)
	})
}

func TestUS16to20UpdateUserFromPlatformHandlerPlatformHappyPaths(t *testing.T) {
	tests := []struct {
		id       string
		name     string
		platform string
		body     string
	}{
		{"US-16", "Discord", "discord", `{"id":"123","username":"discorduser"}`},
		{"US-17", "Minecraft", "minecraft", `{"id":"00000000-0000-0000-0000-000000000000","username":"mcuser","skins":[],"capes":[]}`},
		{"US-18", "Twitch", "twitch", `{"id":"123","login":"twitchuser"}`},
		{"US-19", "XboxLive", "xboxlive", `{"xuid":"123","gamertag":"xboxuser"}`},
		{"US-20", "Microsoft", "microsoft", `{"sub":"123","name":"msuser"}`},
	}
	for _, tc := range tests {
		t.Run(tc.id+"_UpdateUserFromPlatform"+tc.name+"HappyPath", func(t *testing.T) {
			svc := &stubUserService{updatedUser: &auth.Account{UserID: "u1"}}
			r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", tc.platform, tc.body)
			r.SetPathValue("platform_id", "123")
			w := httptest.NewRecorder()

			UpdateUserFromPlatformHandler(svc)(w, r)

			expectStatus(t, w, http.StatusOK)
			if !svc.updateFromPlatformCalled {
				t.Error("expected UpdateUserFromPlatform to be called")
			}
		})
	}
}

func TestUS21UpdateUserFromPlatformHandlerUnsupportedPlatform(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", "bogus-platform", `{}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("US-21_UpdateUserFromPlatformUnsupportedPlatform", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgUnsupportedPlatform)
		if svc.updateFromPlatformCalled {
			t.Error("expected UpdateUserFromPlatform to never be called for an unsupported platform")
		}
	})
}

func TestUS22UpdateUserFromPlatformHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("US-22_UpdateUserFromPlatformForbidden", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToUpdateUsers)
		if svc.updateFromPlatformCalled {
			t.Error("expected UpdateUserFromPlatform to never be called for a forbidden request")
		}
	})
}

func TestUS23UpdateUserFromPlatformHandlerMalformedBody(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", "discord", `not json`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("US-23_UpdateUserFromPlatformMalformedBody", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
		if svc.updateFromPlatformCalled {
			t.Error("expected UpdateUserFromPlatform to never be called for a malformed body")
		}
	})
}

func TestUS56UpdateUserFromPlatformHandlerMalformedBodyOtherPlatforms(t *testing.T) {
	for _, platform := range []auth.Platform{auth.PlatformMinecraft, auth.PlatformTwitch, auth.PlatformXboxLive, auth.PlatformMicrosoft} {
		t.Run("US-56_MalformedBody_"+string(platform), func(t *testing.T) {
			svc := &stubUserService{}
			r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", string(platform), `not json`)
			r.SetPathValue("platform_id", "123")
			w := httptest.NewRecorder()

			UpdateUserFromPlatformHandler(svc)(w, r)

			expectStatus(t, w, http.StatusBadRequest)
			expectDetail(t, w, msgInvalidRequestBody)
			if svc.updateFromPlatformCalled {
				t.Error("expected UpdateUserFromPlatform to never be called for a malformed body")
			}
		})
	}
}

func TestUS24UpdateUserFromPlatformHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("US-24_UpdateUserFromPlatformServiceErrorMapsTo500", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToUpdateUser)
	})
}

func TestUS25DeleteUserHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodDelete, adminUsersSession("admin1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-25_DeleteUserHappyPath", func(t *testing.T) {
		DeleteUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNoContent)
	})
}

func TestUS26DeleteUserHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-26_DeleteUserForbidden", func(t *testing.T) {
		DeleteUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToDeleteUsers)
	})
}

func TestUS27DeleteUserHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{deleteUserErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodDelete, adminUsersSession("admin1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-27_DeleteUserServiceErrorMapsTo500", func(t *testing.T) {
		DeleteUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToDeleteUser)
	})
}

func TestUS28GetUserLinkedAccountsHandlerSelfHappyPath(t *testing.T) {
	svc := &stubUserService{links: []*auth.LinkedAccount{{Platform: auth.PlatformDiscord}}}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-28_GetUserLinkedAccountsSelfHappyPath", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS29GetUserLinkedAccountsHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{links: []*auth.LinkedAccount{{Platform: auth.PlatformDiscord}}}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-29_GetUserLinkedAccountsAdminHappyPath", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS30GetUserLinkedAccountsHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-30_GetUserLinkedAccountsForbidden", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToViewLinkedAccounts)
	})
}

func TestUS31GetUserLinkedAccountsHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{linksErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-31_GetUserLinkedAccountsServiceErrorMapsTo500", func(t *testing.T) {
		GetUserLinkedAccountsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToGetLinkedAccounts)
	})
}

func TestUS32UnlinkPlatformHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("US-32_UnlinkPlatformHappyPath", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNoContent)
		if len(svc.unlinkCalls) != 1 || svc.unlinkCalls[0] != auth.PlatformDiscord {
			t.Errorf("expected UnlinkPlatform(discord) to be called, got %v", svc.unlinkCalls)
		}
	})
}

func TestUS33UnlinkPlatformHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "someone-else", "discord", "")
	w := httptest.NewRecorder()

	t.Run("US-33_UnlinkPlatformForbidden", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToUnlinkPlatforms)
		if len(svc.unlinkCalls) != 0 {
			t.Error("expected UnlinkPlatform to never be called for a forbidden request")
		}
	})
}

func TestUS34UnlinkPlatformHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{unlinkErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("US-34_UnlinkPlatformNotFoundMapsTo404", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgPlatformNotLinked)
	})
}

func TestUS35UnlinkPlatformHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubUserService{unlinkErr: auth.ErrWouldLockAccount}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("US-35_UnlinkPlatformWouldLockAccountMapsTo400", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgSetPasswordOrLinkBeforeUnlinking)
	})
}

func TestUS36UnlinkPlatformHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{unlinkErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodDelete, selfSession("u1"), "u1", "discord", "")
	w := httptest.NewRecorder()

	t.Run("US-36_UnlinkPlatformUnclassifiedErrorMapsTo500", func(t *testing.T) {
		UnlinkPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToUnlinkPlatform)
	})
}

func TestUS37SetPlatformLoginEnabledHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("US-37_SetPlatformLoginEnabledHappyPath", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNoContent)
		if len(svc.setEnableCalls) != 1 || svc.setEnableCalls[0] != true {
			t.Errorf("expected SetPlatformLoginEnabled(true), got %v", svc.setEnableCalls)
		}
	})
}

func TestUS38SetPlatformLoginEnabledHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "someone-else", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("US-38_SetPlatformLoginEnabledForbidden", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToUpdatePlatforms)
	})
}

func TestUS39SetPlatformLoginEnabledHandlerMalformedBody(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `not json`)
	w := httptest.NewRecorder()

	t.Run("US-39_SetPlatformLoginEnabledMalformedBody", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setEnableCalls) != 0 {
			t.Error("expected SetPlatformLoginEnabled to never be called for a malformed body")
		}
	})
}

func TestUS40SetPlatformLoginEnabledHandlerMissingField(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{}`)
	w := httptest.NewRecorder()

	t.Run("US-40_SetPlatformLoginEnabledMissingField", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setEnableCalls) != 0 {
			t.Error("expected SetPlatformLoginEnabled to never be called when login_enabled is omitted")
		}
	})
}

func TestUS41SetPlatformLoginEnabledHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{setEnableErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("US-41_SetPlatformLoginEnabledNotFoundMapsTo404", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgPlatformNotLinked)
	})
}

func TestUS42SetPlatformLoginEnabledHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubUserService{setEnableErr: auth.ErrWouldLockAccount}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":false}`)
	w := httptest.NewRecorder()

	t.Run("US-42_SetPlatformLoginEnabledWouldLockAccountMapsTo400", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgSetPasswordOrLinkBeforeDisabling)
	})
}

func TestUS43SetPlatformLoginEnabledHandlerUnverifiedMapsTo400(t *testing.T) {
	svc := &stubUserService{setEnableErr: auth.ErrLinkedAccountUnverified}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("US-43_SetPlatformLoginEnabledUnverifiedMapsTo400", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgLinkedAccountUnverified)
	})
}

func TestUS44SetPlatformLoginEnabledHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{setEnableErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	t.Run("US-44_SetPlatformLoginEnabledUnclassifiedErrorMapsTo500", func(t *testing.T) {
		SetPlatformLoginEnabledHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToUpdatePlatform)
	})
}

func TestUS45GetAccountSettingsHandlerSelfHappyPath(t *testing.T) {
	svc := &stubUserService{settings: &auth.AccountSettings{UserID: "u1", PasswordAuthEnabled: true}}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-45_GetAccountSettingsSelfHappyPath", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS46GetAccountSettingsHandlerAdminHappyPath(t *testing.T) {
	svc := &stubUserService{settings: &auth.AccountSettings{UserID: "someone-else", PasswordAuthEnabled: true}}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-46_GetAccountSettingsAdminHappyPath", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusOK)
	})
}

func TestUS47GetAccountSettingsHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "someone-else", "", "")
	w := httptest.NewRecorder()

	t.Run("US-47_GetAccountSettingsForbidden", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToViewSettings)
	})
}

func TestUS48GetAccountSettingsHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{getSettingsErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-48_GetAccountSettingsServiceErrorMapsTo500", func(t *testing.T) {
		GetAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToGetAccountSettings)
	})
}

func TestUS49UpdateAccountSettingsHandlerHappyPath(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("US-49_UpdateAccountSettingsHappyPath", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNoContent)
		if len(svc.setPasswordAuthCalls) != 1 || svc.setPasswordAuthCalls[0] != true {
			t.Errorf("expected SetPasswordAuthEnabled(true), got %v", svc.setPasswordAuthCalls)
		}
	})
}

func TestUS50UpdateAccountSettingsHandlerForbidden(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "someone-else", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("US-50_UpdateAccountSettingsForbidden", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusForbidden)
		expectDetail(t, w, msgNoPermissionToUpdateSettings)
		if len(svc.setPasswordAuthCalls) != 0 {
			t.Error("expected SetPasswordAuthEnabled to never be called for a forbidden request")
		}
	})
}

func TestUS51UpdateAccountSettingsHandlerMalformedBody(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "", `not json`)
	w := httptest.NewRecorder()

	t.Run("US-51_UpdateAccountSettingsMalformedBody", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
	})
}

func TestUS52UpdateAccountSettingsHandlerMissingField(t *testing.T) {
	svc := &stubUserService{}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "", `{}`)
	w := httptest.NewRecorder()

	t.Run("US-52_UpdateAccountSettingsMissingField", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgInvalidRequestBody)
		if len(svc.setPasswordAuthCalls) != 0 {
			t.Error("expected SetPasswordAuthEnabled to never be called when password_auth is omitted")
		}
	})
}

func TestUS53UpdateAccountSettingsHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &stubUserService{setPasswordAuthErr: auth.ErrWouldLockAccount}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "", `{"password_auth":false}`)
	w := httptest.NewRecorder()

	t.Run("US-53_UpdateAccountSettingsWouldLockAccountMapsTo400", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgEnableLoginMethodBeforeDisablingPassword)
	})
}

func TestUS54UpdateAccountSettingsHandlerNoPasswordSetMapsTo400(t *testing.T) {
	svc := &stubUserService{setPasswordAuthErr: auth.ErrNoPasswordSet}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("US-54_UpdateAccountSettingsNoPasswordSetMapsTo400", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusBadRequest)
		expectDetail(t, w, msgSetPasswordBeforeEnablingPasswordLogin)
	})
}

func TestUS55UpdateAccountSettingsHandlerUnclassifiedErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{setPasswordAuthErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodPatch, selfSession("u1"), "u1", "", `{"password_auth":true}`)
	w := httptest.NewRecorder()

	t.Run("US-55_UpdateAccountSettingsUnclassifiedErrorMapsTo500", func(t *testing.T) {
		UpdateAccountSettingsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToUpdateUserSettings)
	})
}

func TestUS57GetUserFromPlatformHandlerServiceErrorMapsTo500(t *testing.T) {
	svc := &stubUserService{userErr: testerrors.ErrDBDown}
	r := newSessionRequest(http.MethodGet, adminUsersSession("admin1"), "", "discord", "")
	r.SetPathValue("platform_id", "12345")
	w := httptest.NewRecorder()

	t.Run("US-57_GetUserFromPlatformServiceErrorMapsTo500", func(t *testing.T) {
		GetUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusInternalServerError)
		expectDetail(t, w, msgFailedToGetUser)
	})
}

func TestUS58GetUserHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{userErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-58_GetUserNotFoundMapsTo404", func(t *testing.T) {
		GetUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgUserNotFound)
	})
}

func TestUS59GetUserPermissionsHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{permissionsErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodGet, selfSession("u1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-59_GetUserPermissionsNotFoundMapsTo404", func(t *testing.T) {
		GetUserPermissionsHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgUserNotFound)
	})
}

func TestUS60UpdateUserHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{updateUserErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("US-60_UpdateUserNotFoundMapsTo404", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgUserNotFound)
	})
}

func TestUS61UpdateUserHandlerEmailExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateUserErr: auth.ErrEmailAlreadyExists}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("US-61_UpdateUserEmailExistsMapsTo409", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusConflict)
		expectDetail(t, w, msgEmailAlreadyExists)
	})
}

func TestUS62UpdateUserHandlerUsernameExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateUserErr: auth.ErrUsernameAlreadyExists}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "u1", "", `{"username":"newname"}`)
	w := httptest.NewRecorder()

	t.Run("US-62_UpdateUserUsernameExistsMapsTo409", func(t *testing.T) {
		UpdateUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusConflict)
		expectDetail(t, w, msgUsernameAlreadyExists)
	})
}

func TestUS63UpdateUserFromPlatformHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("US-63_UpdateUserFromPlatformNotFoundMapsTo404", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgUserNotFound)
	})
}

func TestUS64UpdateUserFromPlatformHandlerEmailExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: auth.ErrEmailAlreadyExists}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("US-64_UpdateUserFromPlatformEmailExistsMapsTo409", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusConflict)
		expectDetail(t, w, msgEmailAlreadyExists)
	})
}

func TestUS65UpdateUserFromPlatformHandlerUsernameExistsMapsTo409(t *testing.T) {
	svc := &stubUserService{updateFromPlatformErr: auth.ErrUsernameAlreadyExists}
	r := newSessionRequest(http.MethodPatch, adminUsersSession("admin1"), "", "discord", `{"id":"123"}`)
	r.SetPathValue("platform_id", "123")
	w := httptest.NewRecorder()

	t.Run("US-65_UpdateUserFromPlatformUsernameExistsMapsTo409", func(t *testing.T) {
		UpdateUserFromPlatformHandler(svc)(w, r)
		expectStatus(t, w, http.StatusConflict)
		expectDetail(t, w, msgUsernameAlreadyExists)
	})
}

func TestUS66DeleteUserHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &stubUserService{deleteUserErr: auth.ErrNotFound}
	r := newSessionRequest(http.MethodDelete, adminUsersSession("admin1"), "u1", "", "")
	w := httptest.NewRecorder()

	t.Run("US-66_DeleteUserNotFoundMapsTo404", func(t *testing.T) {
		DeleteUserHandler(svc)(w, r)
		expectStatus(t, w, http.StatusNotFound)
		expectDetail(t, w, msgUserNotFound)
	})
}
