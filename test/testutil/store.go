// Package testutil holds helpers shared by the store tests. Only _test.go
// files import it.
package testutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// UnusedTCPPort returns a port unlikely to be reused, so a later connection
// to it fails fast with "connection refused" instead of hanging.
func UnusedTCPPort(t *testing.T) int {
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

// UnreachablePool returns a pgx pool pointed at a closed port.
func UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	port := UnusedTCPPort(t)
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

// UnreachableRedis returns a Redis client pointed at a closed port.
func UnreachableRedis(t *testing.T) *redis.Client {
	t.Helper()
	port := UnusedTCPPort(t)
	c := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", port),
		DialTimeout: 2 * time.Second,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// AssertRawConnectionError fails unless err is non-nil and matches none of the
// translated sentinels.
func AssertRawConnectionError(t *testing.T, err error, sentinels ...error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a non-nil connection error")
	}
	for _, s := range sentinels {
		if errors.Is(err, s) {
			t.Errorf("expected the raw connection error to pass through unchanged, got a translated sentinel: %v", err)
			return
		}
	}
}
