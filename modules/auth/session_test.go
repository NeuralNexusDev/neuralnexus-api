package auth

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/golang-jwt/jwt/v5"
)

type seFakeSessionStore struct {
	addToDBErr   error
	addToDBCalls int

	getFromDBSession *Session
	getFromDBErr     error
	getFromDBCalls   int

	updateInDBErr   error
	updateInDBCalls int

	deleteInDBErr   error
	deleteInDBCalls int

	addToCacheErr   error
	addToCacheCalls int

	getFromCacheSession *Session
	getFromCacheErr     error
	getFromCacheCalls   int

	deleteFromCacheErr   error
	deleteFromCacheCalls int
}

func (f *seFakeSessionStore) AddSessionToDB(_ *Session) error {
	f.addToDBCalls++
	return f.addToDBErr
}
func (f *seFakeSessionStore) GetSessionFromDB(_ string) (*Session, error) {
	f.getFromDBCalls++
	return f.getFromDBSession, f.getFromDBErr
}
func (f *seFakeSessionStore) UpdateSessionInDB(_ *Session) error {
	f.updateInDBCalls++
	return f.updateInDBErr
}
func (f *seFakeSessionStore) DeleteSessionInDB(_ string) error {
	f.deleteInDBCalls++
	return f.deleteInDBErr
}
func (f *seFakeSessionStore) AddSessionToCache(_ *Session) error {
	f.addToCacheCalls++
	return f.addToCacheErr
}
func (f *seFakeSessionStore) GetSessionFromCache(_ string) (*Session, error) {
	f.getFromCacheCalls++
	return f.getFromCacheSession, f.getFromCacheErr
}
func (f *seFakeSessionStore) DeleteSessionFromCache(_ string) error {
	f.deleteFromCacheCalls++
	return f.deleteFromCacheErr
}

type seFakeStore struct {
	ss SessionStore
}

func (f *seFakeStore) Account() AccountStore { panic("seFakeStore: Account not implemented") }
func (f *seFakeStore) AccountSettings() AccountSettingsStore {
	panic("seFakeStore: AccountSettings not implemented")
}
func (f *seFakeStore) Session() SessionStore { return f.ss }
func (f *seFakeStore) LinkAccount() LinkAccountStore {
	panic("seFakeStore: LinkAccount not implemented")
}
func (f *seFakeStore) RateLimit() RateLimitStore   { panic("seFakeStore: RateLimit not implemented") }
func (f *seFakeStore) OAuthToken() OAuthTokenStore { panic("seFakeStore: OAuthToken not implemented") }

func TestSE01ToProto(t *testing.T) {
	t.Run("SE-01_CopiesEveryField", func(t *testing.T) {
		s := &Session{
			ID:          "s1",
			UserID:      "u1",
			Permissions: []string{"a|b"},
			IssuedAt:    100,
			LastUsedAt:  200,
			ExpiresAt:   300,
		}
		got := s.ToProto()
		pb, ok := got.(interface {
			GetId() string
			GetUserId() string
			GetPermissions() []string
			GetIssuedAt() int64
			GetLastUsedAt() int64
			GetExpiresAt() int64
		})
		if !ok {
			t.Fatalf("ToProto() returned an unexpected type: %T", got)
		}
		if pb.GetId() != s.ID || pb.GetUserId() != s.UserID || pb.GetIssuedAt() != s.IssuedAt ||
			pb.GetLastUsedAt() != s.LastUsedAt || pb.GetExpiresAt() != s.ExpiresAt {
			t.Errorf("ToProto() did not copy scalar fields correctly: %+v", got)
		}
		if len(pb.GetPermissions()) != 1 || pb.GetPermissions()[0] != "a|b" {
			t.Errorf("ToProto() Permissions = %v, want [a|b]", pb.GetPermissions())
		}
	})
}

