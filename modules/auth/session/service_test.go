package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
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

	getFromCacheCtx     context.Context
	getFromCacheSession *Session
	getFromCacheErr     error
	getFromCacheCalls   int

	deleteFromCacheErr   error
	deleteFromCacheCalls int
}

func (f *seFakeSessionStore) AddSessionToDB(_ context.Context, _ *Session) error {
	f.addToDBCalls++
	return f.addToDBErr
}

func (f *seFakeSessionStore) GetSessionFromDB(_ context.Context, _ string) (*Session, error) {
	f.getFromDBCalls++
	return f.getFromDBSession, f.getFromDBErr
}

func (f *seFakeSessionStore) UpdateSessionInDB(_ context.Context, _ *Session) error {
	f.updateInDBCalls++
	return f.updateInDBErr
}

func (f *seFakeSessionStore) DeleteSessionInDB(_ context.Context, _ string) error {
	f.deleteInDBCalls++
	return f.deleteInDBErr
}

func (f *seFakeSessionStore) AddSessionToCache(_ context.Context, _ *Session) error {
	f.addToCacheCalls++
	return f.addToCacheErr
}

func (f *seFakeSessionStore) GetSessionFromCache(ctx context.Context, _ string) (*Session, error) {
	f.getFromCacheCtx = ctx
	f.getFromCacheCalls++
	return f.getFromCacheSession, f.getFromCacheErr
}

func (f *seFakeSessionStore) DeleteSessionFromCache(_ context.Context, _ string) error {
	f.deleteFromCacheCalls++
	return f.deleteFromCacheErr
}

func TestSE37RequestContextReachesStore(t *testing.T) {
	t.Run("SE-37_RequestContextReachesStore", func(t *testing.T) {
		fs := &seFakeSessionStore{getFromCacheSession: &Session{ID: "s1"}}
		ctx := context.WithValue(context.Background(), reqctx.UserIDKey, "u1")

		if _, err := NewService(testutil.NewConfig(), fs).GetSession(ctx, "s1"); err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}
		if got := fs.getFromCacheCtx.Value(reqctx.UserIDKey); got != "u1" {
			t.Errorf("store context user ID = %v, want %q", got, "u1")
		}
	})
}

func TestSE07NewService(t *testing.T) {
	t.Run("SE-07_WiresGivenSubStore", func(t *testing.T) {
		fs := &seFakeSessionStore{getFromDBErr: ErrNotFound, getFromCacheErr: ErrNotFound}
		svc := NewService(testutil.NewConfig(), fs)

		_, _ = svc.GetSession(testutil.LogContext(context.Background()), "s1")
		if fs.getFromCacheCalls != 1 {
			t.Errorf("expected GetSession to delegate to the Store passed to NewService, got %d cache calls", fs.getFromCacheCalls)
		}
	})
}

func TestSE08to10AddSession(t *testing.T) {
	t.Run("SE-08_Success", func(t *testing.T) {
		fs := &seFakeSessionStore{}
		svc := NewService(testutil.NewConfig(), fs)

		if err := svc.AddSession(testutil.LogContext(context.Background()), &Session{ID: "s1"}); err != nil {
			t.Errorf("AddSession() = %v, want nil", err)
		}
		if fs.addToDBCalls != 1 || fs.addToCacheCalls != 1 {
			t.Errorf("expected one DB call and one cache call, got db=%d cache=%d", fs.addToDBCalls, fs.addToCacheCalls)
		}
	})

	t.Run("SE-09_DBFailurePropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		fs := &seFakeSessionStore{addToDBErr: wantErr}
		svc := NewService(testutil.NewConfig(), fs)

		err := svc.AddSession(testutil.LogContext(context.Background()), &Session{ID: "s1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("AddSession() err = %v, want %v", err, wantErr)
		}
		if fs.addToCacheCalls != 0 {
			t.Error("expected AddSessionToCache to never be called after a DB failure")
		}
	})

	t.Run("SE-10_CacheFailureIsFailOpen", func(t *testing.T) {
		fs := &seFakeSessionStore{addToCacheErr: testutil.ErrCacheDown}
		svc := NewService(testutil.NewConfig(), fs)

		if err := svc.AddSession(testutil.LogContext(context.Background()), &Session{ID: "s1"}); err != nil {
			t.Errorf("AddSession() = %v, want nil (cache failure should be fail-open)", err)
		}
	})
}

