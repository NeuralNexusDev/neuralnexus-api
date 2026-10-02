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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
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
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrEmailAlreadyExists) || errors.Is(err, ErrUsernameAlreadyExists) || errors.Is(err, ErrAlreadyLinked) {
		t.Errorf("expected the raw connection error to pass through unchanged, got a translated sentinel: %v", err)
	}
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

func TestST02to07Accessors(t *testing.T) {
	s := &store{}
	cases := []struct {
		id string
		ok func() bool
	}{
		{"ST-02_Account", func() bool { v, ok := s.Account().(*store); return ok && v == s }},
		{"ST-03_AccountSettings", func() bool { v, ok := s.AccountSettings().(*store); return ok && v == s }},
		{"ST-04_Session", func() bool { v, ok := s.Session().(*store); return ok && v == s }},
		{"ST-05_LinkAccount", func() bool { v, ok := s.LinkAccount().(*store); return ok && v == s }},
		{"ST-06_RateLimit", func() bool { v, ok := s.RateLimit().(*store); return ok && v == s }},
		{"ST-07_OAuthToken", func() bool { v, ok := s.OAuthToken().(*store); return ok && v == s }},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if !c.ok() {
				t.Errorf("%s: accessor did not return the same *store instance", c.id)
			}
		})
	}
}

func TestST08to12TranslateAccountConstraintErr(t *testing.T) {
	t.Run("ST-08_NonPgError", func(t *testing.T) {
		orig := testerrors.ErrBoom
		if got := translateAccountConstraintErr(orig); !errors.Is(got, orig) {
			t.Errorf("translateAccountConstraintErr() = %v, want unchanged %v", got, orig)
		}
	})

	t.Run("ST-09_WrongCode", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23503", ConstraintName: "accounts_email_key"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, pgErr) {
			t.Errorf("translateAccountConstraintErr() = %v, want unchanged %v", got, pgErr)
		}
	})

	t.Run("ST-10_UnrecognizedConstraint", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "some_other_constraint"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, pgErr) {
			t.Errorf("translateAccountConstraintErr() = %v, want unchanged %v", got, pgErr)
		}
	})

	t.Run("ST-11_EmailConstraint", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "accounts_email_key"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, ErrEmailAlreadyExists) {
			t.Errorf("translateAccountConstraintErr() = %v, want ErrEmailAlreadyExists", got)
		}
	})

	t.Run("ST-12_UsernameConstraint", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "accounts_username_key"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, ErrUsernameAlreadyExists) {
			t.Errorf("translateAccountConstraintErr() = %v, want ErrUsernameAlreadyExists", got)
		}
	})
}

func TestST13AddAccountToDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-13_ConnectionErrorPassesThroughUnchanged", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddAccountToDB(&Account{UserID: "u1", Username: "alice"}))
	})
}

func TestST14GetAccountByID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-14_ConnectionError", func(t *testing.T) {
		got, err := s.GetAccountByID("u1")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST15GetAccountByUsername(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-15_ConnectionError", func(t *testing.T) {
		got, err := s.GetAccountByUsername("alice")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST16GetAccountByEmail(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-16_ConnectionError", func(t *testing.T) {
		got, err := s.GetAccountByEmail("a@b.com")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST17UpdateAccountInDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-17_ConnectionErrorPassesThroughUnchanged", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateAccountInDB(&Account{UserID: "u1"}))
	})
}

func TestST18DeleteAccountFromDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-18_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteAccountFromDB("u1"))
	})
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

func TestST28AddLinkedAccountToDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-28_ConnectionErrorNotMistranslated", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddLinkedAccountToDB(&LinkedAccount{UserID: "u1", Platform: PlatformDiscord, PlatformID: "p1"}))
	})
}

func TestST29UpdateLinkedAccount(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-29_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateLinkedAccount(&LinkedAccount{UserID: "u1", Platform: PlatformDiscord}))
	})
}