func TestSE02to03HasPermission(t *testing.T) {
	scope := perms.Scope{Name: "perm", Value: "value"}

	t.Run("SE-02_Match", func(t *testing.T) {
		s := &Session{Permissions: []string{"perm|value"}}
		if !s.HasPermission(scope) {
			t.Error("HasPermission() = false, want true")
		}
	})

	t.Run("SE-03_NoMatch", func(t *testing.T) {
		s := &Session{Permissions: []string{}}
		if s.HasPermission(scope) {
			t.Error("HasPermission() = true, want false")
		}
		s2 := &Session{Permissions: []string{"other|thing"}}
		if s2.HasPermission(scope) {
			t.Error("HasPermission() = true for a non-matching entry, want false")
		}
	})
}

func TestSE04to06IsValid(t *testing.T) {
	t.Run("SE-04_FutureExpiry", func(t *testing.T) {
		s := &Session{ExpiresAt: time.Now().Add(time.Hour).Unix()}
		if !s.IsValid() {
			t.Error("IsValid() = false, want true")
		}
	})

	t.Run("SE-05_NeverExpires", func(t *testing.T) {
		s := &Session{ExpiresAt: 0}
		if !s.IsValid() {
			t.Error("IsValid() = false, want true")
		}
	})

	t.Run("SE-06_PastExpiry", func(t *testing.T) {
		s := &Session{ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		if s.IsValid() {
			t.Error("IsValid() = true, want false")
		}
	})
}

func TestSE07NewSessionService(t *testing.T) {
	t.Run("SE-07_WiresGivenSubStore", func(t *testing.T) {
		fs := &seFakeSessionStore{getFromDBErr: ErrNotFound, getFromCacheErr: ErrNotFound}
		svc := NewSessionService(&seFakeStore{ss: fs})

		_, _ = svc.GetSession("s1")
		if fs.getFromCacheCalls != 1 {
			t.Errorf("expected GetSession to delegate to the store.Session() instance passed to NewSessionService, got %d cache calls", fs.getFromCacheCalls)
		}
	})
}

func TestSE08to10AddSession(t *testing.T) {
	t.Run("SE-08_Success", func(t *testing.T) {
		fs := &seFakeSessionStore{}
		svc := NewSessionService(&seFakeStore{ss: fs})

		if err := svc.AddSession(&Session{ID: "s1"}); err != nil {
			t.Errorf("AddSession() = %v, want nil", err)
		}
		if fs.addToDBCalls != 1 || fs.addToCacheCalls != 1 {
			t.Errorf("expected one DB call and one cache call, got db=%d cache=%d", fs.addToDBCalls, fs.addToCacheCalls)
		}
	})

	t.Run("SE-09_DBFailurePropagates", func(t *testing.T) {
		wantErr := testerrors.ErrDBDown
		fs := &seFakeSessionStore{addToDBErr: wantErr}
		svc := NewSessionService(&seFakeStore{ss: fs})

		err := svc.AddSession(&Session{ID: "s1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("AddSession() err = %v, want %v", err, wantErr)
		}
		if fs.addToCacheCalls != 0 {
			t.Error("expected AddSessionToCache to never be called after a DB failure")
		}
	})

	t.Run("SE-10_CacheFailureIsFailOpen", func(t *testing.T) {
		fs := &seFakeSessionStore{addToCacheErr: testerrors.ErrCacheDown}
		svc := NewSessionService(&seFakeStore{ss: fs})

		if err := svc.AddSession(&Session{ID: "s1"}); err != nil {
			t.Errorf("AddSession() = %v, want nil (cache failure should be fail-open)", err)
		}
	})
}

func TestSE11to14GetSession(t *testing.T) {
	t.Run("SE-11_CacheHit", func(t *testing.T) {
		want := &Session{ID: "s1"}
		fs := &seFakeSessionStore{getFromCacheSession: want}
		svc := NewSessionService(&seFakeStore{ss: fs})

		got, err := svc.GetSession("s1")
		if err != nil || got != want {
			t.Errorf("GetSession() = (%v, %v), want (%v, nil)", got, err, want)
		}
		if fs.getFromDBCalls != 0 {
			t.Error("expected GetSessionFromDB to never be called on a cache hit")
		}
	})

	t.Run("SE-12_CacheMissFallsBackToDB", func(t *testing.T) {
		want := &Session{ID: "s1"}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: want}
		svc := NewSessionService(&seFakeStore{ss: fs})

		got, err := svc.GetSession("s1")
		if err != nil || got != want {
			t.Errorf("GetSession() = (%v, %v), want (%v, nil)", got, err, want)
		}
		if fs.addToCacheCalls != 1 {
			t.Errorf("expected the cache to be repopulated once, got %d", fs.addToCacheCalls)
		}
	})

	t.Run("SE-13_CacheMissAndDBMiss", func(t *testing.T) {
		wantErr := ErrNotFound
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBErr: wantErr}
		svc := NewSessionService(&seFakeStore{ss: fs})

		got, err := svc.GetSession("s1")
		if got != nil || !errors.Is(err, wantErr) {
			t.Errorf("GetSession() = (%v, %v), want (nil, %v)", got, err, wantErr)
		}
	})

	t.Run("SE-14_RepopulateFailureIsFailOpen", func(t *testing.T) {
		want := &Session{ID: "s1"}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: want, addToCacheErr: testerrors.ErrCacheDown}
		svc := NewSessionService(&seFakeStore{ss: fs})

		got, err := svc.GetSession("s1")
		if err != nil || got != want {
			t.Errorf("GetSession() = (%v, %v), want (%v, nil) even when repopulation fails", got, err, want)
		}
	})
}

