package session

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func stAssertRawConnectionError(t *testing.T, err error) {
	t.Helper()
	testutil.AssertRawConnectionError(t, err, ErrNotFound)
}

func stStoreWithUnreachableDB(t *testing.T) *store {
	return &store{db: testutil.UnreachablePool(t)}
}

func stStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{rdb: testutil.UnreachableRedis(t)}
}

func TestST19AddSessionToDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("SES-19_ConnectionErrorNoPanicFromDeferredCleanup", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddSessionToDB(testutil.LogContext(context.Background()), &Session{ID: "s1"}))
	})
}

func TestST20GetSessionFromDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("SES-20_ConnectionError", func(t *testing.T) {
		got, err := s.GetSessionFromDB(testutil.LogContext(context.Background()), "s1")
		if got != nil {
			t.Errorf("expected nil session, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST21DeleteSessionInDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("SES-21_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteSessionInDB(testutil.LogContext(context.Background()), "s1"))
	})
}

func TestST22UpdateSessionInDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("SES-22_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateSessionInDB(testutil.LogContext(context.Background()), &Session{ID: "s1"}))
	})
}

func TestST23ClearExpiredSessions(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("SES-23_SwallowsErrorWithoutPanicking", func(t *testing.T) {
		s.ClearExpiredSessions(testutil.LogContext(context.Background()))
	})
}

func TestST24to25AddSessionToCache(t *testing.T) {
	t.Run("SES-24_PastExpirySkipsRedisEntirely", func(t *testing.T) {
		s := &store{}
		err := s.AddSessionToCache(testutil.LogContext(context.Background()), &Session{ID: "s1", ExpiresAt: time.Now().Add(-time.Hour).Unix()})
		if err != nil {
			t.Errorf("AddSessionToCache() = %v, want nil", err)
		}
	})

	t.Run("SES-25_NeverExpiringAttemptsCacheWrite", func(t *testing.T) {
		s := stStoreWithUnreachableRedis(t)
		err := s.AddSessionToCache(testutil.LogContext(context.Background()), &Session{ID: "s1", ExpiresAt: 0})
		stAssertRawConnectionError(t, err)
	})
}

func TestST26GetSessionFromCache(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("SES-26_ConnectionErrorNotTreatedAsMiss", func(t *testing.T) {
		got, err := s.GetSessionFromCache(testutil.LogContext(context.Background()), "s1")
		if got != nil {
			t.Errorf("expected nil session, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST27DeleteSessionFromCache(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("SES-27_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteSessionFromCache(testutil.LogContext(context.Background()), "s1"))
	})
}

func stLiveSessionStore(t *testing.T) Store {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run these store tests")
	}

	pool, err := pgxpool.New(testutil.LogContext(context.Background()), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	t.Cleanup(pool.Close)

	return NewStore(pool, nil)
}

func TestST76_GetSessionFromDBNotFound(t *testing.T) {
	s := stLiveSessionStore(t)

	t.Run("SES-76_GetSessionFromDBNotFound", func(t *testing.T) {
		got, err := s.GetSessionFromDB(testutil.LogContext(context.Background()), "910000000000000101")
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetSessionFromDB() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
		}
	})
}

func TestST80_GetSessionFromCacheMiss(t *testing.T) {
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set; skipping live-Redis test")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("failed to parse TEST_REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { rdb.Close() })
	s := &store{rdb: rdb}

	t.Run("SES-80_GetSessionFromCacheNotFound", func(t *testing.T) {
		got, err := s.GetSessionFromCache(testutil.LogContext(context.Background()), "910000000000000201")
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetSessionFromCache() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
		}
	})
}
