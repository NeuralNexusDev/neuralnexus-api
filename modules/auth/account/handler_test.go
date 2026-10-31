package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil/authtest"
)

type stubService struct {
	account              *Account
	lookupErr            error
	passwordAuthDisabled bool
	passwordAuthErr      error
	newSessionErr        error
}

var _ Service = (*stubService)(nil)

func (s *stubService) AddAccount(context.Context, *Account) error { return nil }

func (s *stubService) GetAccountByID(context.Context, string) (*Account, error) {
	return nil, ErrNotFound
}

func (s *stubService) GetAccountByUsername(context.Context, string) (*Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, ErrNotFound
}

func (s *stubService) GetAccountByEmail(context.Context, string) (*Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, ErrNotFound
}

func (s *stubService) UpdateAccount(context.Context, *Account) error { return nil }

func (s *stubService) DeleteAccount(context.Context, string) error { return nil }

func (s *stubService) IsPasswordAuthEnabled(context.Context, string) (bool, error) {
	return !s.passwordAuthDisabled, s.passwordAuthErr
}

const stubSessionPermission = "stub:perm"

func (s *stubService) NewSession(_ context.Context, a *Account, expiresAt int64) (*session.Session, error) {
	if s.newSessionErr != nil {
		return nil, s.newSessionErr
	}
	return &session.Session{ID: "stub-session", UserID: a.UserID, Permissions: []string{stubSessionPermission}, ExpiresAt: expiresAt}, nil
}

func TestAU01LoginHandlerUsernameHappyPath(t *testing.T) {
	account := passwordAccount("testuser", "test@example.com", "correct-password")
	as := &stubService{account: account}
	ss := &authtest.FakeSessionService{CreateJWTFunc: func(*session.Session) (string, error) { return "test-jwt", nil }}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-01_LoginUsernameHappyPath", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		cookie := testutil.FindCookie(w, session.CookieName)
		if cookie == nil || cookie.Value != "test-jwt" {
			t.Errorf("expected session cookie with value %q, got %+v", "test-jwt", cookie)
		}
		if len(ss.Added) != 1 || len(ss.Added[0].Permissions) != 1 || ss.Added[0].Permissions[0] != stubSessionPermission {
			t.Errorf("stored sessions = %+v, want one session carrying the permissions the account service built", ss.Added)
		}
	})
}

func TestAU02LoginHandlerEmailHappyPath(t *testing.T) {
	account := passwordAccount("testuser", "test@example.com", "correct-password")
	as := &stubService{account: account}
	ss := &authtest.FakeSessionService{CreateJWTFunc: func(*session.Session) (string, error) { return "test-jwt", nil }}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"test@example.com","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-02_LoginEmailHappyPath", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		if testutil.FindCookie(w, session.CookieName) == nil {
			t.Error("expected a session cookie to be set")
		}
	})
}

func TestAU03LoginHandlerMalformedBody(t *testing.T) {
	as := &stubService{}
	ss := &authtest.FakeSessionService{}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`not json`))
	w := httptest.NewRecorder()

	t.Run("ACH-03_LoginMalformedBody", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgInvalidUsernameOrPassword)
	})
}

func TestAU04LoginHandlerAccountLookupFails(t *testing.T) {
	as := &stubService{}
	ss := &authtest.FakeSessionService{}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"no-such-user","password":"whatever"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-04_LoginAccountLookupFails", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgInvalidUsernameOrPassword)
		if testutil.FindCookie(w, session.CookieName) != nil {
			t.Error("expected no session cookie on a failed lookup")
		}
	})
}

func TestAU05LoginHandlerWrongPassword(t *testing.T) {
	account := passwordAccount("testuser", "test@example.com", "correct-password")
	as := &stubService{account: account}
	ss := &authtest.FakeSessionService{}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"wrong-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-05_LoginWrongPassword", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgInvalidUsernameOrPassword)
		if testutil.FindCookie(w, session.CookieName) != nil {
			t.Error("expected no session cookie on a wrong password")
		}
	})
}

