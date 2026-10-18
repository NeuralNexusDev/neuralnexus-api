package account

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

type stubAccountService struct {
	account              *Account
	lookupErr            error
	passwordAuthDisabled bool
	passwordAuthErr      error
	newSessionErr        error
}

var _ Service = (*stubAccountService)(nil)

func (s *stubAccountService) AddAccount(*Account) error { return nil }

func (s *stubAccountService) GetAccountByID(string) (*Account, error) {
	return nil, ErrNotFound
}

func (s *stubAccountService) GetAccountByUsername(string) (*Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, ErrNotFound
}

func (s *stubAccountService) GetAccountByEmail(string) (*Account, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.account != nil {
		return s.account, s.lookupErr
	}
	return nil, ErrNotFound
}

func (s *stubAccountService) UpdateAccount(*Account) error { return nil }

func (s *stubAccountService) DeleteAccount(string) error { return nil }

func (s *stubAccountService) IsPasswordAuthEnabled(string) (bool, error) {
	return !s.passwordAuthDisabled, s.passwordAuthErr
}

const stubSessionPermission = "stub:perm"

func (s *stubAccountService) NewSession(a *Account, expiresAt int64) (*session.Session, error) {
	if s.newSessionErr != nil {
		return nil, s.newSessionErr
	}
	return &session.Session{ID: "stub-session", UserID: a.UserID, Permissions: []string{stubSessionPermission}, ExpiresAt: expiresAt}, nil
}

type stubSessionService struct {
	createJWT        func(*session.Session) (string, error)
	addSessionErr    error
	deleteSessionErr error
	deletedIDs       []string
	added            []*session.Session
}

var _ session.Service = (*stubSessionService)(nil)

func (s *stubSessionService) AddSession(session *session.Session) error {
	s.added = append(s.added, session)
	return s.addSessionErr
}

func (s *stubSessionService) GetSession(string) (*session.Session, error) {
	return nil, ErrNotFound
}

func (s *stubSessionService) UpdateSession(*session.Session) error { return nil }

func (s *stubSessionService) DeleteSession(id string) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return s.deleteSessionErr
}

func (s *stubSessionService) CreateJWT(session *session.Session) (string, error) {
	if s.createJWT != nil {
		return s.createJWT(session)
	}
	return "", nil
}

func (s *stubSessionService) ReadJWT(string) (*session.Session, error) {
	return nil, ErrNotFound
}

func requireRedirect(t *testing.T, w *httptest.ResponseRecorder, wantTarget string) *url.URL {
	t.Helper()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse Location %q: %v", location, err)
	}
	target := (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
	if target != wantTarget {
		t.Errorf("expected redirect target %q, got %q (full Location %q)", wantTarget, target, location)
	}
	return u
}

func requireProblemDetail(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", w.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func findCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAU01LoginHandlerUsernameHappyPath(t *testing.T) {
	account, err := NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "test-jwt", nil }}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-01_LoginUsernameHappyPath", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		cookie := findCookie(w, mw.SessionCookieName)
		if cookie == nil || cookie.Value != "test-jwt" {
			t.Errorf("expected session cookie with value %q, got %+v", "test-jwt", cookie)
		}
		if len(ss.added) != 1 || len(ss.added[0].Permissions) != 1 || ss.added[0].Permissions[0] != stubSessionPermission {
			t.Errorf("stored sessions = %+v, want one session carrying the permissions the account service built", ss.added)
		}
	})
}

func TestAU02LoginHandlerEmailHappyPath(t *testing.T) {
	account, err := NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "test-jwt", nil }}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"test@example.com","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-02_LoginEmailHappyPath", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		if findCookie(w, mw.SessionCookieName) == nil {
			t.Error("expected a session cookie to be set")
		}
	})
}

func TestAU03LoginHandlerMalformedBody(t *testing.T) {
	as := &stubAccountService{}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`not json`))
	w := httptest.NewRecorder()

	t.Run("ACH-03_LoginMalformedBody", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgInvalidUsernameOrPassword)
	})
}

func TestAU04LoginHandlerAccountLookupFails(t *testing.T) {
	as := &stubAccountService{}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"no-such-user","password":"whatever"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-04_LoginAccountLookupFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgInvalidUsernameOrPassword)
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie on a failed lookup")
		}
	})
}

func TestAU05LoginHandlerWrongPassword(t *testing.T) {
	account, err := NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"wrong-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-05_LoginWrongPassword", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgInvalidUsernameOrPassword)
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie on a wrong password")
		}
	})
}

func TestAU06LoginHandlerPasswordAuthCheckErrors(t *testing.T) {
	account, err := NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account, passwordAuthErr: testerrors.ErrDBDown}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-06_LoginPasswordAuthCheckErrors", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgAuthenticationFailed)
	})
}

func TestAU07LoginHandlerPasswordAuthDisabled(t *testing.T) {
	account, err := NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account, passwordAuthDisabled: true}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-07_LoginPasswordAuthDisabled", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgInvalidUsernameOrPassword)
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie when password auth is disabled")
		}
	})
}

func TestAU08LoginHandlerAddSessionFails(t *testing.T) {
	account, err := NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{addSessionErr: testerrors.ErrDBDown}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-08_LoginAddSessionFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgAuthenticationFailed)
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie when AddSession fails")
		}
	})
}