func TestST30GetLinkedAccountByPlatformID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-30_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountByPlatformID(PlatformDiscord, "p1")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST31GetLinkedAccountByPlatformName(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-31_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountByPlatformName(PlatformDiscord, "alice")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST32GetLinkedAccountByUserID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-32_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountByUserID("u1", PlatformDiscord)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST33GetLinkedAccountsByUserID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-33_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountsByUserID("u1")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST34DeleteLinkedAccount(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-34_BeginFailsBeforeGuardLogic", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteLinkedAccount("u1", PlatformDiscord))
	})
}

func TestST35to36SetLinkedAccountLoginEnabled(t *testing.T) {
	t.Run("ST-35_EnableConnectionError", func(t *testing.T) {
		s := stStoreWithUnreachableDB(t)
		stAssertRawConnectionError(t, s.SetLinkedAccountLoginEnabled("u1", PlatformDiscord, true))
	})

	t.Run("ST-36_DisableBeginFailsBeforeGuardLogic", func(t *testing.T) {
		s := stStoreWithUnreachableDB(t)
		stAssertRawConnectionError(t, s.SetLinkedAccountLoginEnabled("u1", PlatformDiscord, false))
	})
}

func TestST37GetAccountSettings(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-37_ConnectionErrorNotDefaultsFallback", func(t *testing.T) {
		got, err := s.GetAccountSettings("u1")
		if got != nil {
			t.Errorf("expected nil settings (not the DefaultAccountSettings fallback), got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST38to39SetPasswordAuthEnabled(t *testing.T) {
	t.Run("ST-38_EnableBeginFails", func(t *testing.T) {
		s := stStoreWithUnreachableDB(t)
		stAssertRawConnectionError(t, s.SetPasswordAuthEnabled("u1", true))
	})

	t.Run("ST-39_DisableBeginFails", func(t *testing.T) {
		s := stStoreWithUnreachableDB(t)
		stAssertRawConnectionError(t, s.SetPasswordAuthEnabled("u1", false))
	})
}

func TestST40GetRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("ST-40_ConnectionErrorNotTreatedAsMiss", func(t *testing.T) {
		got, err := s.GetRateLimit("k")
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST41SetRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("ST-41_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.SetRateLimit("k", 1))
	})
}

func TestST42IncrementRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("ST-42_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.IncrementRateLimit("k"))
	})
}

func TestST43AddOAuthTokenToDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-43_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddOAuthTokenToDB(&OAuthToken{UserID: "u1", Platform: PlatformDiscord}))
	})
}

func TestST44GetOAuthTokenByUserID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-44_ConnectionError", func(t *testing.T) {
		got, err := s.GetOAuthTokenByUserID("u1", PlatformDiscord)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST45UpdateOAuthToken(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-45_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateOAuthToken(&OAuthToken{UserID: "u1", Platform: PlatformDiscord}))
	})
}

func TestST46DeleteOAuthToken(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ST-46_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteOAuthToken("u1", PlatformDiscord))
	})
}

// stLiveStore's cleanup deletes rows in userID range
// 910000000000000000..910000000000049999, kept disjoint from other test
// files' ranges so it can't delete their data.
func stLiveStore(t *testing.T) (AccountStore, LinkAccountStore, AccountSettingsStore) {
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

	t.Cleanup(func() {
		db.Exec(context.Background(), "DELETE FROM sessions WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM oauth_tokens WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM account_settings WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM linked_accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id BETWEEN 910000000000000000 AND 910000000000049999")
		db.Close()
	})

	return NewStore(db, nil)
}

func stSeedBareAccount(t *testing.T, as AccountStore, userID string) {
	t.Helper()
	if err := as.AddAccountToDB(&Account{UserID: userID, Username: "sttest-" + userID}); err != nil {
		t.Fatalf("failed to seed account %s: %v", userID, err)
	}
}

func stSeedPasswordAccount(t *testing.T, as AccountStore, userID string) *Account {
	t.Helper()
	a := &Account{UserID: userID, Username: "sttest-" + userID}
	if err := a.HashPassword("sttest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account %s: %v", userID, err)
	}
	return a
}

