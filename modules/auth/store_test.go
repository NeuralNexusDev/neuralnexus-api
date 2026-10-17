package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
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

func stStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{rdb: stUnreachableRedis(t)}
}

func TestST01NewStore(t *testing.T) {
	t.Run("ST-01_WrapsGivenPoolAndClient", func(t *testing.T) {
		pool := stUnreachablePool(t)
		rdb := stUnreachableRedis(t)

		got := NewStore(pool, rdb)
		s, ok := got.(*store)
		if !ok {
			t.Fatalf("NewStore() returned type %T, want *store", got)
		}
		if s.db != pool {
			t.Error("NewStore() did not retain the given *pgxpool.Pool")
		}
		if s.rdb != rdb {
			t.Error("NewStore() did not retain the given *redis.Client")
		}
	})
}

func TestST02to05And07Accessors(t *testing.T) {
	s := &store{}
	cases := []struct {
		id string
		ok func() bool
	}{
		{"ST-02_Account", func() bool { return s.Account() != nil }},
		{"ST-03_AccountSettings", func() bool { return s.AccountSettings() != nil }},
		{"ST-04_Session", func() bool { return s.Session() != nil }},
		{"ST-05_LinkAccount", func() bool { return s.LinkAccount() != nil }},
		{"ST-07_OAuthToken", func() bool { return s.OAuthToken() != nil }},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if !c.ok() {
				t.Errorf("%s: accessor check failed", c.id)
			}
		})
	}
}

// stLiveLockKey serializes the live-database tests across packages, since each
// package's cleanup deletes rows in the same userID range.
const stLiveLockKey = 910000

// stLiveStore's cleanup deletes rows in userID range
// 910000000000000000..910000000000049999, so stLiveLockKey keeps the live
// tests of every package from running at once.
func stLiveStore(t *testing.T) (authaccount.AccountStore, linking.LinkAccountStore, authaccount.AccountSettingsStore) {
	t.Helper()
	s := stLiveFullStore(t)
	return s.Account(), s.LinkAccount(), s.AccountSettings()
}

func stLiveFullStore(t *testing.T) Store {
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
	if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_lock($1)", stLiveLockKey); err != nil {
		t.Fatalf("failed to take the live test lock: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(context.Background(), "DELETE FROM sessions WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM oauth_tokens WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM account_settings WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM linked_accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", stLiveLockKey)
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

	return NewStore(db, nil)
}

func stSeedBareAccount(t *testing.T, as authaccount.AccountStore, userID string) {
	t.Helper()
	if err := as.AddAccountToDB(&authaccount.Account{UserID: userID, Username: "sttest-" + userID}); err != nil {
		t.Fatalf("failed to seed account %s: %v", userID, err)
	}
}

func stSeedPasswordAccount(t *testing.T, as authaccount.AccountStore, userID string) *authaccount.Account {
	t.Helper()
	a := &authaccount.Account{UserID: userID, Username: "sttest-" + userID}
	if err := a.HashPassword("sttest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account %s: %v", userID, err)
	}
	return a
}

func stSeedLink(t *testing.T, las linking.LinkAccountStore, userID string, platform linking.Platform, platformID string, verified, loginEnabled bool) {
	t.Helper()
	if err := las.AddLinkedAccountToDB(&linking.LinkedAccount{
		UserID:           userID,
		Platform:         platform,
		PlatformUsername: "sttest",
		PlatformID:       platformID,
		Data:             map[string]string{},
		Verified:         verified,
		LoginEnabled:     loginEnabled,
	}); err != nil {
		t.Fatalf("failed to seed %s link for %s: %v", platform, userID, err)
	}
}

func TestST69SetPasswordAuthEnabledEnableAllowedWithPassword(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-69_EnableAllowedWithPassword", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000023")
		stSeedLink(t, las, a.UserID, linking.PlatformDiscord, "sttest-discord-23", true, true)

		if err := ass.SetPasswordAuthEnabled(a.UserID, false); err != nil {
			t.Fatalf("expected the disable to succeed, got: %v", err)
		}
		if err := ass.SetPasswordAuthEnabled(a.UserID, true); err != nil {
			t.Fatalf("expected the re-enable to succeed, got: %v", err)
		}
		settings, err := ass.GetAccountSettings(a.UserID)
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if !settings.PasswordAuthEnabled {
			t.Error("expected password auth to be re-enabled")
		}
	})
}

func TestST71SetPasswordAuthEnabledDisableAllowedWithLinkedAccount(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-71_DisableAllowedWithLinkedAccount", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000025")
		stSeedLink(t, las, a.UserID, linking.PlatformDiscord, "sttest-discord-25", true, true)

		if err := ass.SetPasswordAuthEnabled(a.UserID, false); err != nil {
			t.Fatalf("expected the disable to succeed, got: %v", err)
		}
		settings, err := ass.GetAccountSettings(a.UserID)
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if settings.PasswordAuthEnabled {
			t.Error("expected password auth to be disabled")
		}
	})
}

func TestST72SetPasswordAuthEnabledConcurrentWithDeleteLinkedAccountNeverBothSucceed(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-72_ConcurrentWithDeleteLinkedAccountNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000003%04d", i)
			a := stSeedPasswordAccount(t, as, userID)
			stSeedLink(t, las, userID, linking.PlatformDiscord, "sttest-discord-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() { defer wg.Done(); results[0] = ass.SetPasswordAuthEnabled(a.UserID, false) }()
			go func() { defer wg.Done(); results[1] = las.DeleteLinkedAccount(a.UserID, linking.PlatformDiscord) }()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, authaccount.ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent operations to succeed, got %d (results: %v)", i, successes, results)
			}

			settings, err := ass.GetAccountSettings(userID)
			if err != nil {
				t.Fatalf("trial %d: GetAccountSettings returned error: %v", i, err)
			}
			links, err := las.GetLinkedAccountsByUserID(userID)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountsByUserID returned error: %v", i, err)
			}
			if !settings.PasswordAuthEnabled && len(links) == 0 {
				t.Fatalf("trial %d: account left with NO usable login method (password disabled and link gone)", i)
			}
		}
	})
}

func TestST73SetPasswordAuthEnabledConcurrentWithSetLinkedAccountLoginEnabledNeverBothSucceed(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-73_ConcurrentWithSetLinkedAccountLoginEnabledNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000004%04d", i)
			a := stSeedPasswordAccount(t, as, userID)
			stSeedLink(t, las, userID, linking.PlatformDiscord, "sttest-discord-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() { defer wg.Done(); results[0] = ass.SetPasswordAuthEnabled(a.UserID, false) }()
			go func() {
				defer wg.Done()
				results[1] = las.SetLinkedAccountLoginEnabled(a.UserID, linking.PlatformDiscord, false)
			}()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, authaccount.ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent operations to succeed, got %d (results: %v)", i, successes, results)
			}

			settings, err := ass.GetAccountSettings(userID)
			if err != nil {
				t.Fatalf("trial %d: GetAccountSettings returned error: %v", i, err)
			}
			la, err := las.GetLinkedAccountByUserID(userID, linking.PlatformDiscord)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountByUserID returned error: %v", i, err)
			}
			if !settings.PasswordAuthEnabled && !la.LoginEnabled {
				t.Fatalf("trial %d: account left with NO usable login method (password disabled and link disabled)", i)
			}
		}
	})
}
