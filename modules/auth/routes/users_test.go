package authroutes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

// mockUserService implements auth.UserService for unit testing the
// self-or-admin auth boundary and error-to-status mapping on the new
// linked-account endpoints, without needing a real store.
type mockUserService struct {
	links              []*auth.LinkedAccount
	unlinkErr          error
	setEnableErr       error
	setPasswordAuthErr error

	unlinkCalls          []auth.Platform
	setEnableCalls       []bool
	setPasswordAuthCalls []bool
}

var _ auth.UserService = (*mockUserService)(nil)

func (m *mockUserService) GetUser(string) (*auth.Account, error) { return nil, auth.ErrNotFound }
func (m *mockUserService) GetUserFromPlatform(auth.Platform, string) (*auth.Account, error) {
	return nil, auth.ErrNotFound
}
func (m *mockUserService) GetUserPermissions(string) ([]string, error) { return nil, nil }
func (m *mockUserService) UpdateUser(*auth.Account) error              { return nil }
func (m *mockUserService) UpdateUserFromPlatform(auth.Platform, string, auth.PlatformData) (*auth.Account, error) {
	return nil, nil
}
func (m *mockUserService) DeleteUser(string) error { return nil }

func (m *mockUserService) GetUserLinkedAccounts(string) ([]*auth.LinkedAccount, error) {
	return m.links, nil
}
func (m *mockUserService) UnlinkPlatform(_ string, platform auth.Platform) error {
	m.unlinkCalls = append(m.unlinkCalls, platform)
	return m.unlinkErr
}
func (m *mockUserService) SetPlatformLoginEnabled(_ string, _ auth.Platform, enabled bool) error {
	m.setEnableCalls = append(m.setEnableCalls, enabled)
	return m.setEnableErr
}
func (m *mockUserService) SetPasswordAuthEnabled(_ string, enabled bool) error {
	m.setPasswordAuthCalls = append(m.setPasswordAuthCalls, enabled)
	return m.setPasswordAuthErr
}

// requestAsSession builds a request with the given session in context and
// user_id/platform path values set, mirroring what net/http's real routing
// would populate from "/api/v1/users/{user_id}/link/{platform}".
func requestAsSession(method string, session *auth.Session, userID, platform string) *http.Request {
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(method, "/", nil).WithContext(ctx)
	r.SetPathValue("user_id", userID)
	if platform != "" {
		r.SetPathValue("platform", platform)
	}
	return r
}

func adminSession(userID string) *auth.Session {
	return &auth.Session{UserID: userID, Permissions: []string{perms.ScopeAdminUsers.Name + "|" + perms.ScopeAdminUsers.Value}}
}

// -------------- GetUserLinkedAccountsHandler --------------