func stSeedLink(t *testing.T, las LinkAccountStore, userID string, platform Platform, platformID string, verified, loginEnabled bool) {
	t.Helper()
	if err := las.AddLinkedAccountToDB(&LinkedAccount{
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

func TestST47GetAccountByID(t *testing.T) {
	as, _, _ := stLiveStore(t)
	t.Run("ST-47_NotFound", func(t *testing.T) {
		got, err := as.GetAccountByID("910000000000000001")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestST48GetAccountByUsername(t *testing.T) {
	as, _, _ := stLiveStore(t)
	t.Run("ST-48_NotFound", func(t *testing.T) {
		got, err := as.GetAccountByUsername("sttest-nonexistent")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestST49GetAccountByEmail(t *testing.T) {
	as, _, _ := stLiveStore(t)
	t.Run("ST-49_NotFound", func(t *testing.T) {
		got, err := as.GetAccountByEmail("sttest-nonexistent@example.test")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestST50AddAccountToDBNilEmailsDoNotCollide(t *testing.T) {
	as, _, _ := stLiveStore(t)
	t.Run("ST-50_NilEmailsDoNotCollide", func(t *testing.T) {
		a1 := &Account{UserID: "910000000000000002", Username: "sttest-noemail1"}
		if err := as.AddAccountToDB(a1); err != nil {
			t.Fatalf("first no-email AddAccountToDB returned error: %v", err)
		}
		a2 := &Account{UserID: "910000000000000003", Username: "sttest-noemail2"}
		if err := as.AddAccountToDB(a2); err != nil {
			t.Fatalf("second no-email AddAccountToDB returned error: %v", err)
		}
	})
}

func TestST51AddAccountToDBEmptyUsernamesDoNotCollide(t *testing.T) {
	as, _, _ := stLiveStore(t)
	t.Run("ST-51_EmptyUsernamesDoNotCollide", func(t *testing.T) {
		a1, err := NewIDOnlyAccount()
		if err != nil {
			t.Fatalf("NewIDOnlyAccount returned error: %v", err)
		}
		a1.UserID = "910000000000000004"
		if err := as.AddAccountToDB(a1); err != nil {
			t.Fatalf("first empty-username AddAccountToDB returned error: %v", err)
		}

		a2, err := NewIDOnlyAccount()
		if err != nil {
			t.Fatalf("NewIDOnlyAccount returned error: %v", err)
		}
		a2.UserID = "910000000000000005"
		if err := as.AddAccountToDB(a2); err != nil {
			t.Fatalf("second empty-username AddAccountToDB returned error (regression: an empty username must never collide): %v", err)
		}
	})
}

func TestST52AddAccountToDBDuplicateUsername(t *testing.T) {
	as, _, _ := stLiveStore(t)
	t.Run("ST-52_DuplicateUsernameTranslatesToSentinel", func(t *testing.T) {
		if err := as.AddAccountToDB(&Account{UserID: "910000000000000006", Username: "sttest-dupuser"}); err != nil {
			t.Fatalf("first insert returned error: %v", err)
		}
		err := as.AddAccountToDB(&Account{UserID: "910000000000000007", Username: "sttest-dupuser"})
		if !errors.Is(err, ErrUsernameAlreadyExists) {
			t.Errorf("expected ErrUsernameAlreadyExists, got %v", err)
		}
	})
}

func TestST53AddAccountToDBDuplicateEmail(t *testing.T) {
	as, _, _ := stLiveStore(t)
	t.Run("ST-53_DuplicateEmailTranslatesToSentinel", func(t *testing.T) {
		email := "sttest-dupemail@example.test"
		if err := as.AddAccountToDB(&Account{UserID: "910000000000000008", Username: "sttest-dupemail1", Email: &email}); err != nil {
			t.Fatalf("first insert returned error: %v", err)
		}
		err := as.AddAccountToDB(&Account{UserID: "910000000000000009", Username: "sttest-dupemail2", Email: &email})
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
		}
	})
}

func TestST54DeleteLinkedAccountBlockedAsLastLoginMethod(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-54_BlockedAsLastLoginMethod", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000010")
		stSeedLink(t, las, "910000000000000010", PlatformDiscord, "sttest-discord-10", true, true)

		err := las.DeleteLinkedAccount("910000000000000010", PlatformDiscord)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount, got: %v", err)
		}
		if _, err := las.GetLinkedAccountByUserID("910000000000000010", PlatformDiscord); err != nil {
			t.Errorf("expected the blocked link to still exist, got: %v", err)
		}
	})
}

func TestST55DeleteLinkedAccountAllowedWithPassword(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-55_AllowedWithPassword", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000011")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-11", true, true)

		if err := las.DeleteLinkedAccount(a.UserID, PlatformDiscord); err != nil {
			t.Fatalf("expected the delete to succeed, got: %v", err)
		}
		if _, err := las.GetLinkedAccountByUserID(a.UserID, PlatformDiscord); !errors.Is(err, ErrNotFound) {
			t.Errorf("expected the link to be gone, got: %v", err)
		}
	})
}

func TestST56DeleteLinkedAccountAllowedWithAnotherEnabledLink(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-56_AllowedWithAnotherEnabledLink", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000012")
		stSeedLink(t, las, "910000000000000012", PlatformDiscord, "sttest-discord-12", true, true)
		stSeedLink(t, las, "910000000000000012", PlatformTwitch, "sttest-twitch-12", true, true)

		if err := las.DeleteLinkedAccount("910000000000000012", PlatformDiscord); err != nil {
			t.Fatalf("expected the delete to succeed, got: %v", err)
		}
		if _, err := las.GetLinkedAccountByUserID("910000000000000012", PlatformTwitch); err != nil {
			t.Errorf("expected the Twitch link to remain untouched, got: %v", err)
		}
	})
}

func TestST57DeleteLinkedAccountNotFound(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-57_NotFound", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000013")
		err := las.DeleteLinkedAccount("910000000000000013", PlatformDiscord)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound for a platform that was never linked, got: %v", err)
		}
	})
}

