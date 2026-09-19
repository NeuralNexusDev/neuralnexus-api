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
