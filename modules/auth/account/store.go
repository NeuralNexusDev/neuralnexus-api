package account

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound is returned when an account, linked account or OAuth token is missing, including from the link store.
	ErrNotFound = errors.New("not found")
	// ErrInvalidRoleID when an account role ID is not a canonical positive integer.
	ErrInvalidRoleID = errors.New("invalid role id")
	// ErrUnknownRoleID when an account role ID has no role.
	ErrUnknownRoleID = errors.New("unknown role id")
)

// store - the Postgres implementation of Store
type store struct {
	db *pgxpool.Pool
}

// NewStore - Create a store for accounts
func NewStore(db *pgxpool.Pool) Store {
	return &store{db: db}
}

type settingsStore struct {
	db *pgxpool.Pool
}

// NewSettingsStore - Create a store for account settings
func NewSettingsStore(db *pgxpool.Pool) SettingsStore {
	return &settingsStore{db: db}
}

// Store - The account store interface
type Store interface {
	AddAccountToDB(ctx context.Context, account *Account) error
	GetAccountByID(ctx context.Context, userID string) (*Account, error)
	GetAccountByUsername(ctx context.Context, username string) (*Account, error)
	GetAccountByEmail(ctx context.Context, email string) (*Account, error)
	ListAccounts(ctx context.Context, limit, offset int) ([]*Account, error)
	UpdateAccountInDB(ctx context.Context, account *Account) error
	DeleteAccountFromDB(ctx context.Context, userID string) error
}

var ErrEmailAlreadyExists = errors.New("account with this email already exists")

var ErrUsernameAlreadyExists = errors.New("account with this username already exists")

// translateAccountConstraintErr maps a Postgres unique-violation on the
// accounts table to the matching sentinel, so callers never see a raw,
// driver-specific error for a condition they're expected to recover from.
func translateAccountConstraintErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "accounts_email_key":
			return ErrEmailAlreadyExists
		case "accounts_username_key":
			return ErrUsernameAlreadyExists
		}
	}
	return err
}

// AddAccountToDB creates an account in the database
func (s *store) AddAccountToDB(ctx context.Context, account *Account) error {
	return translateAccountConstraintErr(s.writeAccount(ctx, account,
		"INSERT INTO accounts (user_id, username, email, hashed_secret, salt, role_ids) VALUES ($1, NULLIF($2, ''), $3, $4, $5, COALESCE($6::text[], '{}')::bigint[])"))
}

// GetAccountByID gets an account by ID
func (s *store) GetAccountByID(ctx context.Context, userID string) (*Account, error) {
	rows, err := s.db.Query(ctx, "SELECT user_id, COALESCE(username, '') AS username, email, hashed_secret, salt, role_ids::text[] AS role_ids, updated_at FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return account, nil
}

// GetAccountByUsername gets an account by username
func (s *store) GetAccountByUsername(ctx context.Context, username string) (*Account, error) {
	rows, err := s.db.Query(ctx, "SELECT user_id, COALESCE(username, '') AS username, email, hashed_secret, salt, role_ids::text[] AS role_ids, updated_at FROM accounts WHERE username = $1", username)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return account, nil
}

// GetAccountByEmail gets an account by email
func (s *store) GetAccountByEmail(ctx context.Context, email string) (*Account, error) {
	rows, err := s.db.Query(ctx, "SELECT user_id, COALESCE(username, '') AS username, email, hashed_secret, salt, role_ids::text[] AS role_ids, updated_at FROM accounts WHERE email = $1", email)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return account, nil
}

// ListAccounts gets a page of accounts ordered by ID
func (s *store) ListAccounts(ctx context.Context, limit, offset int) ([]*Account, error) {
	rows, err := s.db.Query(ctx, "SELECT user_id, COALESCE(username, '') AS username, email, hashed_secret, salt, role_ids::text[] AS role_ids, updated_at FROM accounts ORDER BY user_id LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[Account])
}

// UpdateAccountInDB updates an account in the database
func (s *store) UpdateAccountInDB(ctx context.Context, account *Account) error {
	return translateAccountConstraintErr(s.writeAccount(ctx, account,
		"UPDATE accounts SET username = NULLIF($2, ''), email = $3, hashed_secret = $4, salt = $5, role_ids = COALESCE($6::text[], '{}')::bigint[] WHERE user_id = $1"))
}

// DeleteAccountFromDB deletes an account from the database
func (s *store) DeleteAccountFromDB(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return err
	}
	return nil
}

// SettingsStore - The account settings store interface
type SettingsStore interface {
	GetAccountSettings(ctx context.Context, userID string) (*AccountSettings, error)
	SetPasswordAuthEnabled(ctx context.Context, userID string, enabled bool) error
}

var ErrNoPasswordSet = errors.New("this account has no password set")

var ErrWouldLockAccount = errors.New("this is the account's last usable login method; set a password or link another platform first")

// GetAccountSettings returns userID's settings, the defaults if the account
// has no account_settings row yet, or ErrNotFound if the account does not exist.
func (s *settingsStore) GetAccountSettings(ctx context.Context, userID string) (*AccountSettings, error) {
	rows, err := s.db.Query(ctx, "SELECT * FROM account_settings WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	settings, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[AccountSettings])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if err := s.db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM accounts WHERE user_id = $1)", userID).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				return nil, ErrNotFound
			}
			return DefaultAccountSettings(userID), nil
		}
		return nil, err
	}
	return settings, nil
}

