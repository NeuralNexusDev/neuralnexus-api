package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// stUnusedTCPPort returns a port unlikely to be reused, so a later connection
// to it fails fast with "connection refused" instead of hanging.
func stUnusedTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find an unused port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close probe listener: %v", err)
	}
	return port
}

func stUnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	port := stUnusedTCPPort(t)
	cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://user:pass@127.0.0.1:%d/db?sslmode=disable&connect_timeout=2", port))
	if err != nil {
		t.Fatalf("failed to parse pool config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func stUnreachableRedis(t *testing.T) *redis.Client {
	t.Helper()
	port := stUnusedTCPPort(t)
	c := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", port),
		DialTimeout: 2 * time.Second,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func stStoreWithUnreachableDB(t *testing.T) *store {
	return &store{db: stUnreachablePool(t)}
}

func stStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{rdb: stUnreachableRedis(t)}
}

func stAssertRawConnectionError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a non-nil connection error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("expected the raw connection error to pass through unchanged, got a translated sentinel: %v", err)
	}
}

func TestST19AddSessionToDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-19_ConnectionErrorNoPanicFromDeferredCleanup", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddSessionToDB(&Session{ID: "s1"}))
	})
}

func TestST20GetSessionFromDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-20_ConnectionError", func(t *testing.T) {
		got, err := s.GetSessionFromDB("s1")
		if got != nil {
			t.Errorf("expected nil session, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST21DeleteSessionInDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-21_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteSessionInDB("s1"))
	})
}

func TestST22UpdateSessionInDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-22_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateSessionInDB(&Session{ID: "s1"}))
	})
}

func TestST23ClearExpiredSessions(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-23_SwallowsErrorWithoutPanicking", func(t *testing.T) {
		s.ClearExpiredSessions()
	})
}

func TestST24to25AddSessionToCache(t *testing.T) {
	t.Run("ST-24_PastExpirySkipsRedisEntirely", func(t *testing.T) {
		s := &store{}
		err := s.AddSessionToCache(&Session{ID: "s1", ExpiresAt: time.Now().Add(-time.Hour).Unix()})
		if err != nil {
			t.Errorf("AddSessionToCache() = %v, want nil", err)
		}
	})

	t.Run("ST-25_NeverExpiringAttemptsCacheWrite", func(t *testing.T) {
		s := stStoreWithUnreachableRedis(t)
		err := s.AddSessionToCache(&Session{ID: "s1", ExpiresAt: 0})
		stAssertRawConnectionError(t, err)
	})
}

func TestST26GetSessionFromCache(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("ST-26_ConnectionErrorNotTreatedAsMiss", func(t *testing.T) {
		got, err := s.GetSessionFromCache("s1")
		if got != nil {
			t.Errorf("expected nil session, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST27DeleteSessionFromCache(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("ST-27_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteSessionFromCache("s1"))
	})
}

func stLiveSessionStore(t *testing.T) Store {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run these store tests")
	}

	db, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			user_id BIGINT PRIMARY KEY NOT NULL,
			username TEXT UNIQUE,
			email TEXT UNIQUE,
			hashed_secret BYTEA,
			salt BYTEA,
			role_ids BIGINT[] NOT NULL DEFAULT '{}',
			updated_at timestamp with time zone default current_timestamp
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			session_id BIGINT PRIMARY KEY NOT NULL,
			user_id BIGINT NOT NULL,
			permissions TEXT[] NOT NULL,
			iat BIGINT NOT NULL,
			lua BIGINT NOT NULL,
			exp BIGINT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES accounts(user_id)
		)`,
	} {
		if _, err := db.Exec(context.Background(), ddl); err != nil {
			db.Close()
			t.Fatalf("failed to create test schema: %v", err)
		}
	}
	t.Cleanup(db.Close)

	return NewStore(db, nil)
}

func TestST76_GetSessionFromDBNotFound(t *testing.T) {
	s := stLiveSessionStore(t)

	t.Run("ST-76_GetSessionFromDBNotFound", func(t *testing.T) {
		got, err := s.GetSessionFromDB("910000000000000101")
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

	t.Run("ST-80_GetSessionFromCacheNotFound", func(t *testing.T) {
		got, err := s.GetSessionFromCache("910000000000000201")
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetSessionFromCache() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
		}
	})
}