func TestST58DeleteLinkedAccountIgnoresDisabledOtherLink(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-58_IgnoresDisabledOtherLink", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000014")
		stSeedLink(t, las, "910000000000000014", PlatformDiscord, "sttest-discord-14", true, true)
		stSeedLink(t, las, "910000000000000014", PlatformTwitch, "sttest-twitch-14", true, false)

		err := las.DeleteLinkedAccount("910000000000000014", PlatformDiscord)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount (the other link is disabled, so it doesn't count), got: %v", err)
		}
	})
}

func TestST59DeleteLinkedAccountBlockedWhenPasswordAuthDisabled(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-59_BlockedWhenPasswordAuthDisabled", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000015")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-15", true, true)
		if err := ass.SetPasswordAuthEnabled(a.UserID, false); err != nil {
			t.Fatalf("expected disabling password auth to succeed with a linked account present, got: %v", err)
		}

		err := las.DeleteLinkedAccount(a.UserID, PlatformDiscord)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount once password auth is disabled and this is the only link, got: %v", err)
		}
	})
}

func TestST60DeleteLinkedAccountConcurrentDifferentPlatformsNeverBothSucceed(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-60_ConcurrentDifferentPlatformsNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000001%04d", i)
			stSeedBareAccount(t, as, userID)
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)
			stSeedLink(t, las, userID, PlatformTwitch, "sttest-twitch-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() { defer wg.Done(); results[0] = las.DeleteLinkedAccount(userID, PlatformDiscord) }()
			go func() { defer wg.Done(); results[1] = las.DeleteLinkedAccount(userID, PlatformTwitch) }()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent deletes to succeed, got %d (results: %v)", i, successes, results)
			}

			remaining, err := las.GetLinkedAccountsByUserID(userID)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountsByUserID returned error: %v", i, err)
			}
			if len(remaining) != 1 {
				t.Fatalf("trial %d: expected exactly 1 link to remain (never 0), got %d", i, len(remaining))
			}
		}
	})
}