func TestSE11to14GetSession(t *testing.T) {
	t.Run("SE-11_CacheHit", func(t *testing.T) {
		want := &Session{ID: "s1"}
		fs := &seFakeSessionStore{getFromCacheSession: want}
		svc := NewService(testutil.NewConfig(), fs)

		got, err := svc.GetSession(testutil.LogContext(context.Background()), "s1")
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
		svc := NewService(testutil.NewConfig(), fs)

		got, err := svc.GetSession(testutil.LogContext(context.Background()), "s1")
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
		svc := NewService(testutil.NewConfig(), fs)

		got, err := svc.GetSession(testutil.LogContext(context.Background()), "s1")
		if got != nil || !errors.Is(err, wantErr) {
			t.Errorf("GetSession() = (%v, %v), want (nil, %v)", got, err, wantErr)
		}
	})

	t.Run("SE-14_RepopulateFailureIsFailOpen", func(t *testing.T) {
		want := &Session{ID: "s1"}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: want, addToCacheErr: testutil.ErrCacheDown}
		svc := NewService(testutil.NewConfig(), fs)

		got, err := svc.GetSession(testutil.LogContext(context.Background()), "s1")
		if err != nil || got != want {
			t.Errorf("GetSession() = (%v, %v), want (%v, nil) even when repopulation fails", got, err, want)
		}
	})
}

func TestSE15to17UpdateSession(t *testing.T) {
	t.Run("SE-15_Success", func(t *testing.T) {
		fs := &seFakeSessionStore{}
		svc := NewService(testutil.NewConfig(), fs)

		if err := svc.UpdateSession(testutil.LogContext(context.Background()), &Session{ID: "s1"}); err != nil {
			t.Errorf("UpdateSession() = %v, want nil", err)
		}
	})

	t.Run("SE-16_DBFailurePropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		fs := &seFakeSessionStore{updateInDBErr: wantErr}
		svc := NewService(testutil.NewConfig(), fs)

		err := svc.UpdateSession(testutil.LogContext(context.Background()), &Session{ID: "s1"})
		if !errors.Is(err, wantErr) {
			t.Errorf("UpdateSession() err = %v, want %v", err, wantErr)
		}
		if fs.addToCacheCalls != 0 {
			t.Error("expected the cache to never be touched after a DB failure")
		}
	})

	t.Run("SE-17_CacheFailureIsFailOpen", func(t *testing.T) {
		fs := &seFakeSessionStore{addToCacheErr: testutil.ErrCacheDown}
		svc := NewService(testutil.NewConfig(), fs)

		if err := svc.UpdateSession(testutil.LogContext(context.Background()), &Session{ID: "s1"}); err != nil {
			t.Errorf("UpdateSession() = %v, want nil (cache failure should be fail-open)", err)
		}
	})
}

func TestSE18to20DeleteSession(t *testing.T) {
	t.Run("SE-18_Success", func(t *testing.T) {
		fs := &seFakeSessionStore{}
		svc := NewService(testutil.NewConfig(), fs)

		if err := svc.DeleteSession(testutil.LogContext(context.Background()), "s1"); err != nil {
			t.Errorf("DeleteSession() = %v, want nil", err)
		}
	})

	t.Run("SE-19_DBFailurePropagates", func(t *testing.T) {
		wantErr := testutil.ErrDBDown
		fs := &seFakeSessionStore{deleteInDBErr: wantErr}
		svc := NewService(testutil.NewConfig(), fs)

		err := svc.DeleteSession(testutil.LogContext(context.Background()), "s1")
		if !errors.Is(err, wantErr) {
			t.Errorf("DeleteSession() err = %v, want %v", err, wantErr)
		}
		if fs.deleteFromCacheCalls != 0 {
			t.Error("expected cache eviction to never be attempted after a DB failure")
		}
	})

	t.Run("SE-20_CacheEvictFailureIsFailClosed", func(t *testing.T) {
		fs := &seFakeSessionStore{deleteFromCacheErr: testutil.ErrCacheDown}
		svc := NewService(testutil.NewConfig(), fs)

		err := svc.DeleteSession(testutil.LogContext(context.Background()), "s1")
		if !errors.Is(err, testutil.ErrCacheDown) {
			t.Fatalf("DeleteSession() error = %v, want it to wrap %v (this must be fail-closed, unlike Add/Update)", err, testutil.ErrCacheDown)
		}
	})
}