func TestAU06LoginHandlerPasswordAuthCheckErrors(t *testing.T) {
	account := passwordAccount("testuser", "test@example.com", "correct-password")
	as := &stubService{account: account, passwordAuthErr: testutil.ErrDBDown}
	ss := &authtest.FakeSessionService{}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-06_LoginPasswordAuthCheckErrors", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgAuthenticationFailed)
	})
}

func TestAU07LoginHandlerPasswordAuthDisabled(t *testing.T) {
	account := passwordAccount("testuser", "test@example.com", "correct-password")
	as := &stubService{account: account, passwordAuthDisabled: true}
	ss := &authtest.FakeSessionService{}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-07_LoginPasswordAuthDisabled", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgInvalidUsernameOrPassword)
		if testutil.FindCookie(w, session.CookieName) != nil {
			t.Error("expected no session cookie when password auth is disabled")
		}
	})
}

func TestAU08LoginHandlerAddSessionFails(t *testing.T) {
	account := passwordAccount("testuser", "test@example.com", "correct-password")
	as := &stubService{account: account}
	ss := &authtest.FakeSessionService{AddSessionErr: testutil.ErrDBDown}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-08_LoginAddSessionFails", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgAuthenticationFailed)
		if testutil.FindCookie(w, session.CookieName) != nil {
			t.Error("expected no session cookie when AddSession fails")
		}
	})
}

func TestAU10LogoutHandlerHappyPath(t *testing.T) {
	sess := &session.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(testutil.LogContext(context.Background()), reqctx.SessionKey, sess)
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &authtest.FakeSessionService{}

	t.Run("ACH-10_LogoutHappyPath", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		if len(ss.DeletedIDs) != 1 || ss.DeletedIDs[0] != "s1" {
			t.Errorf("expected DeleteSession(\"s1\"), got %v", ss.DeletedIDs)
		}
		cookie := testutil.FindCookie(w, session.CookieName)
		if cookie == nil {
			t.Fatal("expected the session cookie to be cleared")
		}
		if cookie.Value != "" {
			t.Errorf("expected an empty cleared cookie value, got %q", cookie.Value)
		}
		if !cookie.Expires.Before(time.Now()) {
			t.Errorf("expected the cleared cookie's Expires to be in the past, got %v", cookie.Expires)
		}
	})
}

func TestAU12LogoutHandlerDeleteSessionFails(t *testing.T) {
	sess := &session.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(testutil.LogContext(context.Background()), reqctx.SessionKey, sess)
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &authtest.FakeSessionService{DeleteSessionErr: testutil.ErrDBDown}

	t.Run("ACH-12_LogoutDeleteSessionFails", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgFailedToDeleteSession)
		if testutil.FindCookie(w, session.CookieName) != nil {
			t.Error("expected no cleared cookie when DeleteSession fails")
		}
	})
}

func TestAU58LoginHandlerNewSessionFails(t *testing.T) {
	account := passwordAccount("testuser", "", "correct-password")
	as := &stubService{account: account, newSessionErr: testutil.ErrBoom}
	ss := &authtest.FakeSessionService{CreateJWTFunc: func(*session.Session) (string, error) { return "test-jwt", nil }}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-58_LoginNewSessionFails", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgAuthenticationFailed)
		if len(ss.Added) != 0 {
			t.Errorf("stored sessions = %v, want none", ss.Added)
		}
	})
}

func TestAU59LoginHandlerAccountLookupInfraFails(t *testing.T) {
	as := &stubService{lookupErr: testutil.ErrDBDown}
	ss := &authtest.FakeSessionService{}
	r := testutil.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"someone","password":"whatever"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-59_LoginAccountLookupInfraFails", func(t *testing.T) {
		LoginHandler(testutil.NewConfig().Auth.Account, as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		testutil.ExpectDetail(t, w, msgAuthenticationFailed)
		if testutil.FindCookie(w, session.CookieName) != nil {
			t.Error("expected no session cookie on a failed lookup")
		}
	})
}