// SetPasswordAuthEnabled toggles whether userID's password can be used to
// log in. Disabling shares DeleteLinkedAccount/SetLinkedAccountLoginEnabled's
// guard: it's refused if the account has no other verified, login-enabled
// linked platform. Enabling is refused if the account has no password to
// enable in the first place.
func (s *settingsStore) SetPasswordAuthEnabled(ctx context.Context, userID string, enabled bool) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(27745, hashtext($1))", userID); err != nil {
		return err
	}

	var one int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM accounts WHERE user_id = $1 FOR KEY SHARE", userID).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	var tag pgconn.CommandTag
	if enabled {
		tag, err = tx.Exec(ctx, `INSERT INTO account_settings (user_id, password_auth)
			SELECT $1, true WHERE EXISTS (SELECT 1 FROM accounts WHERE user_id = $1 AND hashed_secret IS NOT NULL)
			ON CONFLICT (user_id) DO UPDATE SET password_auth = true, updated_at = current_timestamp`, userID)
	} else {
		tag, err = tx.Exec(ctx, `INSERT INTO account_settings (user_id, password_auth)
			SELECT $1, false WHERE EXISTS (SELECT 1 FROM linked_accounts WHERE user_id = $1 AND verified = true AND login_enabled = true)
			ON CONFLICT (user_id) DO UPDATE SET password_auth = false, updated_at = current_timestamp`, userID)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if tag.RowsAffected() > 0 {
		return nil
	}
	if enabled {
		return ErrNoPasswordSet
	}
	return ErrWouldLockAccount
}

// writeAccount share-locks the account's roles so rbac.DeleteRole, which takes FOR UPDATE
// and then checks accounts.role_ids, cannot remove one before the write commits.
func (s *store) writeAccount(ctx context.Context, account *Account, query string) error {
	roleIDs := make([]string, 0, len(account.Roles))
	seen := make(map[string]struct{}, len(account.Roles))
	for _, id := range account.Roles {
		if n, err := strconv.ParseInt(id, 10, 64); err != nil || n < 1 || strconv.FormatInt(n, 10) != id {
			return ErrInvalidRoleID
		}
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			roleIDs = append(roleIDs, id)
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if len(roleIDs) > 0 {
		var found int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM (SELECT id FROM roles WHERE id = ANY($1::text[]::bigint[]) FOR SHARE) locked", roleIDs).Scan(&found); err != nil {
			return err
		}
		if found != len(roleIDs) {
			return ErrUnknownRoleID
		}
	}
	if _, err := tx.Exec(ctx, query, account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, roleIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