func TestSE23to31ReadJWT(t *testing.T) {
	cfg := testutil.NewConfig()
	newSvcAndToken := func(session *Session) (Service, *seFakeSessionStore, string) {
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: session}
		svc := NewService(cfg, fs)
		tok, err := svc.CreateJWT(context.Background(), session)
		if err != nil {
			t.Fatalf("failed to build a token fixture: %v", err)
		}
		return svc, fs, tok
	}

	t.Run("SE-23_Success", func(t *testing.T) {
		session := &Session{ID: "s1", UserID: "u1", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		svc, fs, tok := newSvcAndToken(session)

		got, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok)
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
		svc := NewService(cfg, &seFakeSessionStore{})
		if _, err := svc.ReadJWT(testutil.LogContext(context.Background()), "not-a-jwt"); err == nil {
			t.Fatal("expected an error for a malformed token")
		}
	})

	t.Run("SE-25_WrongSigningSecret", func(t *testing.T) {
		claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", Audience: cfg.Auth.Session.ValidAudiences, IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("a-completely-different-secret"))
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewService(cfg, &seFakeSessionStore{})
		if _, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok); err == nil {
			t.Fatal("expected an error for a token signed with the wrong secret")
		}
	})

	t.Run("SE-26_EmptyAudience", func(t *testing.T) {
		claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(cfg.Auth.Session.JwtSecret)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewService(cfg, &seFakeSessionStore{})
		if _, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok); !errors.Is(err, ErrMissingAudience) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrMissingAudience)
		}
	})

	t.Run("SE-27_EmptyAudienceEntry", func(t *testing.T) {
		claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", Audience: []string{""}, IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(cfg.Auth.Session.JwtSecret)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewService(cfg, &seFakeSessionStore{})
		if _, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok); !errors.Is(err, ErrEmptyAudienceEntry) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrEmptyAudienceEntry)
		}
	})

	t.Run("SE-28_InvalidAudience", func(t *testing.T) {
		claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
			Subject: "u1", ID: "s1", Audience: []string{"https://evil.example"}, IssuedAt: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(cfg.Auth.Session.JwtSecret)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		svc := NewService(cfg, &seFakeSessionStore{})
		if _, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok); !errors.Is(err, ErrInvalidAudience) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrInvalidAudience)
		}
	})

	t.Run("SE-29_RevokedSessionRejected", func(t *testing.T) {
		session := &Session{ID: "s1", UserID: "u1", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: session}
		svc := NewService(cfg, fs)
		tok, err := svc.CreateJWT(context.Background(), session)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		fs.getFromDBSession = nil
		fs.getFromDBErr = ErrNotFound

		if _, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ReadJWT() error = %v, want %v for a validly signed, unexpired token whose session no longer exists", err, ErrNotFound)
		}
	})

	t.Run("SE-30_SubjectMismatch", func(t *testing.T) {
		signedFor := &Session{ID: "s1", UserID: "user-A", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound}
		svc := NewService(cfg, fs)
		tok, err := svc.CreateJWT(context.Background(), signedFor)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		fs.getFromDBSession = &Session{ID: "s1", UserID: "user-B", ExpiresAt: signedFor.ExpiresAt}

		if _, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok); !errors.Is(err, ErrSessionSubjectMismatch) {
			t.Fatalf("ReadJWT() error = %v, want %v", err, ErrSessionSubjectMismatch)
		}
	})

	t.Run("SE-31_UpdateSessionFailurePropagates", func(t *testing.T) {
		session := &Session{ID: "s1", UserID: "u1", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
		fs := &seFakeSessionStore{getFromCacheErr: ErrNotFound, getFromDBSession: session}
		svc := NewService(cfg, fs)
		tok, err := svc.CreateJWT(context.Background(), session)
		if err != nil {
			t.Fatalf("failed to build fixture token: %v", err)
		}
		fs.updateInDBErr = testutil.ErrDBDown

		got, err := svc.ReadJWT(testutil.LogContext(context.Background()), tok)
		if got != nil || !errors.Is(err, testutil.ErrDBDown) {
			t.Errorf("ReadJWT() = (%v, %v), want (nil, %v)", got, err, testutil.ErrDBDown)
		}
	})
}
