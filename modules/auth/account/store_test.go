package account

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func stAssertRawConnectionError(t *testing.T, err error) {
	t.Helper()
	testutil.AssertRawConnectionError(t, err, ErrNotFound, ErrEmailAlreadyExists, ErrUsernameAlreadyExists)
}

func stStoreWithUnreachableDB(t *testing.T) *store {
	return &store{db: testutil.UnreachablePool(t)}
}

// stLiveLockKey serializes the live-database tests across packages, since each
// package's cleanup deletes rows in the same userID range.
const stLiveLockKey = 910000

// stLiveStore's cleanup deletes rows in userID range
// 910000000000000000..910000000000049999, so stLiveLockKey keeps the live
// tests of every package from running at once.
func stLiveStore(t *testing.T) (Store, SettingsStore) {
	t.Helper()
	s := stLiveFullStore(t)
	return s, NewSettingsStore(s.db)
}

func stLiveFullStore(t *testing.T) *store {
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

	return &store{db: db}
}

func stSeedRoles(t *testing.T, db *pgxpool.Pool, ids ...int64) {
	t.Helper()
	ctx := context.Background()
	for _, id := range ids {
		if _, err := db.Exec(ctx, "INSERT INTO roles (id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING", id, fmt.Sprintf("sttest_role_%d", id)); err != nil {
			t.Fatalf("failed to seed role %d: %v", id, err)
		}
	}
}

func stSeedBareAccount(t *testing.T, as Store, userID string) {
	t.Helper()
	if err := as.AddAccountToDB(context.Background(), &Account{UserID: userID, Username: "sttest-" + userID}); err != nil {
		t.Fatalf("failed to seed account %s: %v", userID, err)
	}
}

func stSeedPasswordAccount(t *testing.T, as Store, userID string) *Account {
	t.Helper()
	a := &Account{UserID: userID, Username: "sttest-" + userID}
	if err := a.HashPassword(testutil.NewConfig().Auth.Account.Pepper, "sttest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(context.Background(), a); err != nil {
		t.Fatalf("failed to seed passworded account %s: %v", userID, err)
	}
	return a
}

func TestST08to12TranslateAccountConstraintErr(t *testing.T) {
	t.Run("ACS-08_NonPgError", func(t *testing.T) {
		orig := testutil.ErrBoom
		if got := translateAccountConstraintErr(orig); !errors.Is(got, orig) {
			t.Errorf("translateAccountConstraintErr() = %v, want unchanged %v", got, orig)
		}
	})

	t.Run("ACS-09_WrongCode", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23503", ConstraintName: "accounts_email_key"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, pgErr) {
			t.Errorf("translateAccountConstraintErr() = %v, want unchanged %v", got, pgErr)
		}
	})

	t.Run("ACS-10_UnrecognizedConstraint", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "some_other_constraint"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, pgErr) {
			t.Errorf("translateAccountConstraintErr() = %v, want unchanged %v", got, pgErr)
		}
	})

	t.Run("ACS-11_EmailConstraint", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "accounts_email_key"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, ErrEmailAlreadyExists) {
			t.Errorf("translateAccountConstraintErr() = %v, want ErrEmailAlreadyExists", got)
		}
	})

	t.Run("ACS-12_UsernameConstraint", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "accounts_username_key"}
		got := translateAccountConstraintErr(pgErr)
		if !errors.Is(got, ErrUsernameAlreadyExists) {
			t.Errorf("translateAccountConstraintErr() = %v, want ErrUsernameAlreadyExists", got)
		}
	})
}

func TestST13AddAccountToDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ACS-13_ConnectionErrorPassesThroughUnchanged", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddAccountToDB(context.Background(), &Account{UserID: "u1", Username: "alice"}))
	})
}