func TestSE15to17UpdateSession(t *testing.T) {
	t.Run("SE-15_Success", func(t *testing.T) {
		fs := &seFakeSessionStore{}
		svc := NewSessionService(&seFakeStore{ss: fs})

		if err := svc.UpdateSession(&Session{ID: "s1"}); err != nil {
			t.Errorf("UpdateSession() = %v, want nil", err)
		}
	})

	t.Run("SE-16_DBFailurePropagates", func(t *testing.T) {
		wantErr := testerrors.ErrDBDown
		fs := &seFakeSessionStore{updateInDBErr: wantErr}
		svc := NewSessionService(&seFakeStore{ss: fs})

		err := svc.UpdateSession(&Session{ID: "s1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateSession() err = %v, want %v", err, wantErr)
		}
		if fs.addToCacheCalls != 0 {
			t.Error("expected the cache to never be touched after a DB failure")
		}
	})

	t.Run("SE-17_CacheFailureIsFailOpen", func(t *testing.T) {
		fs := &seFakeSessionStore{addToCacheErr: testerrors.ErrCacheDown}
		svc := NewSessionService(&seFakeStore{ss: fs})

		if err := svc.UpdateSession(&Session{ID: "s1"}); err != nil {
			t.Errorf("UpdateSession() = %v, want nil (cache failure should be fail-open)", err)
		}
	})
}

func TestSE18to20DeleteSession(t *testing.T) {
	t.Run("SE-18_Success", func(t *testing.T) {
		fs := &seFakeSessionStore{}
		svc := NewSessionService(&seFakeStore{ss: fs})

		if err := svc.DeleteSession("s1"); err != nil {
			t.Errorf("DeleteSession() = %v, want nil", err)
		}
	})

	t.Run("SE-19_DBFailurePropagates", func(t *testing.T) {
		wantErr := testerrors.ErrDBDown
		fs := &seFakeSessionStore{deleteInDBErr: wantErr}
		svc := NewSessionService(&seFakeStore{ss: fs})

		err := svc.DeleteSession("s1")
		if !errors.Is(err, wantErr) {
			t.Errorf("DeleteSession() err = %v, want %v", err, wantErr)
		}
		if fs.deleteFromCacheCalls != 0 {
			t.Error("expected cache eviction to never be attempted after a DB failure")
		}
	})

	t.Run("SE-20_CacheEvictFailureIsFailClosed", func(t *testing.T) {
		fs := &seFakeSessionStore{deleteFromCacheErr: testerrors.ErrCacheDown}
		svc := NewSessionService(&seFakeStore{ss: fs})

		err := svc.DeleteSession("s1")
		if !errors.Is(err, testerrors.ErrCacheDown) {
			t.Fatalf("DeleteSession() error = %v, want it to wrap %v (this must be fail-closed, unlike Add/Update)", err, testerrors.ErrCacheDown)
		}
	})
}