func TestGetUserLinkedAccountsHandlerSelfAllowed(t *testing.T) {
	svc := &mockUserService{links: []*auth.LinkedAccount{{Platform: auth.PlatformDiscord}}}
	req := requestAsSession(http.MethodGet, &auth.Session{UserID: "u1"}, "u1", "")
	w := httptest.NewRecorder()

	GetUserLinkedAccountsHandler(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserLinkedAccountsHandlerCrossUserForbidden(t *testing.T) {
	svc := &mockUserService{}
	req := requestAsSession(http.MethodGet, &auth.Session{UserID: "u1"}, "someone-else", "")
	w := httptest.NewRecorder()

	GetUserLinkedAccountsHandler(svc)(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a cross-user request, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserLinkedAccountsHandlerAdminAllowedCrossUser(t *testing.T) {
	svc := &mockUserService{links: []*auth.LinkedAccount{{Platform: auth.PlatformDiscord}}}
	req := requestAsSession(http.MethodGet, adminSession("admin1"), "someone-else", "")
	w := httptest.NewRecorder()

	GetUserLinkedAccountsHandler(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for an admin request, got %d: %s", w.Code, w.Body.String())
	}
}

// -------------- UnlinkPlatformHandler --------------

func TestUnlinkPlatformHandlerCrossUserForbidden(t *testing.T) {
	svc := &mockUserService{}
	req := requestAsSession(http.MethodDelete, &auth.Session{UserID: "u1"}, "someone-else", "discord")
	w := httptest.NewRecorder()

	UnlinkPlatformHandler(svc)(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.unlinkCalls) != 0 {
		t.Error("expected UnlinkPlatform to never be called for a forbidden request")
	}
}

func TestUnlinkPlatformHandlerSuccess(t *testing.T) {
	svc := &mockUserService{}
	req := requestAsSession(http.MethodDelete, &auth.Session{UserID: "u1"}, "u1", "discord")
	w := httptest.NewRecorder()

	UnlinkPlatformHandler(svc)(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.unlinkCalls) != 1 || svc.unlinkCalls[0] != auth.PlatformDiscord {
		t.Errorf("expected UnlinkPlatform to be called with discord, got: %v", svc.unlinkCalls)
	}
}

func TestUnlinkPlatformHandlerNotFoundMapsTo404(t *testing.T) {
	svc := &mockUserService{unlinkErr: auth.ErrNotFound}
	req := requestAsSession(http.MethodDelete, &auth.Session{UserID: "u1"}, "u1", "discord")
	w := httptest.NewRecorder()

	UnlinkPlatformHandler(svc)(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnlinkPlatformHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &mockUserService{unlinkErr: auth.ErrWouldLockAccount}
	req := requestAsSession(http.MethodDelete, &auth.Session{UserID: "u1"}, "u1", "discord")
	w := httptest.NewRecorder()

	UnlinkPlatformHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for ErrWouldLockAccount, got %d: %s", w.Code, w.Body.String())
	}
}

// -------------- SetPlatformLoginEnabledHandler --------------

func requestWithJSONBody(method string, session *auth.Session, userID, platform, body string) *http.Request {
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(method, "/", strings.NewReader(body)).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("user_id", userID)
	r.SetPathValue("platform", platform)
	return r
}

func TestSetPlatformLoginEnabledHandlerCrossUserForbidden(t *testing.T) {
	svc := &mockUserService{}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "someone-else", "discord", `{"login_enabled":false}`)
	w := httptest.NewRecorder()

	SetPlatformLoginEnabledHandler(svc)(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetPlatformLoginEnabledHandlerSuccess(t *testing.T) {
	svc := &mockUserService{}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "discord", `{"login_enabled":false}`)
	w := httptest.NewRecorder()

	SetPlatformLoginEnabledHandler(svc)(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.setEnableCalls) != 1 || svc.setEnableCalls[0] != false {
		t.Errorf("expected SetPlatformLoginEnabled(false) to be called, got: %v", svc.setEnableCalls)
	}
}

func TestSetPlatformLoginEnabledHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &mockUserService{setEnableErr: auth.ErrWouldLockAccount}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "discord", `{"login_enabled":false}`)
	w := httptest.NewRecorder()

	SetPlatformLoginEnabledHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for ErrWouldLockAccount, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetPlatformLoginEnabledHandlerUnverifiedMapsTo400(t *testing.T) {
	svc := &mockUserService{setEnableErr: auth.ErrLinkedAccountUnverified}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "discord", `{"login_enabled":true}`)
	w := httptest.NewRecorder()

	SetPlatformLoginEnabledHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for ErrLinkedAccountUnverified, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetPlatformLoginEnabledHandlerInvalidBodyRejected(t *testing.T) {
	svc := &mockUserService{}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "discord", `not json`)
	w := httptest.NewRecorder()

	SetPlatformLoginEnabledHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid body, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.setEnableCalls) != 0 {
		t.Error("expected SetPlatformLoginEnabled to never be called for an invalid body")
	}
}

func TestSetPlatformLoginEnabledHandlerMissingFieldRejected(t *testing.T) {
	svc := &mockUserService{}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "discord", `{}`)
	w := httptest.NewRecorder()

	SetPlatformLoginEnabledHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when login_enabled is omitted, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.setEnableCalls) != 0 {
		t.Error("expected SetPlatformLoginEnabled to never be called when login_enabled is omitted")
	}
}

// -------------- SetPasswordAuthEnabledHandler --------------

func TestSetPasswordAuthEnabledHandlerCrossUserForbidden(t *testing.T) {
	svc := &mockUserService{}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "someone-else", "", `{"password_auth_enabled":false}`)
	w := httptest.NewRecorder()

	SetPasswordAuthEnabledHandler(svc)(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.setPasswordAuthCalls) != 0 {
		t.Error("expected SetPasswordAuthEnabled to never be called for a forbidden request")
	}
}

func TestSetPasswordAuthEnabledHandlerSuccess(t *testing.T) {
	svc := &mockUserService{}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "", `{"password_auth_enabled":false}`)
	w := httptest.NewRecorder()

	SetPasswordAuthEnabledHandler(svc)(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.setPasswordAuthCalls) != 1 || svc.setPasswordAuthCalls[0] != false {
		t.Errorf("expected SetPasswordAuthEnabled(false) to be called, got: %v", svc.setPasswordAuthCalls)
	}
}

func TestSetPasswordAuthEnabledHandlerWouldLockAccountMapsTo400(t *testing.T) {
	svc := &mockUserService{setPasswordAuthErr: auth.ErrWouldLockAccount}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "", `{"password_auth_enabled":false}`)
	w := httptest.NewRecorder()

	SetPasswordAuthEnabledHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for ErrWouldLockAccount, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetPasswordAuthEnabledHandlerNoPasswordSetMapsTo400(t *testing.T) {
	svc := &mockUserService{setPasswordAuthErr: auth.ErrNoPasswordSet}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "", `{"password_auth_enabled":true}`)
	w := httptest.NewRecorder()

	SetPasswordAuthEnabledHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for ErrNoPasswordSet, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetPasswordAuthEnabledHandlerMissingFieldRejected(t *testing.T) {
	svc := &mockUserService{}
	req := requestWithJSONBody(http.MethodPatch, &auth.Session{UserID: "u1"}, "u1", "", `{}`)
	w := httptest.NewRecorder()

	SetPasswordAuthEnabledHandler(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when password_auth_enabled is omitted, got %d: %s", w.Code, w.Body.String())
	}
	if len(svc.setPasswordAuthCalls) != 0 {
		t.Error("expected SetPasswordAuthEnabled to never be called when password_auth_enabled is omitted")
	}
}