func TestST14GetAccountByID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ACS-14_ConnectionError", func(t *testing.T) {
		got, err := s.GetAccountByID(context.Background(), "u1")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST15GetAccountByUsername(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ACS-15_ConnectionError", func(t *testing.T) {
		got, err := s.GetAccountByUsername(context.Background(), "alice")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST16GetAccountByEmail(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ACS-16_ConnectionError", func(t *testing.T) {
		got, err := s.GetAccountByEmail(context.Background(), "a@b.com")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST17UpdateAccountInDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ACS-17_ConnectionErrorPassesThroughUnchanged", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateAccountInDB(context.Background(), &Account{UserID: "u1"}))
	})
}

func TestST18DeleteAccountFromDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("ACS-18_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteAccountFromDB(context.Background(), "u1"))
	})
}

func TestST37GetAccountSettings(t *testing.T) {
	s := &settingsStore{db: testutil.UnreachablePool(t)}
	t.Run("ACS-37_ConnectionErrorNotDefaultsFallback", func(t *testing.T) {
		got, err := s.GetAccountSettings(context.Background(), "u1")
		if got != nil {
			t.Errorf("expected nil settings (not the DefaultAccountSettings fallback), got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestST38to39SetPasswordAuthEnabled(t *testing.T) {
	t.Run("ACS-38_EnableBeginFails", func(t *testing.T) {
		s := &settingsStore{db: testutil.UnreachablePool(t)}
		stAssertRawConnectionError(t, s.SetPasswordAuthEnabled(context.Background(), "u1", true))
	})

	t.Run("ACS-39_DisableBeginFails", func(t *testing.T) {
		s := &settingsStore{db: testutil.UnreachablePool(t)}
		stAssertRawConnectionError(t, s.SetPasswordAuthEnabled(context.Background(), "u1", false))
	})
}

func TestST47GetAccountByID(t *testing.T) {
	as, _ := stLiveStore(t)
	t.Run("ACS-47_NotFound", func(t *testing.T) {
		got, err := as.GetAccountByID(context.Background(), "910000000000000001")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestST48GetAccountByUsername(t *testing.T) {
	as, _ := stLiveStore(t)
	t.Run("ACS-48_NotFound", func(t *testing.T) {
		got, err := as.GetAccountByUsername(context.Background(), "sttest-nonexistent")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestST49GetAccountByEmail(t *testing.T) {
	as, _ := stLiveStore(t)
	t.Run("ACS-49_NotFound", func(t *testing.T) {
		got, err := as.GetAccountByEmail(context.Background(), "sttest-nonexistent@example.test")
		if got != nil {
			t.Errorf("expected nil account, got %v", got)
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestST50AddAccountToDBNilEmailsDoNotCollide(t *testing.T) {
	as, _ := stLiveStore(t)
	t.Run("ACS-50_NilEmailsDoNotCollide", func(t *testing.T) {
		a1 := &Account{UserID: "910000000000000002", Username: "sttest-noemail1"}
		if err := as.AddAccountToDB(context.Background(), a1); err != nil {
			t.Fatalf("first no-email AddAccountToDB returned error: %v", err)
		}
		a2 := &Account{UserID: "910000000000000003", Username: "sttest-noemail2"}
		if err := as.AddAccountToDB(context.Background(), a2); err != nil {
			t.Fatalf("second no-email AddAccountToDB returned error: %v", err)
		}
	})
}

func TestST51AddAccountToDBEmptyUsernamesDoNotCollide(t *testing.T) {
	as, _ := stLiveStore(t)
	t.Run("ACS-51_EmptyUsernamesDoNotCollide", func(t *testing.T) {
		a1, err := NewIDOnlyAccount()
		if err != nil {
			t.Fatalf("NewIDOnlyAccount returned error: %v", err)
		}
		a1.UserID = "910000000000000004"
		if err := as.AddAccountToDB(context.Background(), a1); err != nil {
			t.Fatalf("first empty-username AddAccountToDB returned error: %v", err)
		}

		a2, err := NewIDOnlyAccount()
		if err != nil {
			t.Fatalf("NewIDOnlyAccount returned error: %v", err)
		}
		a2.UserID = "910000000000000005"
		if err := as.AddAccountToDB(context.Background(), a2); err != nil {
			t.Fatalf("second empty-username AddAccountToDB returned error (regression: an empty username must never collide): %v", err)
		}
	})
}

func TestST52AddAccountToDBDuplicateUsername(t *testing.T) {
	as, _ := stLiveStore(t)
	t.Run("ACS-52_DuplicateUsernameTranslatesToSentinel", func(t *testing.T) {
		if err := as.AddAccountToDB(context.Background(), &Account{UserID: "910000000000000006", Username: "sttest-dupuser"}); err != nil {
			t.Fatalf("first insert returned error: %v", err)
		}
		err := as.AddAccountToDB(context.Background(), &Account{UserID: "910000000000000007", Username: "sttest-dupuser"})
		if !errors.Is(err, ErrUsernameAlreadyExists) {
			t.Errorf("expected ErrUsernameAlreadyExists, got %v", err)
		}
	})
}

func TestST53AddAccountToDBDuplicateEmail(t *testing.T) {
	as, _ := stLiveStore(t)
	t.Run("ACS-53_DuplicateEmailTranslatesToSentinel", func(t *testing.T) {
		email := "sttest-dupemail@example.test"
		if err := as.AddAccountToDB(context.Background(), &Account{UserID: "910000000000000008", Username: "sttest-dupemail1", Email: &email}); err != nil {
			t.Fatalf("first insert returned error: %v", err)
		}
		err := as.AddAccountToDB(context.Background(), &Account{UserID: "910000000000000009", Username: "sttest-dupemail2", Email: &email})
		if !errors.Is(err, ErrEmailAlreadyExists) {
			t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
		}
	})
}

func TestST67GetAccountSettingsDefaultsWhenNoRow(t *testing.T) {
	as, ass := stLiveStore(t)
	t.Run("ACS-67_DefaultsWhenNoRow", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000021")

		settings, err := ass.GetAccountSettings(context.Background(), "910000000000000021")
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if !settings.PasswordAuthEnabled {
			t.Error("expected password auth to default to enabled with no account_settings row")
		}
	})
}

func TestST68SetPasswordAuthEnabledEnableBlockedWithNoPassword(t *testing.T) {
	as, ass := stLiveStore(t)
	t.Run("ACS-68_EnableBlockedWithNoPassword", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000022")
		err := ass.SetPasswordAuthEnabled(context.Background(), "910000000000000022", true)
		if !errors.Is(err, ErrNoPasswordSet) {
			t.Fatalf("expected ErrNoPasswordSet for an account with no password, got: %v", err)
		}
	})
}

func TestST70SetPasswordAuthEnabledDisableBlockedAsLastLoginMethod(t *testing.T) {
	as, ass := stLiveStore(t)
	t.Run("ACS-70_DisableBlockedAsLastLoginMethod", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000024")

		err := ass.SetPasswordAuthEnabled(context.Background(), a.UserID, false)
		if !errors.Is(err, ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount for an account with no linked platform, got: %v", err)
		}
		settings, err := ass.GetAccountSettings(context.Background(), a.UserID)
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if !settings.PasswordAuthEnabled {
			t.Error("expected password auth to remain enabled after a blocked disable")
		}
	})
}

func TestST81to83_AccountSettingsUnknownUser(t *testing.T) {
	_, ass := stLiveStore(t)
	const unknownID = "910000000000000031"

	t.Run("ACS-81_GetAccountSettingsUnknownUser", func(t *testing.T) {
		got, err := ass.GetAccountSettings(context.Background(), unknownID)
		if got != nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("GetAccountSettings() = (%v, %v), want (nil, %v)", got, err, ErrNotFound)
		}
	})

	t.Run("ACS-82_EnablePasswordAuthUnknownUser", func(t *testing.T) {
		err := ass.SetPasswordAuthEnabled(context.Background(), unknownID, true)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("SetPasswordAuthEnabled(true) err = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("ACS-83_DisablePasswordAuthUnknownUser", func(t *testing.T) {
		err := ass.SetPasswordAuthEnabled(context.Background(), unknownID, false)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("SetPasswordAuthEnabled(false) err = %v, want %v", err, ErrNotFound)
		}
	})
}

func TestST84_SetPasswordAuthEnabledAccountDeletedConcurrently(t *testing.T) {
	full := stLiveFullStore(t)
	db := full.db
	a := stSeedPasswordAccount(t, full, "910000000000000032")
	ctx := context.Background()

	t.Run("ACS-84_AccountDeletedWhileEnabling", func(t *testing.T) {
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
		go func() { done <- NewSettingsStore(db).SetPasswordAuthEnabled(context.Background(), a.UserID, true) }()

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

func TestST85to95AccountRoleIDs(t *testing.T) {
	full := stLiveFullStore(t)
	as := full
	stSeedRoles(t, full.db, 910000000000000301, 910000000000000302, 910000000000000303)

	t.Run("ACS-85_RoleIDsRoundTrip", func(t *testing.T) {
		const userID = "910000000000000040"
		email := "sttest-" + userID + "@example.com"
		a := &Account{UserID: userID, Username: "sttest-" + userID, Email: &email, Roles: []string{"910000000000000301", "910000000000000302"}}
		if err := as.AddAccountToDB(context.Background(), a); err != nil {
			t.Fatalf("AddAccountToDB() err = %v", err)
		}

		got, err := as.GetAccountByID(context.Background(), userID)
		if err != nil {
			t.Fatalf("GetAccountByID() err = %v", err)
		}
		if len(got.Roles) != 2 || got.Roles[0] != "910000000000000301" || got.Roles[1] != "910000000000000302" {
			t.Errorf("Roles = %v, want [910000000000000301 910000000000000302]", got.Roles)
		}
		byName, err := as.GetAccountByUsername(context.Background(), "sttest-"+userID)
		if err != nil || len(byName.Roles) != 2 {
			t.Errorf("GetAccountByUsername() = (%v, %v), want the same two role IDs", byName, err)
		}
		byEmail, err := as.GetAccountByEmail(context.Background(), email)
		if err != nil || len(byEmail.Roles) != 2 {
			t.Errorf("GetAccountByEmail() = (%v, %v), want the same two role IDs", byEmail, err)
		}
	})

	t.Run("ACS-86_UpdateReplacesRoleIDs", func(t *testing.T) {
		const userID = "910000000000000041"
		stSeedBareAccount(t, as, userID)
		a, err := as.GetAccountByID(context.Background(), userID)
		if err != nil {
			t.Fatalf("GetAccountByID() err = %v", err)
		}
		a.Roles = []string{"910000000000000303"}

		if err := as.UpdateAccountInDB(context.Background(), a); err != nil {
			t.Fatalf("UpdateAccountInDB() err = %v", err)
		}

		got, err := as.GetAccountByID(context.Background(), userID)
		if err != nil || len(got.Roles) != 1 || got.Roles[0] != "910000000000000303" {
			t.Errorf("GetAccountByID() = (%v, %v), want role IDs [910000000000000303]", got, err)
		}
	})

	t.Run("ACS-87_NilRolesAreStoredAsEmpty", func(t *testing.T) {
		const userID = "910000000000000042"
		stSeedBareAccount(t, as, userID)

		got, err := as.GetAccountByID(context.Background(), userID)

		if err != nil || len(got.Roles) != 0 {
			t.Errorf("GetAccountByID() = (%v, %v), want no role IDs", got, err)
		}
	})

	t.Run("ACS-88_NonNumericRoleIDRefusedOnAdd", func(t *testing.T) {
		const userID = "910000000000000043"

		err := as.AddAccountToDB(context.Background(), &Account{UserID: userID, Username: "sttest-" + userID, Roles: []string{"910000000000000301", "admin"}})

		if !errors.Is(err, ErrInvalidRoleID) {
			t.Errorf("AddAccountToDB() err = %v, want %v", err, ErrInvalidRoleID)
		}
		if _, err := as.GetAccountByID(context.Background(), userID); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetAccountByID() err = %v, want %v (nothing stored)", err, ErrNotFound)
		}
	})

	t.Run("ACS-89_NonNumericRoleIDRefusedOnUpdate", func(t *testing.T) {
		const userID = "910000000000000044"
		stSeedBareAccount(t, as, userID)
		a, _ := as.GetAccountByID(context.Background(), userID)
		a.Roles = []string{"0"}

		err := as.UpdateAccountInDB(context.Background(), a)

		if !errors.Is(err, ErrInvalidRoleID) {
			t.Errorf("UpdateAccountInDB() err = %v, want %v", err, ErrInvalidRoleID)
		}
	})

	t.Run("ACS-91_UnknownRoleIDRefusedOnAdd", func(t *testing.T) {
		const userID = "910000000000000045"

		err := as.AddAccountToDB(context.Background(), &Account{UserID: userID, Username: "sttest-" + userID, Roles: []string{"910000000000000301", "910000000000000399"}})

		if !errors.Is(err, ErrUnknownRoleID) {
			t.Errorf("AddAccountToDB() err = %v, want %v", err, ErrUnknownRoleID)
		}
		if _, err := as.GetAccountByID(context.Background(), userID); !errors.Is(err, ErrNotFound) {
			t.Errorf("GetAccountByID() err = %v, want %v (nothing stored)", err, ErrNotFound)
		}
	})

	t.Run("ACS-92_UnknownRoleIDRefusedOnUpdate", func(t *testing.T) {
		const userID = "910000000000000046"
		stSeedBareAccount(t, as, userID)
		a, _ := as.GetAccountByID(context.Background(), userID)
		a.Roles = []string{"910000000000000399"}

		err := as.UpdateAccountInDB(context.Background(), a)

		if !errors.Is(err, ErrUnknownRoleID) {
			t.Errorf("UpdateAccountInDB() err = %v, want %v", err, ErrUnknownRoleID)
		}
		if got, _ := as.GetAccountByID(context.Background(), userID); len(got.Roles) != 0 {
			t.Errorf("Roles = %v, want none stored", got.Roles)
		}
	})

	t.Run("ACS-93_NonCanonicalRoleIDsRefused", func(t *testing.T) {
		const userID = "910000000000000047"
		stSeedBareAccount(t, as, userID)
		a, _ := as.GetAccountByID(context.Background(), userID)
		for _, id := range []string{"0910000000000000301", "+910000000000000301", " 910000000000000301"} {
			a.Roles = []string{id}
			if err := as.UpdateAccountInDB(context.Background(), a); !errors.Is(err, ErrInvalidRoleID) {
				t.Errorf("UpdateAccountInDB() with %q err = %v, want %v", id, err, ErrInvalidRoleID)
			}
		}
	})

	t.Run("ACS-95_AssignmentWaitsForAnInFlightRoleDeleteAndThenRefuses", func(t *testing.T) {
		const userID = "910000000000000049"
		const roleID = 910000000000000311
		db := full.db
		stSeedRoles(t, db, roleID)
		ctx := context.Background()
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin: %v", err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, "DELETE FROM roles WHERE id = $1", roleID); err != nil {
			t.Fatalf("failed to delete the role: %v", err)
		}

		done := make(chan error, 1)
		go func() {
			done <- as.AddAccountToDB(context.Background(), &Account{UserID: userID, Username: "sttest-" + userID, Roles: []string{"910000000000000311"}})
		}()
		select {
		case err := <-done:
			t.Fatalf("AddAccountToDB() returned %v before the role delete committed", err)
		case <-time.After(300 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		if err := <-done; !errors.Is(err, ErrUnknownRoleID) {
			t.Errorf("AddAccountToDB() err = %v, want %v", err, ErrUnknownRoleID)
		}
	})

	t.Run("ACS-94_RepeatedRoleIDIsStoredOnce", func(t *testing.T) {
		const userID = "910000000000000048"
		stSeedBareAccount(t, as, userID)
		a, _ := as.GetAccountByID(context.Background(), userID)
		a.Roles = []string{"910000000000000301", "910000000000000301"}

		if err := as.UpdateAccountInDB(context.Background(), a); err != nil {
			t.Fatalf("UpdateAccountInDB() err = %v, want nil", err)
		}

		got, err := as.GetAccountByID(context.Background(), userID)
		if err != nil || len(got.Roles) != 1 || got.Roles[0] != "910000000000000301" {
			t.Errorf("GetAccountByID() = (%v, %v), want the role ID once", got, err)
		}
	})
}

func TestST96to98ListAccounts(t *testing.T) {
	full := stLiveFullStore(t)
	as := full
	db := full.db
	const shortID = "95"
	cleanShort := func() { _, _ = db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id = 95") }
	cleanShort()
	t.Cleanup(cleanShort)
	stSeedBareAccount(t, as, shortID)
	stSeedBareAccount(t, as, "910000000000000602")
	stSeedBareAccount(t, as, "910000000000000601")
	if err := as.AddAccountToDB(context.Background(), &Account{UserID: "910000000000000603"}); err != nil {
		t.Fatalf("failed to seed the account without a username: %v", err)
	}

	indexOf := func(accounts []*Account, userID string) int {
		for i, a := range accounts {
			if a.UserID == userID {
				return i
			}
		}
		return -1
	}

	t.Run("ACS-96_OrdersByNumericID", func(t *testing.T) {
		all, err := as.ListAccounts(context.Background(), 100000, 0)
		if err != nil {
			t.Fatalf("ListAccounts() err = %v", err)
		}
		short, first, second := indexOf(all, shortID), indexOf(all, "910000000000000601"), indexOf(all, "910000000000000602")
		if short < 0 || first < 0 || second < 0 || !(short < first && first < second) {
			t.Errorf("positions of 95, ...601, ...602 = %d, %d, %d, want ascending by number", short, first, second)
		}
		for _, a := range all {
			if a.Roles == nil {
				t.Fatalf("account %s has nil Roles, want an empty list", a.UserID)
			}
		}
		noName := indexOf(all, "910000000000000603")
		if noName < 0 {
			t.Fatalf("the account stored without a username is not listed")
		}
		if all[noName].Username != "" {
			t.Errorf("Username = %q, want empty", all[noName].Username)
		}
	})

	t.Run("ACS-97_LimitAndOffsetSelectAPage", func(t *testing.T) {
		all, err := as.ListAccounts(context.Background(), 100000, 0)
		if err != nil {
			t.Fatalf("ListAccounts() err = %v", err)
		}
		first := indexOf(all, "910000000000000601")
		page, err := as.ListAccounts(context.Background(), 2, first)
		if err != nil {
			t.Fatalf("ListAccounts() err = %v", err)
		}
		if len(page) != 2 || page[0].UserID != "910000000000000601" || page[1].UserID != "910000000000000602" {
			t.Errorf("page = %v, want accounts ...601 and ...602", page)
		}
		one, err := as.ListAccounts(context.Background(), 1, first+1)
		if err != nil || len(one) != 1 || one[0].UserID != "910000000000000602" {
			t.Errorf("ListAccounts(1, %d) = (%v, %v), want account ...602", first+1, one, err)
		}
		only, err := as.ListAccounts(context.Background(), 1, first)
		if err != nil || len(only) != 1 || only[0].UserID != "910000000000000601" {
			t.Errorf("ListAccounts(1, %d) = (%v, %v), want only account ...601", first, only, err)
		}
	})

	t.Run("ACS-98_AnOffsetPastTheEndIsAnEmptyPage", func(t *testing.T) {
		page, err := as.ListAccounts(context.Background(), 10, 1000000)
		if err != nil || len(page) != 0 {
			t.Errorf("ListAccounts(10, 1000000) = (%v, %v), want an empty page and no error", page, err)
		}
	})
}
