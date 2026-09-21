package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func strPtr(s string) *string { return &s }

func setupAccountStore(t *testing.T) AccountStore {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run store tests")
	}

	db, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	_, err = db.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS accounts (
			user_id BIGINT PRIMARY KEY NOT NULL,
			username TEXT UNIQUE,
			email TEXT UNIQUE,
			hashed_secret BYTEA,
			salt BYTEA,
			roles TEXT[],
			updated_at timestamp with time zone default current_timestamp
		)
	`)
	if err != nil {
		t.Fatalf("failed to create accounts table: %v", err)
	}

	t.Cleanup(func() {
		db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id BETWEEN 900000000000000000 AND 900000000000009999")
		db.Close()
	})

	return NewStore(db, nil).Account()
}

// TestStoreAddAccountToDBDuplicateEmailTranslatesToSentinel verifies the
// exact scenario resolveOrCreateAccountForPlatformUser's race-recovery
// depends on: a real Postgres unique-violation on accounts.email comes back
// as auth.ErrEmailAlreadyExists, not a raw, unmatched pgconn.PgError. This
// pins down the "accounts_email_key" constraint-name assumption in
// AddAccountToDB against the actual auto-generated Postgres name for
// `email TEXT UNIQUE` with no explicit constraint name.
func TestStoreAddAccountToDBDuplicateEmailTranslatesToSentinel(t *testing.T) {
	as := setupAccountStore(t)

	a1 := &Account{UserID: "900000000000000001", Username: "storetest1", Email: strPtr("storetest-shared@example.com")}
	if err := as.AddAccountToDB(a1); err != nil {
		t.Fatalf("first AddAccountToDB returned error: %v", err)
	}

	a2 := &Account{UserID: "900000000000000002", Username: "storetest2", Email: strPtr("storetest-shared@example.com")}
	err := as.AddAccountToDB(a2)
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists for a duplicate email insert, got: %v", err)
	}
}

// TestStoreGetAccountByIDNotFoundTranslatesToSentinel verifies GetAccountByID
// translates a real "no rows" result (pgx.ErrNoRows) into auth.ErrNotFound,
// same as GetAccountByEmail/GetLinkedAccountByPlatformID already did.
func TestStoreGetAccountByIDNotFoundTranslatesToSentinel(t *testing.T) {
	as := setupAccountStore(t)

	_, err := as.GetAccountByID("900000000000009998")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for a missing account ID, got: %v", err)
	}
}

// TestStoreGetAccountByUsernameNotFoundTranslatesToSentinel is the same
// check as above for GetAccountByUsername.
func TestStoreGetAccountByUsernameNotFoundTranslatesToSentinel(t *testing.T) {
	as := setupAccountStore(t)

	_, err := as.GetAccountByUsername("no-such-user-ever")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for a missing username, got: %v", err)
	}
}

// TestStoreAddAccountToDBNilEmailsDoNotCollide is the end-to-end version of
// the NewIDOnlyAccount unit test: two accounts with a nil Email (e.g. two
// Minecraft-linked accounts, which never get a real email) must both
// succeed, since Postgres never treats two NULLs as colliding under
// accounts.email's UNIQUE constraint.
func TestStoreAddAccountToDBNilEmailsDoNotCollide(t *testing.T) {
	as := setupAccountStore(t)

	a1, err := NewIDOnlyAccount()
	if err != nil {
		t.Fatalf("NewIDOnlyAccount returned error: %v", err)
	}
	a1.UserID = "900000000000000003"
	a1.Username = "storetest-noemail1"
	if err := as.AddAccountToDB(a1); err != nil {
		t.Fatalf("first no-email AddAccountToDB returned error: %v", err)
	}

	a2, err := NewIDOnlyAccount()
	if err != nil {
		t.Fatalf("NewIDOnlyAccount returned error: %v", err)
	}
	a2.UserID = "900000000000000004"
	a2.Username = "storetest-noemail2"
	if err := as.AddAccountToDB(a2); err != nil {
		t.Fatalf("second no-email AddAccountToDB returned error: %v", err)
	}

	got, err := as.GetAccountByID(a1.UserID)
	if err != nil {
		t.Fatalf("GetAccountByID(a1) returned error: %v", err)
	}
	if got.Email != nil {
		t.Errorf("expected a1's email to round-trip as nil, got %q", *got.Email)
	}
}

// TestStoreAddAccountToDBEmptyUsernamesDoNotCollide is the real-Postgres
// regression test for a bug in NewIDOnlyAccount: unlike
// TestStoreAddAccountToDBEmptyEmailPlaceholdersDoNotCollide above, this
// does NOT override Username after calling NewIDOnlyAccount - it exercises
// exactly what UpdateUserFromPlatform (the admin
// PUT /api/v1/users/{platform}/{platform_id} endpoint) actually does. Every
// account NewIDOnlyAccount produces has Username == "" (it's a placeholder
// with no username yet), and accounts.username is UNIQUE, so once
// AddAccountToDB stored that "" as a literal empty string, the very first
// such account permanently "used up" the empty username: every subsequent
// platform identity with no linked account yet (any platform, any
// platform ID) would fail on accounts_username_key and this admin endpoint
// would be permanently broken from then on. AddAccountToDB now stores an
// empty username as SQL NULL (via NULLIF), and Postgres allows unlimited
// NULLs under a UNIQUE constraint, so both inserts below must succeed.
func TestStoreAddAccountToDBEmptyUsernamesDoNotCollide(t *testing.T) {
	as := setupAccountStore(t)

	a1, err := NewIDOnlyAccount()
	if err != nil {
		t.Fatalf("NewIDOnlyAccount returned error: %v", err)
	}
	a1.UserID = "900000000000000005"
	if err := as.AddAccountToDB(a1); err != nil {
		t.Fatalf("first empty-username AddAccountToDB returned error: %v", err)
	}

	a2, err := NewIDOnlyAccount()
	if err != nil {
		t.Fatalf("NewIDOnlyAccount returned error: %v", err)
	}
	a2.UserID = "900000000000000006"
	if err := as.AddAccountToDB(a2); err != nil {
		t.Fatalf("second empty-username AddAccountToDB returned error (this is the bug: an empty username should never collide): %v", err)
	}

	// Confirm both accounts round-trip back with Username == "" rather than
	// GetAccountByID choking on a NULL username column (COALESCE handles
	// this in the SELECT).
	got1, err := as.GetAccountByID(a1.UserID)
	if err != nil {
		t.Fatalf("GetAccountByID(a1) returned error: %v", err)
	}
	if got1.Username != "" {
		t.Errorf("expected a1's username to round-trip as \"\", got %q", got1.Username)
	}
	got2, err := as.GetAccountByID(a2.UserID)
	if err != nil {
		t.Fatalf("GetAccountByID(a2) returned error: %v", err)
	}
	if got2.Username != "" {
		t.Errorf("expected a2's username to round-trip as \"\", got %q", got2.Username)
	}
}

// TestStoreAddAccountToDBDuplicateUsernameTranslatesToSentinel verifies
// that a genuine collision between two real, non-empty usernames (as
// opposed to two empty ones, which the fix above makes non-colliding) is
// still rejected, and is translated to auth.ErrUsernameAlreadyExists rather
// than leaking a raw pgconn.PgError, mirroring how the email case is
// already translated to ErrEmailAlreadyExists.
func TestStoreAddAccountToDBDuplicateUsernameTranslatesToSentinel(t *testing.T) {
	as := setupAccountStore(t)

	a1 := &Account{UserID: "900000000000000007", Username: "storetest-shared-username", Email: strPtr("storetest-username1@example.com")}
	if err := as.AddAccountToDB(a1); err != nil {
		t.Fatalf("first AddAccountToDB returned error: %v", err)
	}

	a2 := &Account{UserID: "900000000000000008", Username: "storetest-shared-username", Email: strPtr("storetest-username2@example.com")}
	err := as.AddAccountToDB(a2)
	if !errors.Is(err, ErrUsernameAlreadyExists) {
		t.Errorf("expected ErrUsernameAlreadyExists for a duplicate username insert, got: %v", err)
	}
}

// setupLinkAccountStore returns all three stores backed by the same live
// Postgres connection, with accounts, linked_accounts, and account_settings
// created fresh - DeleteLinkedAccount/SetLinkedAccountLoginEnabled/
// SetPasswordAuthEnabled's guard queries join across all three tables, so
// testing them needs all three to exist together.
func setupLinkAccountStore(t *testing.T) (AccountStore, LinkAccountStore, AccountSettingsStore) {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run store tests")
	}

	db, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	_, err = db.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS accounts (
			user_id BIGINT PRIMARY KEY NOT NULL,
			username TEXT UNIQUE,
			email TEXT UNIQUE,
			hashed_secret BYTEA,
			salt BYTEA,
			roles TEXT[],
			updated_at timestamp with time zone default current_timestamp
		)
	`)
	if err != nil {
		t.Fatalf("failed to create accounts table: %v", err)
	}
	_, err = db.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS linked_accounts (
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
		)
	`)
	if err != nil {
		t.Fatalf("failed to create linked_accounts table: %v", err)
	}
	_, err = db.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS account_settings (
			user_id BIGINT PRIMARY KEY NOT NULL REFERENCES accounts(user_id),
			password_auth BOOLEAN NOT NULL DEFAULT true,
			updated_at timestamp with time zone default current_timestamp
		)
	`)
	if err != nil {
		t.Fatalf("failed to create account_settings table: %v", err)
	}

	t.Cleanup(func() {
		db.Exec(context.Background(), "DELETE FROM account_settings WHERE user_id BETWEEN 900000000000000000 AND 900000000000009999")
		db.Exec(context.Background(), "DELETE FROM linked_accounts WHERE user_id BETWEEN 900000000000000000 AND 900000000000009999")
		db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id BETWEEN 900000000000000000 AND 900000000000009999")
		db.Close()
	})

	s := NewStore(db, nil)
	return s.Account(), s.LinkAccount(), s.AccountSettings()
}

