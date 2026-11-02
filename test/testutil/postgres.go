package testutil

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresURL returns the connection URL for the named database on the server
// in TEST_POSTGRES_URL, and skips the test when the variable is unset.
func PostgresURL(t *testing.T, database string) string {
	t.Helper()
	raw := os.Getenv("TEST_POSTGRES_URL")
	if raw == "" {
		t.Skip("TEST_POSTGRES_URL not set; skipping live-Postgres test")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse TEST_POSTGRES_URL: %v", err)
	}
	u.Path, u.RawPath = "/"+database, ""
	return u.String()
}

// LivePool opens a pool on the named database and closes it when the test ends.
func LivePool(t *testing.T, database string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), PostgresURL(t, database))
	if err != nil {
		t.Fatalf("failed to connect to the %s database: %v", database, err)
	}
	t.Cleanup(pool.Close)
	return pool
}