func TestST61SetLinkedAccountLoginEnabledEnable(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-61_EnableOnVerifiedRow", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000016")
		stSeedLink(t, las, "910000000000000016", PlatformDiscord, "sttest-discord-16", true, false)

		if err := las.SetLinkedAccountLoginEnabled("910000000000000016", PlatformDiscord, true); err != nil {
			t.Fatalf("expected the enable to succeed, got: %v", err)
		}
		la, err := las.GetLinkedAccountByUserID("910000000000000016", PlatformDiscord)
		if err != nil {
			t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
		}
		if !la.LoginEnabled {
			t.Error("expected LoginEnabled to be true")
		}
	})
}

func TestST62SetLinkedAccountLoginEnabledCannotEnableUnverified(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-62_CannotEnableUnverified", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000017")
		stSeedLink(t, las, "910000000000000017", PlatformDiscord, "sttest-discord-17", false, false)

		err := las.SetLinkedAccountLoginEnabled("910000000000000017", PlatformDiscord, true)
		if !errors.Is(err, ErrLinkedAccountUnverified) {
			t.Fatalf("expected ErrLinkedAccountUnverified, got: %v", err)
		}
	})
}

func TestST63SetLinkedAccountLoginEnabledBlockedAsLastLoginMethod(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-63_BlockedAsLastLoginMethod", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000018")
		stSeedLink(t, las, "910000000000000018", PlatformDiscord, "sttest-discord-18", true, true)

		err := las.SetLinkedAccountLoginEnabled("910000000000000018", PlatformDiscord, false)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount, got: %v", err)
		}
		la, err := las.GetLinkedAccountByUserID("910000000000000018", PlatformDiscord)
		if err != nil {
			t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
		}
		if !la.LoginEnabled {
			t.Error("expected LoginEnabled to remain true after a blocked disable")
		}
	})
}

func TestST64SetLinkedAccountLoginEnabledAllowedWithAnotherEnabledLink(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-64_AllowedWithAnotherEnabledLink", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000019")
		stSeedLink(t, las, "910000000000000019", PlatformDiscord, "sttest-discord-19", true, true)
		stSeedLink(t, las, "910000000000000019", PlatformTwitch, "sttest-twitch-19", true, true)

		if err := las.SetLinkedAccountLoginEnabled("910000000000000019", PlatformDiscord, false); err != nil {
			t.Fatalf("expected the disable to succeed, got: %v", err)
		}
		la, err := las.GetLinkedAccountByUserID("910000000000000019", PlatformDiscord)
		if err != nil {
			t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
		}
		if la.LoginEnabled {
			t.Error("expected LoginEnabled to be false")
		}
	})
}

func TestST65SetLinkedAccountLoginEnabledBlockedWhenPasswordAuthDisabled(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-65_BlockedWhenPasswordAuthDisabled", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000020")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-20", true, true)
		if err := ass.SetPasswordAuthEnabled(a.UserID, false); err != nil {
			t.Fatalf("expected disabling password auth to succeed, got: %v", err)
		}

		err := las.SetLinkedAccountLoginEnabled(a.UserID, PlatformDiscord, false)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount once password auth is disabled and this is the only enabled link, got: %v", err)
		}
	})
}

func TestST66SetLinkedAccountLoginEnabledConcurrentDifferentPlatformsNeverBothSucceed(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-66_ConcurrentDifferentPlatformsNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000002%04d", i)
			stSeedBareAccount(t, as, userID)
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)
			stSeedLink(t, las, userID, PlatformTwitch, "sttest-twitch-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() {
				defer wg.Done()
				results[0] = las.SetLinkedAccountLoginEnabled(userID, PlatformDiscord, false)
			}()
			go func() {
				defer wg.Done()
				results[1] = las.SetLinkedAccountLoginEnabled(userID, PlatformTwitch, false)
			}()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent disables to succeed, got %d (results: %v)", i, successes, results)
			}

			links, err := las.GetLinkedAccountsByUserID(userID)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountsByUserID returned error: %v", i, err)
			}
			enabledCount := 0
			for _, la := range links {
				if la.LoginEnabled {
					enabledCount++
				}
			}
			if enabledCount != 1 {
				t.Fatalf("trial %d: expected exactly 1 login-enabled link to remain (never 0), got %d", i, enabledCount)
			}
		}
	})
}