// seedTestAccount inserts a bare account (no password) for the guard tests
// to link platforms onto.
func seedTestAccount(t *testing.T, as AccountStore, userID string) {
	t.Helper()
	if err := as.AddAccountToDB(&Account{UserID: userID, Username: "storetest-" + userID}); err != nil {
		t.Fatalf("failed to seed account %s: %v", userID, err)
	}
}

// seedTestLink inserts a linked_accounts row directly (not through
// NewLinkedAccount) so tests can control Verified/LoginEnabled explicitly,
// including combinations NewLinkedAccount itself can never produce.
func seedTestLink(t *testing.T, las LinkAccountStore, userID string, platform Platform, platformID string, verified, loginEnabled bool) {
	t.Helper()
	if err := las.AddLinkedAccountToDB(&LinkedAccount{
		UserID:           userID,
		Platform:         platform,
		PlatformUsername: "storetest",
		PlatformID:       platformID,
		Data:             map[string]string{},
		Verified:         verified,
		LoginEnabled:     loginEnabled,
	}); err != nil {
		t.Fatalf("failed to seed %s link for %s: %v", platform, userID, err)
	}
}

// TestStoreDeleteLinkedAccountBlockedAsLastLoginMethod is the real-Postgres
// proof that the guard in hasOtherUsableLoginMethod actually works: a
// passwordless account with exactly one verified, login-enabled link must
// not be able to delete it - the row must still exist afterward.
func TestStoreDeleteLinkedAccountBlockedAsLastLoginMethod(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000100")
	seedTestLink(t, las, "900000000000000100", PlatformDiscord, "storetest-discord-100", true, true)

	err := las.DeleteLinkedAccount("900000000000000100", PlatformDiscord)
	if !errors.Is(err, ErrWouldLockAccount) {
		t.Fatalf("expected ErrWouldLockAccount, got: %v", err)
	}
	if _, err := las.GetLinkedAccountByUserID("900000000000000100", PlatformDiscord); err != nil {
		t.Errorf("expected the blocked link to still exist, got: %v", err)
	}
}

