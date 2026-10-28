package authtest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// liveLockKey serializes the live-database tests across packages, since each
// package's cleanup deletes rows in the same user ID range.
const liveLockKey = 910000

// LiveAuthPool returns a pool on the TEST_POSTGRES_URL server skipping the test when the variable
// is unset. It holds an advisory lock until the test ends, then deletes the rows in the user ID range
// 910000000000000000 to 910000000000049999 and closes the pool.
func LiveAuthPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run these store tests")
	}

	pool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire a connection: %v", err)
	}
	if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_lock($1)", liveLockKey); err != nil {
		t.Fatalf("failed to take the live test lock: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM sessions WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		pool.Exec(context.Background(), "DELETE FROM oauth_tokens WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		pool.Exec(context.Background(), "DELETE FROM account_settings WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		pool.Exec(context.Background(), "DELETE FROM linked_accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		pool.Exec(context.Background(), "DELETE FROM accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", liveLockKey)
		conn.Release()
		pool.Close()
	})

	return pool
}
