package auth

import (
	"context"
	"errors"
	"os"
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

// setupLinkAccountStore returns both stores backed by the same live
// Postgres connection, with accounts and linked_accounts created fresh -
// DeleteLinkedAccount/SetLinkedAccountLoginEnabled's guard queries join
// across both tables, so testing them needs both to exist together.
func setupLinkAccountStore(t *testing.T) (AccountStore, LinkAccountStore) {
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

	t.Cleanup(func() {
		db.Exec(context.Background(), "DELETE FROM linked_accounts WHERE user_id BETWEEN 900000000000000000 AND 900000000000009999")
		db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id BETWEEN 900000000000000000 AND 900000000000009999")
		db.Close()
	})

	s := NewStore(db, nil)
	return s.Account(), s.LinkAccount()
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
	as, las := setupLinkAccountStore(t)
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