func TestSE21to22CreateJWT(t *testing.T) {
	svc := NewSessionService(&seFakeStore{ss: &seFakeSessionStore{}})

	t.Run("SE-21_Success", func(t *testing.T) {
		s := &Session{ID: "s1", UserID: "u1", Permissions: []string{"a|b"}, IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		tok, err := svc.CreateJWT(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		parsed, err := jwt.ParseWithClaims(tok, &SessionClaims{}, func(*jwt.Token) (interface{}, error) { return JWT_SECRET, nil })
		if err != nil {
			t.Fatalf("failed to parse the created token: %v", err)
		}
		claims := parsed.Claims.(*SessionClaims)
		if claims.Subject != "u1" || claims.ID != "s1" {
			t.Errorf("claims = %+v, want Subject=u1 ID=s1", claims)
		}
	})

	t.Run("SE-22_NeverExpiringOmitsExpClaim", func(t *testing.T) {
		s := &Session{ID: "s1", UserID: "u1", IssuedAt: time.Now().Unix(), ExpiresAt: 0}
		tok, err := svc.CreateJWT(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		parsed, err := jwt.ParseWithClaims(tok, &SessionClaims{}, func(*jwt.Token) (interface{}, error) { return JWT_SECRET, nil })
		if err != nil {
			t.Fatalf("failed to parse the created token: %v", err)
		}
		claims := parsed.Claims.(*SessionClaims)
		if claims.ExpiresAt != nil {
			t.Errorf("ExpiresAt claim = %v, want nil (omitted) for a never-expiring session", claims.ExpiresAt)
		}
	})
}

func TestSE23to31ReadJWT(t *testing.T) {
	newSvcAndToken := func(session *Session) (SessionService, *seFakeSessionStore, string) {
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: session}
		svc := NewSessionService(&seFakeStore{ss: fs})
		tok, err := svc.CreateJWT(session)
		if err != nil {
			t.Fatalf("failed to build a token fixture: %v", err)
		}
		return svc, fs, tok
	}

	t.Run("SE-23_Success", func(t *testing.T) {
		session := &Session{ID: "s1", UserID: "u1", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		svc, fs, tok := newSvcAndToken(session)

		got, err := svc.ReadJWT(tok)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "s1" {
			t.Errorf("ReadJWT() session ID = %q, want s1", got.ID)
		}
		if fs.updateInDBCalls != 1 {
			t.Errorf("expected LastUsedAt bump (UpdateSession) to be called once, got %d", fs.updateInDBCalls)
		}
	})

	t.Run("SE-24_MalformedToken", func(t *testing.T) {
		svc := NewSessionService(&seFakeStore{ss: &seFakeSessionStore{}})
		if _, err := svc.ReadJWT("not-a-jwt"); err == nil {
			t.Fatal("expected an error for a malformed token")
		}
	})

	t.Run("SE-25_WrongSigningSecret", func(t *testing.T) {
		claims := SessionClaims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", Audience: validAudiences, IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("a-completely-different-secret"))
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewSessionService(&seFakeStore{ss: &seFakeSessionStore{}})
		if _, err := svc.ReadJWT(tok); err == nil {
			t.Fatal("expected an error for a token signed with the wrong secret")
		}
	})

	t.Run("SE-26_EmptyAudience", func(t *testing.T) {
		claims := SessionClaims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWT_SECRET)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewSessionService(&seFakeStore{ss: &seFakeSessionStore{}})
		if _, err := svc.ReadJWT(tok); !errors.Is(err, ErrMissingAudience) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrMissingAudience)
		}
	})

	t.Run("SE-27_EmptyAudienceEntry", func(t *testing.T) {
		claims := SessionClaims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", Audience: []string{""}, IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWT_SECRET)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewSessionService(&seFakeStore{ss: &seFakeSessionStore{}})
		if _, err := svc.ReadJWT(tok); !errors.Is(err, ErrEmptyAudienceEntry) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrEmptyAudienceEntry)
		}
	})

	t.Run("SE-28_InvalidAudience", func(t *testing.T) {
		claims := SessionClaims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", Audience: []string{"https://evil.example"}, IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWT_SECRET)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewSessionService(&seFakeStore{ss: &seFakeSessionStore{}})
		if _, err := svc.ReadJWT(tok); !errors.Is(err, ErrInvalidAudience) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrInvalidAudience)
		}
	})

	t.Run("SE-29_RevokedSessionRejected", func(t *testing.T) {
		session := &Session{ID: "s1", UserID: "u1", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: session}
		svc := NewSessionService(&seFakeStore{ss: fs})
		tok, err := svc.CreateJWT(session)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		fs.getFromDBSession = nil
		fs.getFromDBErr = ErrNotFound

		if _, err := svc.ReadJWT(tok); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ReadJWT() error = %v, want %v for a validly signed, unexpired token whose session no longer exists", err, ErrNotFound)
		}
	})

	t.Run("SE-30_SubjectMismatch", func(t *testing.T) {
		signedFor := &Session{ID: "s1", UserID: "user-A", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound}
		svc := NewSessionService(&seFakeStore{ss: fs})
		tok, err := svc.CreateJWT(signedFor)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		fs.getFromDBSession = &Session{ID: "s1", UserID: "user-B", ExpiresAt: signedFor.ExpiresAt}

		if _, err := svc.ReadJWT(tok); !errors.Is(err, ErrSessionSubjectMismatch) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrSessionSubjectMismatch)
		}
	})

	t.Run("SE-31_UpdateSessionFailurePropagates", func(t *testing.T) {
		session := &Session{ID: "s1", UserID: "u1", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: session}
		svc := NewSessionService(&seFakeStore{ss: fs})
		tok, err := svc.CreateJWT(session)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		fs.updateInDBErr = testerrors.ErrDBDown

		got, err := svc.ReadJWT(tok)
		if got != nil || !errors.Is(err, testerrors.ErrDBDown) {
			t.Errorf("ReadJWT() = (%v, %v), want (nil, %v)", got, err, testerrors.ErrDBDown)
		}
	})
}

