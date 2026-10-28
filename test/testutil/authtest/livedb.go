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

// LiveAuthPool returns a pool on the TEST_POSTGRES_URL server with the auth
// tables created, skipping the test when the variable is unset. It holds an
// advisory lock until the test ends, then deletes the rows in the user ID range
// 910000000000000000 to 910000000000049999 and closes the pool.
func LiveAuthPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run these store tests")
	}

	db, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	conn, err := db.Acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire a connection: %v", err)
	}
	if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_lock($1)", liveLockKey); err != nil {
		t.Fatalf("failed to take the live test lock: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(context.Background(), "DELETE FROM sessions WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM oauth_tokens WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM account_settings WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM linked_accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", liveLockKey)
		conn.Release()
		db.Close()
	})

	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS roles (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			user_id BIGINT PRIMARY KEY NOT NULL,
			username TEXT UNIQUE,
			email TEXT UNIQUE,
			hashed_secret BYTEA,
			salt BYTEA,
			role_ids BIGINT[] NOT NULL DEFAULT '{}',
			updated_at timestamp with time zone default current_timestamp
		)`,
		`CREATE TABLE IF NOT EXISTS linked_accounts (
			user_id BIGINT NOT NULL,
			platform TEXT NOT NULL,
			platform_username TEXT NOT NULL,
			platform_id TEXT NOT NULL,
			data JSONB NOT NULL,
			verified BOOLEAN NOT NULL DEFAULT true,
			login_enabled BOOLEAN NOT NULL DEFAULT true,
			created_at timestamp with time zone default current_timestamp,
			updated_at timestamp with time zone default current_timestamp,
			FOREIGN KEY (user_id) REFERENCES accounts(user_id),
			CONSTRAINT linked_accounts_unique UNIQUE (user_id, platform),
			CONSTRAINT linked_accounts_platform_unique UNIQUE (platform, platform_id)
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
		`CREATE TABLE IF NOT EXISTS oauth_tokens (
			user_id BIGINT NOT NULL,
			platform TEXT NOT NULL,
			access_token TEXT NOT NULL,
			token_type TEXT,
			refresh_token TEXT,
			expiry BIGINT,
			expires_in BIGINT,
			scope TEXT[],
			created_at timestamp with time zone default current_timestamp,
			updated_at timestamp with time zone default current_timestamp,
			FOREIGN KEY (user_id) REFERENCES accounts(user_id),
			CONSTRAINT oauth_tokens_unique UNIQUE (user_id, platform)
		)`,
		`CREATE TABLE IF NOT EXISTS account_settings (
			user_id BIGINT PRIMARY KEY NOT NULL REFERENCES accounts(user_id),
			password_auth BOOLEAN NOT NULL DEFAULT true,
			updated_at timestamp with time zone default current_timestamp
		)`,
	} {
		if _, err := db.Exec(context.Background(), ddl); err != nil {
			t.Fatalf("failed to apply schema: %v", err)
		}
	}

	return db
}