// TestStoreDeleteLinkedAccountAllowedWithPassword verifies the other half
// of the guard: an account WITH a password can delete its only linked
// platform, since the password remains a usable login method.
func TestStoreDeleteLinkedAccountAllowedWithPassword(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	a := &Account{UserID: "900000000000000101", Username: "storetest-101"}
	if err := a.HashPassword("storetest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account: %v", err)
	}
	seedTestLink(t, las, "900000000000000101", PlatformDiscord, "storetest-discord-101", true, true)

	if err := las.DeleteLinkedAccount("900000000000000101", PlatformDiscord); err != nil {
		t.Fatalf("expected the delete to succeed for a passworded account, got: %v", err)
	}
	if _, err := las.GetLinkedAccountByUserID("900000000000000101", PlatformDiscord); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected the link to be gone, got: %v", err)
	}
}

// TestStoreDeleteLinkedAccountAllowedWithAnotherEnabledLink verifies a
// passwordless account CAN unlink one platform as long as another
// verified, login-enabled one remains - and that the other one is left
// completely untouched.
func TestStoreDeleteLinkedAccountAllowedWithAnotherEnabledLink(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000102")
	seedTestLink(t, las, "900000000000000102", PlatformDiscord, "storetest-discord-102", true, true)
	seedTestLink(t, las, "900000000000000102", PlatformTwitch, "storetest-twitch-102", true, true)

	if err := las.DeleteLinkedAccount("900000000000000102", PlatformDiscord); err != nil {
		t.Fatalf("expected the delete to succeed, got: %v", err)
	}
	if _, err := las.GetLinkedAccountByUserID("900000000000000102", PlatformTwitch); err != nil {
		t.Errorf("expected the Twitch link to remain untouched, got: %v", err)
	}
}