func TestAU09LoginHandlerCreateJWTFails(t *testing.T) {
	account, err := NewAccount("testuser", "test@example.com", "correct-password")
	if err != nil {
		t.Fatalf("failed to build account: %v", err)
	}
	as := &stubAccountService{account: account}
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "", testerrors.ErrSigningFailed }}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-09_LoginCreateJWTFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgAuthenticationFailed)
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie when CreateJWT fails")
		}
	})
}

func TestAU10LogoutHandlerHappyPath(t *testing.T) {
	session := &session.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &stubSessionService{}

	t.Run("ACH-10_LogoutHappyPath", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		if len(ss.deletedIDs) != 1 || ss.deletedIDs[0] != "s1" {
			t.Errorf("expected DeleteSession(\"s1\"), got %v", ss.deletedIDs)
		}
		cookie := findCookie(w, mw.SessionCookieName)
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

func TestAU11LogoutHandlerNilSessionInContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), mw.SessionKey, (*session.Session)(nil))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &stubSessionService{}

	t.Run("ACH-11_LogoutNilSessionInContext", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgInvalidSession)
		if len(ss.deletedIDs) != 0 {
			t.Error("expected DeleteSession to never be called for a nil session")
		}
	})
}

func TestAU12LogoutHandlerDeleteSessionFails(t *testing.T) {
	session := &session.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), mw.SessionKey, session)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	ss := &stubSessionService{deleteSessionErr: testerrors.ErrDBDown}

	t.Run("ACH-12_LogoutDeleteSessionFails", func(t *testing.T) {
		LogoutHandler(ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgFailedToDeleteSession)
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no cleared cookie when DeleteSession fails")
		}
	})
}

func TestAU58LoginHandlerNewSessionFails(t *testing.T) {
	account := &Account{UserID: "u1", Username: "testuser"}
	if err := account.HashPassword("correct-password"); err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	as := &stubAccountService{account: account, newSessionErr: testerrors.ErrBoom}
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "test-jwt", nil }}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"testuser","password":"correct-password"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-58_LoginNewSessionFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgAuthenticationFailed)
		if len(ss.added) != 0 {
			t.Errorf("stored sessions = %v, want none", ss.added)
		}
	})
}

func TestAU36SetSessionCookieHappyPath(t *testing.T) {
	sess := &session.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "test-jwt", nil }}
	w := httptest.NewRecorder()

	t.Run("ACH-36_SetSessionCookieHappyPath", func(t *testing.T) {
		if err := SetSessionCookie(ss, w, sess); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		cookie := findCookie(w, mw.SessionCookieName)
		if cookie == nil {
			t.Fatal("expected a session cookie to be set")
		}
		if cookie.Value != "test-jwt" {
			t.Errorf("expected cookie value %q, got %q", "test-jwt", cookie.Value)
		}
		wantExpires := time.Unix(sess.ExpiresAt, 0)
		if !cookie.Expires.Equal(wantExpires) {
			t.Errorf("expected cookie Expires %v, got %v", wantExpires, cookie.Expires)
		}
	})
}

func TestAU37SetSessionCookieCreateJWTFails(t *testing.T) {
	sess := &session.Session{ID: "s1", UserID: "u1"}
	wantErr := testerrors.ErrSigningFailed
	ss := &stubSessionService{createJWT: func(*session.Session) (string, error) { return "", wantErr }}
	w := httptest.NewRecorder()

	t.Run("ACH-37_SetSessionCookieCreateJWTFails", func(t *testing.T) {
		err := SetSessionCookie(ss, w, sess)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no cookie to be set when CreateJWT fails")
		}
	})
}

func TestAU49SessionCookie(t *testing.T) {
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("ACH-49_SessionCookie", func(t *testing.T) {
		c := sessionCookie("abc", expires)

		if c.Name != mw.SessionCookieName {
			t.Errorf("expected Name %q, got %q", mw.SessionCookieName, c.Name)
		}
		if c.Value != "abc" {
			t.Errorf("expected Value %q, got %q", "abc", c.Value)
		}
		if c.Domain != ".neuralnexus.dev" {
			t.Errorf("expected Domain %q, got %q", ".neuralnexus.dev", c.Domain)
		}
		if c.Path != "/" {
			t.Errorf("expected Path %q, got %q", "/", c.Path)
		}
		if !c.Expires.Equal(expires) {
			t.Errorf("expected Expires %v, got %v", expires, c.Expires)
		}
		if !c.Secure {
			t.Error("expected Secure=true")
		}
		if !c.HttpOnly {
			t.Error("expected HttpOnly=true")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("expected SameSite=Lax, got %v", c.SameSite)
		}
	})
}

func TestAU59LoginHandlerAccountLookupInfraFails(t *testing.T) {
	as := &stubAccountService{lookupErr: testerrors.ErrDBDown}
	ss := &stubSessionService{}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"someone","password":"whatever"}`))
	w := httptest.NewRecorder()

	t.Run("ACH-59_LoginAccountLookupInfraFails", func(t *testing.T) {
		LoginHandler(as, ss)(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
		}
		requireProblemDetail(t, w, msgAuthenticationFailed)
		if findCookie(w, mw.SessionCookieName) != nil {
			t.Error("expected no session cookie on a failed lookup")
		}
	})
}