func TestST67GetAccountSettingsDefaultsWhenNoRow(t *testing.T) {
	as, _, ass := stLiveStore(t)
	t.Run("ST-67_DefaultsWhenNoRow", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000021")

		settings, err := ass.GetAccountSettings("910000000000000021")
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if !settings.PasswordAuthEnabled {
			t.Error("expected password auth to default to enabled with no account_settings row")
		}
	})
}

func TestST68SetPasswordAuthEnabledEnableBlockedWithNoPassword(t *testing.T) {
	as, _, ass := stLiveStore(t)
	t.Run("ST-68_EnableBlockedWithNoPassword", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000022")
		err := ass.SetPasswordAuthEnabled("910000000000000022", true)
		if !errors.Is(err, ErrNoPasswordSet) {
			t.Fatalf("expected ErrNoPasswordSet for an account with no password, got: %v", err)
		}
	})
}

func TestST69SetPasswordAuthEnabledEnableAllowedWithPassword(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-69_EnableAllowedWithPassword", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000023")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-23", true, true)

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

func TestST70SetPasswordAuthEnabledDisableBlockedAsLastLoginMethod(t *testing.T) {
	as, _, ass := stLiveStore(t)
	t.Run("ST-70_DisableBlockedAsLastLoginMethod", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000024")

		err := ass.SetPasswordAuthEnabled(a.UserID, false)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount for an account with no linked platform, got: %v", err)
		}
		settings, err := ass.GetAccountSettings(a.UserID)
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if !settings.PasswordAuthEnabled {
			t.Error("expected password auth to remain enabled after a blocked disable")
		}
	})
}

func TestST71SetPasswordAuthEnabledDisableAllowedWithLinkedAccount(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("ST-71_DisableAllowedWithLinkedAccount", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000025")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-25", true, true)

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
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() { defer wg.Done(); results[0] = ass.SetPasswordAuthEnabled(a.UserID, false) }()
			go func() { defer wg.Done(); results[1] = las.DeleteLinkedAccount(a.UserID, PlatformDiscord) }()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, ErrWouldLockAccount):
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
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() { defer wg.Done(); results[0] = ass.SetPasswordAuthEnabled(a.UserID, false) }()
			go func() {
				defer wg.Done()
				results[1] = las.SetLinkedAccountLoginEnabled(a.UserID, PlatformDiscord, false)
			}()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, ErrWouldLockAccount):
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
			la, err := las.GetLinkedAccountByUserID(userID, PlatformDiscord)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountByUserID returned error: %v", i, err)
			}
			if !settings.PasswordAuthEnabled && !la.LoginEnabled {
				t.Fatalf("trial %d: account left with NO usable login method (password disabled and link disabled)", i)
			}
		}
	})
}

func TestST74GetLinkedAccountsByUserID(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-74_ReturnsAllLinkedPlatforms", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000026")
		stSeedLink(t, las, "910000000000000026", PlatformDiscord, "sttest-discord-26", true, true)
		stSeedLink(t, las, "910000000000000026", PlatformTwitch, "sttest-twitch-26", true, true)

		links, err := las.GetLinkedAccountsByUserID("910000000000000026")
		if err != nil {
			t.Fatalf("GetLinkedAccountsByUserID returned error: %v", err)
		}
		if len(links) != 2 {
			t.Fatalf("expected 2 linked accounts, got %d: %+v", len(links), links)
		}
	})
}