// TestStoreDeleteLinkedAccountNotFound verifies deleting a platform that
// was never linked returns ErrNotFound, not ErrWouldLockAccount - the two
// are ambiguous from RowsAffected == 0 alone and must be told apart.
func TestStoreDeleteLinkedAccountNotFound(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000103")

	err := las.DeleteLinkedAccount("900000000000000103", PlatformDiscord)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for a platform that was never linked, got: %v", err)
	}
}

// TestStoreDeleteLinkedAccountIgnoresDisabledOtherLink verifies the guard
// counts only OTHER links that are themselves verified and login-enabled -
// a second linked platform that's already disabled for login doesn't count
// as a usable fallback, so deleting the last enabled one must still block.
func TestStoreDeleteLinkedAccountIgnoresDisabledOtherLink(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000104")
	seedTestLink(t, las, "900000000000000104", PlatformDiscord, "storetest-discord-104", true, true)
	seedTestLink(t, las, "900000000000000104", PlatformTwitch, "storetest-twitch-104", true, false)

	err := las.DeleteLinkedAccount("900000000000000104", PlatformDiscord)
	if !errors.Is(err, ErrWouldLockAccount) {
		t.Fatalf("expected ErrWouldLockAccount (Twitch is disabled, so it doesn't count), got: %v", err)
	}
}

// TestStoreSetLinkedAccountLoginEnabledBlockedAsLastLoginMethod mirrors the
// delete guard test for the disable-login toggle.
func TestStoreSetLinkedAccountLoginEnabledBlockedAsLastLoginMethod(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000105")
	seedTestLink(t, las, "900000000000000105", PlatformDiscord, "storetest-discord-105", true, true)

	err := las.SetLinkedAccountLoginEnabled("900000000000000105", PlatformDiscord, false)
	if !errors.Is(err, ErrWouldLockAccount) {
		t.Fatalf("expected ErrWouldLockAccount, got: %v", err)
	}
	la, err := las.GetLinkedAccountByUserID("900000000000000105", PlatformDiscord)
	if err != nil {
		t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
	}
	if !la.LoginEnabled {
		t.Error("expected LoginEnabled to remain true after a blocked disable")
	}
}

// TestStoreSetLinkedAccountLoginEnabledAllowedWithAnotherEnabledLink mirrors
// the delete guard's "another usable method remains" success case.
func TestStoreSetLinkedAccountLoginEnabledAllowedWithAnotherEnabledLink(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000106")
	seedTestLink(t, las, "900000000000000106", PlatformDiscord, "storetest-discord-106", true, true)
	seedTestLink(t, las, "900000000000000106", PlatformTwitch, "storetest-twitch-106", true, true)

	if err := las.SetLinkedAccountLoginEnabled("900000000000000106", PlatformDiscord, false); err != nil {
		t.Fatalf("expected the disable to succeed, got: %v", err)
	}
	la, err := las.GetLinkedAccountByUserID("900000000000000106", PlatformDiscord)
	if err != nil {
		t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
	}
	if la.LoginEnabled {
		t.Error("expected LoginEnabled to be false")
	}
}

