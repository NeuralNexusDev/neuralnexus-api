package ratelimit

import (
	"fmt"
	"net"
	"testing"
	"time"

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

func stStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{rdb: stUnreachableRedis(t)}
}

func stAssertRawConnectionError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a non-nil connection error")
	}
}

func TestST01NewStore(t *testing.T) {
	t.Run("RLS-01_WrapsGivenClient", func(t *testing.T) {
		rdb := stUnreachableRedis(t)

		got := NewStore(rdb)
		s, ok := got.(*store)
		if !ok {
			t.Fatalf("NewStore() returned type %T, want *store", got)
		}
		if s.rdb != rdb {
			t.Error("NewStore() did not retain the given *redis.Client")
		}
	})
}

func TestST40GetRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("RLS-40_ConnectionErrorNotTreatedAsMiss", func(t *testing.T) {
		got, err := s.GetRateLimit("k")
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST41SetRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("RLS-41_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.SetRateLimit("k", 1))
	})
}

func TestST42IncrementRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("RLS-42_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.IncrementRateLimit("k"))
	})
}