func TestST75AddLinkedAccountToDBDuplicatePlatformID(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("ST-75_DuplicatePlatformIDTranslatesToSentinel", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000027")
		stSeedBareAccount(t, as, "910000000000000028")
		stSeedLink(t, las, "910000000000000027", PlatformDiscord, "sttest-dupplatform", true, true)

		err := las.AddLinkedAccountToDB(&LinkedAccount{
			UserID:           "910000000000000028",
			Platform:         PlatformDiscord,
			PlatformUsername: "sttest",
			PlatformID:       "sttest-dupplatform",
			Data:             map[string]string{},
			Verified:         true,
			LoginEnabled:     true,
		})
		if !errors.Is(err, ErrAlreadyLinked) {
			t.Errorf("expected ErrAlreadyLinked, got %v", err)
		}
	})
}

func TestST76to79_NotFoundSentinels(t *testing.T) {
	s := stLiveFullStore(t)

	t.Run("ST-76_GetSessionFromDBNotFound", func(t *testing.T) {
		got, err := s.Session().GetSessionFromDB("910000000000000101")
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetSessionFromDB() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
		}
	})

	t.Run("ST-77_GetLinkedAccountByPlatformNameNotFound", func(t *testing.T) {
		got, err := s.LinkAccount().GetLinkedAccountByPlatformName(PlatformDiscord, "sttest-missing-name")
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetLinkedAccountByPlatformName() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
		}
	})

	t.Run("ST-78_GetLinkedAccountByPlatformNameDuplicate", func(t *testing.T) {
		stSeedBareAccount(t, s.Account(), "910000000000000102")
		stSeedBareAccount(t, s.Account(), "910000000000000103")
		stSeedLink(t, s.LinkAccount(), "910000000000000102", PlatformDiscord, "sttest-dup-102", true, true)
		stSeedLink(t, s.LinkAccount(), "910000000000000103", PlatformDiscord, "sttest-dup-103", true, true)

		got, err := s.LinkAccount().GetLinkedAccountByPlatformName(PlatformDiscord, "sttest")
		if got != nil || !errors.Is(err, ErrDuplicateLinkedAccount) {
			t.Errorf("GetLinkedAccountByPlatformName() = (%v, %v), want (nil, %v)", got, err, ErrDuplicateLinkedAccount)
		}
	})

	t.Run("ST-79_GetOAuthTokenByUserIDNotFound", func(t *testing.T) {
		got, err := s.OAuthToken().GetOAuthTokenByUserID("910000000000000104", PlatformDiscord)
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetOAuthTokenByUserID() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
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

func TestST81to83_AccountSettingsUnknownUser(t *testing.T) {
	_, _, ass := stLiveStore(t)
	const unknownID = "910000000000000031"

	t.Run("ST-81_GetAccountSettingsUnknownUser", func(t *testing.T) {
		got, err := ass.GetAccountSettings(unknownID)
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetAccountSettings() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
		}
	})

	t.Run("ST-82_EnablePasswordAuthUnknownUser", func(t *testing.T) {
		err := ass.SetPasswordAuthEnabled(unknownID, true)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("SetPasswordAuthEnabled(true) err = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("ST-83_DisablePasswordAuthUnknownUser", func(t *testing.T) {
		err := ass.SetPasswordAuthEnabled(unknownID, false)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("SetPasswordAuthEnabled(false) err = %v, want %v", err, ErrNotFound)
		}
	})
}

func TestST84_SetPasswordAuthEnabledAccountDeletedConcurrently(t *testing.T) {
	full := stLiveFullStore(t)
	db := full.(*store).db
	a := stSeedPasswordAccount(t, full.Account(), "910000000000000032")
	ctx := context.Background()

	t.Run("ST-84_AccountDeletedWhileEnabling", func(t *testing.T) {
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin the deleting transaction: %v", err)
		}
		defer tx.Rollback(ctx)
		var deleterPID int32
		if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&deleterPID); err != nil {
			t.Fatalf("failed to read the deleting backend pid: %v", err)
		}
		if _, err := tx.Exec(ctx, "DELETE FROM accounts WHERE user_id = $1", a.UserID); err != nil {
			t.Fatalf("failed to delete the account: %v", err)
		}

		done := make(chan error, 1)
		go func() { done <- full.AccountSettings().SetPasswordAuthEnabled(a.UserID, true) }()

		deadline := time.Now().Add(5 * time.Second)
		for {
			var waiting int
			if err := db.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))", deleterPID).Scan(&waiting); err != nil {
				t.Fatalf("failed to read the blocked backends: %v", err)
			}
			if waiting > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("SetPasswordAuthEnabled never waited on the account row")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("failed to commit the delete: %v", err)
		}

		if err := <-done; !errors.Is(err, ErrNotFound) {
			t.Errorf("SetPasswordAuthEnabled() err = %v, want %v", err, ErrNotFound)
		}
	})
}