// TestStoreSetLinkedAccountLoginEnabledCannotEnableUnverified verifies the
// second half of the toggle's guard: even though nothing in this package
// can construct an unverified row today, the store itself refuses to ever
// make one login-eligible, in case a row ever reaches this table some other
// way (e.g. a future soft-link feature, or a manual data fix).
func TestStoreSetLinkedAccountLoginEnabledCannotEnableUnverified(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000107")
	seedTestLink(t, las, "900000000000000107", PlatformDiscord, "storetest-discord-107", false, false)

	err := las.SetLinkedAccountLoginEnabled("900000000000000107", PlatformDiscord, true)
	if !errors.Is(err, ErrLinkedAccountUnverified) {
		t.Fatalf("expected ErrLinkedAccountUnverified, got: %v", err)
	}
}

// TestStoreGetLinkedAccountsByUserID verifies it returns every platform
// linked to a user, unlike GetLinkedAccountByUserID which takes one.
func TestStoreGetLinkedAccountsByUserID(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000108")
	seedTestLink(t, las, "900000000000000108", PlatformDiscord, "storetest-discord-108", true, true)
	seedTestLink(t, las, "900000000000000108", PlatformTwitch, "storetest-twitch-108", true, true)

	links, err := las.GetLinkedAccountsByUserID("900000000000000108")
	if err != nil {
		t.Fatalf("GetLinkedAccountsByUserID returned error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 linked accounts, got %d: %+v", len(links), links)
	}
}

// TestStoreDeleteLinkedAccountConcurrentDifferentPlatformsNeverBothSucceed
// is the regression test for the review pipeline's critical finding:
// hasOtherUsableLoginMethod alone is not atomic across two concurrent
// requests that each target a DIFFERENT platform on the same account. It's
// an EXISTS check with no lock on the row it reads, and each statement
// only locks the row it writes - so under READ COMMITTED, two deletes for
// two different platforms each see the other's row as still present and
// eligible, and both proceed. This is a write-skew anomaly (the two writes
// touch disjoint rows, so Postgres never needs to block them against each
// other), and it reproduced in ~299/300 trials before
// lockLinkedAccountsForUser was added. Run many trials, not one, since the
// race is real but timing-dependent.
func TestStoreDeleteLinkedAccountConcurrentDifferentPlatformsNeverBothSucceed(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)

	const trials = 50
	for i := 0; i < trials; i++ {
		userID := fmt.Sprintf("90000000000000%04d", 1000+i)
		seedTestAccount(t, as, userID)
		seedTestLink(t, las, userID, PlatformDiscord, "storetest-discord-"+userID, true, true)
		seedTestLink(t, las, userID, PlatformTwitch, "storetest-twitch-"+userID, true, true)

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
				// expected for the loser
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
}

// TestStoreSetLinkedAccountLoginEnabledConcurrentDifferentPlatformsNeverBothSucceed
// mirrors the delete test above for the disable-login toggle, which shares
// the same guard and the same fix.
func TestStoreSetLinkedAccountLoginEnabledConcurrentDifferentPlatformsNeverBothSucceed(t *testing.T) {
	as, las, _ := setupLinkAccountStore(t)

	const trials = 50
	for i := 0; i < trials; i++ {
		userID := fmt.Sprintf("90000000000000%04d", 2000+i)
		seedTestAccount(t, as, userID)
		seedTestLink(t, las, userID, PlatformDiscord, "storetest-discord-"+userID, true, true)
		seedTestLink(t, las, userID, PlatformTwitch, "storetest-twitch-"+userID, true, true)

		var wg sync.WaitGroup
		results := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); results[0] = las.SetLinkedAccountLoginEnabled(userID, PlatformDiscord, false) }()
		go func() { defer wg.Done(); results[1] = las.SetLinkedAccountLoginEnabled(userID, PlatformTwitch, false) }()
		wg.Wait()

		successes := 0
		for _, err := range results {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrWouldLockAccount):
				// expected for the loser
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
}

// -------------- AccountSettings / password auth toggle --------------

// TestStoreGetAccountSettingsDefaultsWhenNoRow verifies the lazy-row design:
// an account with no account_settings row yet must behave as if password
// auth were enabled, not error or read as disabled.
func TestStoreGetAccountSettingsDefaultsWhenNoRow(t *testing.T) {
	as, _, ass := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000115")

	settings, err := ass.GetAccountSettings("900000000000000115")
	if err != nil {
		t.Fatalf("GetAccountSettings returned error: %v", err)
	}
	if !settings.PasswordAuthEnabled {
		t.Error("expected password auth to default to enabled with no account_settings row")
	}
}