func TestSE32to34InitSessionGo(t *testing.T) {
	t.Run("SE-32_PackageLoadedUnderRequiredEnv", func(t *testing.T) {
		if len(JWT_SECRET) == 0 {
			t.Fatal("JWT_SECRET is empty; init() should already have log.Fatal'd if so, so this file's other tests couldn't be running")
		}
		if NN_SITE_URL == "" || NN_API_URL == "" {
			t.Fatal("NN_SITE_URL/NN_API_URL are empty; init() should already have log.Fatal'd if so")
		}
		want := []string{NN_SITE_URL, NN_API_URL}
		if len(validAudiences) != 2 || validAudiences[0] != want[0] || validAudiences[1] != want[1] {
			t.Errorf("validAudiences = %v, want %v", validAudiences, want)
		}
	})

	t.Run("SE-33_MissingJWTSecretFatals", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), "JWT_SECRET=")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.Success() {
			t.Fatalf("re-exec with JWT_SECRET unset: got err=%v, want a non-zero exit from init()'s log.Fatal; stderr:\n%s", err, stderr.String())
		}
		if !strings.Contains(stderr.String(), msgJWTSecretUnset) {
			t.Errorf("subprocess stderr = %q, want it to contain init()'s JWT_SECRET message", stderr.String())
		}
	})

	t.Run("SE-34_MissingSiteOrAPIURLFatals", func(t *testing.T) {
		for _, unset := range []string{"NN_SITE_URL", "NN_API_URL"} {
			cmd := exec.Command(os.Args[0], "-test.run=^$")
			cmd.Env = append(os.Environ(), unset+"=")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.Success() {
				t.Fatalf("re-exec with %s unset: got err=%v, want a non-zero exit from init()'s log.Fatal; stderr:\n%s", unset, err, stderr.String())
			}
			if !strings.Contains(stderr.String(), msgSiteAPIURLUnset) {
				t.Errorf("subprocess stderr (%s unset) = %q, want it to contain init()'s site/API URL message", unset, stderr.String())
			}
		}
	})
}