func TestST85to89AccountRoleIDs(t *testing.T) {
	as, _, _ := stLiveStore(t)

	t.Run("ST-85_RoleIDsRoundTrip", func(t *testing.T) {
		const userID = "910000000000000040"
		a := &Account{UserID: userID, Username: "sttest-" + userID, Roles: []string{"7", "900000000000000001"}}
		if err := as.AddAccountToDB(a); err != nil {
			t.Fatalf("AddAccountToDB() err = %v", err)
		}

		got, err := as.GetAccountByID(userID)
		if err != nil {
			t.Fatalf("GetAccountByID() err = %v", err)
		}
		if len(got.Roles) != 2 || got.Roles[0] != "7" || got.Roles[1] != "900000000000000001" {
			t.Errorf("Roles = %v, want [7 900000000000000001]", got.Roles)
		}
		byName, err := as.GetAccountByUsername("sttest-" + userID)
		if err != nil || len(byName.Roles) != 2 {
			t.Errorf("GetAccountByUsername() = (%v, %v), want the same two role IDs", byName, err)
		}
	})

	t.Run("ST-86_UpdateReplacesRoleIDs", func(t *testing.T) {
		const userID = "910000000000000041"
		stSeedBareAccount(t, as, userID)
		a, err := as.GetAccountByID(userID)
		if err != nil {
			t.Fatalf("GetAccountByID() err = %v", err)
		}
		a.Roles = []string{"3"}

		if err := as.UpdateAccountInDB(a); err != nil {
			t.Fatalf("UpdateAccountInDB() err = %v", err)
		}

		got, err := as.GetAccountByID(userID)
		if err != nil || len(got.Roles) != 1 || got.Roles[0] != "3" {
			t.Errorf("GetAccountByID() = (%v, %v), want role IDs [3]", got, err)
		}
	})

	t.Run("ST-87_NilRolesAreStoredAsEmpty", func(t *testing.T) {
		const userID = "910000000000000042"
		stSeedBareAccount(t, as, userID)

		got, err := as.GetAccountByID(userID)

		if err != nil || len(got.Roles) != 0 {
			t.Errorf("GetAccountByID() = (%v, %v), want no role IDs", got, err)
		}
	})

	t.Run("ST-88_NonNumericRoleIDRefusedOnAdd", func(t *testing.T) {
		const userID = "910000000000000043"

		err := as.AddAccountToDB(&Account{UserID: userID, Username: "sttest-" + userID, Roles: []string{"7", "admin"}})

		if !errors.Is(err, ErrInvalidRoleID) {
			t.Errorf("AddAccountToDB() err = %v, want %v", err, ErrInvalidRoleID)
		}
		if _, err := as.GetAccountByID(userID); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetAccountByID() err = %v, want %v (nothing stored)", err, ErrNotFound)
		}
	})

	t.Run("ST-89_NonNumericRoleIDRefusedOnUpdate", func(t *testing.T) {
		const userID = "910000000000000044"
		stSeedBareAccount(t, as, userID)
		a, _ := as.GetAccountByID(userID)
		a.Roles = []string{"0"}

		err := as.UpdateAccountInDB(a)

		if !errors.Is(err, ErrInvalidRoleID) {
			t.Errorf("UpdateAccountInDB() err = %v, want %v", err, ErrInvalidRoleID)
		}
	})
}