// TestStoreSetPasswordAuthEnabledDisableBlockedAsLastLoginMethod verifies
// SetPasswordAuthEnabled shares the same guard as unlinking/disabling a
// platform: an account with a password and no linked platform can't disable
// that password, since it would leave no usable login method at all.
func TestStoreSetPasswordAuthEnabledDisableBlockedAsLastLoginMethod(t *testing.T) {
	as, _, ass := setupLinkAccountStore(t)
	a := &Account{UserID: "900000000000000111", Username: "storetest-111"}
	if err := a.HashPassword("storetest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account: %v", err)
	}

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
}

// TestStoreSetPasswordAuthEnabledDisableAllowedWithLinkedAccount verifies the
// other half: disabling password auth succeeds once a verified,
// login-enabled linked platform exists as the fallback.
func TestStoreSetPasswordAuthEnabledDisableAllowedWithLinkedAccount(t *testing.T) {
	as, las, ass := setupLinkAccountStore(t)
	a := &Account{UserID: "900000000000000112", Username: "storetest-112"}
	if err := a.HashPassword("storetest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account: %v", err)
	}
	seedTestLink(t, las, a.UserID, PlatformDiscord, "storetest-discord-112", true, true)

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
}

// TestStoreSetPasswordAuthEnabledEnableBlockedWithNoPassword verifies
// enabling password auth is refused when the account has no hashed_secret
// to enable in the first place.
func TestStoreSetPasswordAuthEnabledEnableBlockedWithNoPassword(t *testing.T) {
	as, _, ass := setupLinkAccountStore(t)
	seedTestAccount(t, as, "900000000000000113")

	err := ass.SetPasswordAuthEnabled("900000000000000113", true)
	if !errors.Is(err, ErrNoPasswordSet) {
		t.Fatalf("expected ErrNoPasswordSet for an account with no password, got: %v", err)
	}
}

// TestStoreSetPasswordAuthEnabledEnableAllowedWithPassword round-trips a
// disable followed by a re-enable, both of which must succeed for an
// account that has both a password and a linked fallback.
func TestStoreSetPasswordAuthEnabledEnableAllowedWithPassword(t *testing.T) {
	as, las, ass := setupLinkAccountStore(t)
	a := &Account{UserID: "900000000000000114", Username: "storetest-114"}
	if err := a.HashPassword("storetest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account: %v", err)
	}
	seedTestLink(t, las, a.UserID, PlatformDiscord, "storetest-discord-114", true, true)

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
}

// TestStoreDeleteLinkedAccountBlockedWhenPasswordAuthDisabled verifies the
// updated DeleteLinkedAccount guard: a non-null hashed_secret alone is no
// longer enough once password_auth has been explicitly turned off -
// the account's only remaining linked platform must not be removable.
func TestStoreDeleteLinkedAccountBlockedWhenPasswordAuthDisabled(t *testing.T) {
	as, las, ass := setupLinkAccountStore(t)
	a := &Account{UserID: "900000000000000109", Username: "storetest-109"}
	if err := a.HashPassword("storetest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account: %v", err)
	}
	seedTestLink(t, las, a.UserID, PlatformDiscord, "storetest-discord-109", true, true)

	if err := ass.SetPasswordAuthEnabled(a.UserID, false); err != nil {
		t.Fatalf("expected disabling password auth to succeed with a linked account present, got: %v", err)
	}

	err := las.DeleteLinkedAccount(a.UserID, PlatformDiscord)
	if !errors.Is(err, ErrWouldLockAccount) {
		t.Fatalf("expected ErrWouldLockAccount once password auth is disabled and this is the only link, got: %v", err)
	}
}

// TestStoreSetLinkedAccountLoginEnabledBlockedWhenPasswordAuthDisabled mirrors
// the delete case above for the disable-login toggle.
func TestStoreSetLinkedAccountLoginEnabledBlockedWhenPasswordAuthDisabled(t *testing.T) {
	as, las, ass := setupLinkAccountStore(t)
	a := &Account{UserID: "900000000000000110", Username: "storetest-110"}
	if err := a.HashPassword("storetest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(a); err != nil {
		t.Fatalf("failed to seed passworded account: %v", err)
	}
	seedTestLink(t, las, a.UserID, PlatformDiscord, "storetest-discord-110", true, true)

	if err := ass.SetPasswordAuthEnabled(a.UserID, false); err != nil {
		t.Fatalf("expected disabling password auth to succeed, got: %v", err)
	}

	err := las.SetLinkedAccountLoginEnabled(a.UserID, PlatformDiscord, false)
	if !errors.Is(err, ErrWouldLockAccount) {
		t.Fatalf("expected ErrWouldLockAccount once password auth is disabled and this is the only enabled link, got: %v", err)
	}
}

// TestStoreSetPasswordAuthEnabledConcurrentWithDeleteLinkedAccountNeverBothSucceed
// is the cross-guard version of the write-skew regression tests above:
// SetPasswordAuthEnabled(false) and DeleteLinkedAccount race against the SAME
// underlying invariant from two different entry points. Both share the
// pg_advisory_xact_lock(27745, hashtext(userID)) lock, so exactly one must
// win - if both won, the account would end up with neither a working
// password nor a linked platform.
func TestStoreSetPasswordAuthEnabledConcurrentWithDeleteLinkedAccountNeverBothSucceed(t *testing.T) {
	as, las, ass := setupLinkAccountStore(t)

	const trials = 50
	for i := 0; i < trials; i++ {
		userID := fmt.Sprintf("90000000000000%04d", 3000+i)
		a := &Account{UserID: userID, Username: "storetest-" + userID}
		if err := a.HashPassword("storetest-password"); err != nil {
			t.Fatalf("trial %d: HashPassword returned error: %v", i, err)
		}
		if err := as.AddAccountToDB(a); err != nil {
			t.Fatalf("trial %d: failed to seed passworded account: %v", i, err)
		}
		seedTestLink(t, las, userID, PlatformDiscord, "storetest-discord-"+userID, true, true)

		var wg sync.WaitGroup
		results := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); results[0] = ass.SetPasswordAuthEnabled(userID, false) }()
		go func() { defer wg.Done(); results[1] = las.DeleteLinkedAccount(userID, PlatformDiscord) }()
		wg.Wait()

		successes := 0
		for _, err := range results {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrWouldLockAccount):
				// expected for the loser
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
}

// TestStoreSetPasswordAuthEnabledConcurrentWithSetLinkedAccountLoginEnabledNeverBothSucceed
// mirrors the DeleteLinkedAccount cross-guard test above for the OTHER
// disable entry point: SetPasswordAuthEnabled(false) and
// SetLinkedAccountLoginEnabled(platform, false) race to disable the two
// halves of the same account's last-usable-login-method pair. Both share
// the pg_advisory_xact_lock(27745, hashtext(userID)) lock, so exactly one
// must win - if both won, the account would end up with neither a working
// password nor an enabled linked platform.
func TestStoreSetPasswordAuthEnabledConcurrentWithSetLinkedAccountLoginEnabledNeverBothSucceed(t *testing.T) {
	as, las, ass := setupLinkAccountStore(t)

	const trials = 50
	for i := 0; i < trials; i++ {
		userID := fmt.Sprintf("90000000000000%04d", 4000+i)
		a := &Account{UserID: userID, Username: "storetest-" + userID}
		if err := a.HashPassword("storetest-password"); err != nil {
			t.Fatalf("trial %d: HashPassword returned error: %v", i, err)
		}
		if err := as.AddAccountToDB(a); err != nil {
			t.Fatalf("trial %d: failed to seed passworded account: %v", i, err)
		}
		seedTestLink(t, las, userID, PlatformDiscord, "storetest-discord-"+userID, true, true)

		var wg sync.WaitGroup
		results := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); results[0] = ass.SetPasswordAuthEnabled(userID, false) }()
		go func() {
			defer wg.Done()
			results[1] = las.SetLinkedAccountLoginEnabled(userID, PlatformDiscord, false)
		}()
		wg.Wait()

		successes := 0
		for _, err := range results {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrWouldLockAccount):
				// expected for the loser
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
		link, err := las.GetLinkedAccountByUserID(userID, PlatformDiscord)
		if err != nil {
			t.Fatalf("trial %d: GetLinkedAccountByUserID returned error: %v", i, err)
		}
		if !settings.PasswordAuthEnabled && !link.LoginEnabled {
			t.Fatalf("trial %d: account left with NO usable login method (password disabled and link login-disabled)", i)
		}
	}
}
