package link

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
)

// store - Postgres-backed linked account store
type store struct {
	db *pgxpool.Pool
}

// NewStore - Create a new linked account store
func NewStore(db *pgxpool.Pool) Store {
	return &store{db: db}
}

//CREATE TRIGGER update_linked_accounts_modtime
//BEFORE UPDATE ON linked_accounts
//FOR EACH ROW
//EXECUTE PROCEDURE update_modified_column();

// CREATE TABLE linked_accounts (
//   user_id BIGINT NOT NULL,
//   platform TEXT NOT NULL,
//   platform_username TEXT NOT NULL,
//   platform_id TEXT NOT NULL,
//   data JSONB NOT NULL,
//   verified BOOLEAN NOT NULL DEFAULT false,
//   login_enabled BOOLEAN NOT NULL DEFAULT false,
//   created_at timestamp with time zone default current_timestamp,
//   updated_at timestamp with time zone default current_timestamp,
//   FOREIGN KEY (user_id) REFERENCES accounts(user_id),
//   CONSTRAINT linked_accounts_unique UNIQUE (user_id, platform),
//   CONSTRAINT linked_accounts_platform_unique UNIQUE (platform, platform_id)
// );

// Store - Account Link Store
type Store interface {
	AddLinkedAccountToDB(la *LinkedAccount) error
	UpdateLinkedAccount(la *LinkedAccount) error
	GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error)
	GetLinkedAccountByPlatformName(platform Platform, platformName string) (*LinkedAccount, error)
	GetLinkedAccountByUserID(userID string, platform Platform) (*LinkedAccount, error)
	GetLinkedAccountsByUserID(userID string) ([]*LinkedAccount, error)
	DeleteLinkedAccount(userID string, platform Platform) error
	SetLinkedAccountLoginEnabled(userID string, platform Platform, enabled bool) error
}

var ErrAlreadyLinked = errors.New("platform account already linked")

var ErrDuplicateLinkedAccount = errors.New("multiple linked accounts found for platform ID")

var ErrLinkedAccountUnverified = errors.New("this linked account is unverified and can't be enabled for login")

// AddLinkedAccountToDB adds a linked account to the database
func (s *store) AddLinkedAccountToDB(la *LinkedAccount) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO linked_accounts (user_id, platform, platform_username, platform_id, data, verified, login_enabled) VALUES ($1, $2, $3, $4, $5, $6, $7)", la.UserID, la.Platform, la.PlatformUsername, la.PlatformID, la.Data, la.Verified, la.LoginEnabled)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "linked_accounts_platform_unique" {
			return ErrAlreadyLinked
		}
		return err
	}
	return nil
}

// UpdateLinkedAccount updates a linked account in the database
func (s *store) UpdateLinkedAccount(la *LinkedAccount) error {
	_, err := s.db.Exec(context.Background(), "UPDATE linked_accounts SET platform_username = $1, platform_id = $2, data = $3, updated_at = current_timestamp WHERE user_id = $4 AND platform = $5", la.PlatformUsername, la.PlatformID, la.Data, la.UserID, la.Platform)
	if err != nil {
		return err
	}
	return nil
}

// GetLinkedAccountByPlatformID gets a linked account by user ID and platform
func (s *store) GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE platform = $1 AND platform_id = $2", platform, platformID)
	if err != nil {
		return nil, err
	}

	al, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, account.ErrNotFound
		}
		if errors.Is(err, pgx.ErrTooManyRows) {
			return nil, ErrDuplicateLinkedAccount
		}
		return nil, err
	}
	return al, nil
}

// GetLinkedAccountByPlatformName gets a linked account by name and platform
func (s *store) GetLinkedAccountByPlatformName(platform Platform, platformName string) (*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE platform = $1 AND platform_username = $2", platform, platformName)
	if err != nil {
		return nil, err
	}

	al, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, account.ErrNotFound
		}
		if errors.Is(err, pgx.ErrTooManyRows) {
			return nil, ErrDuplicateLinkedAccount
		}
		return nil, err
	}
	return al, nil
}

// GetLinkedAccountByUserID gets a linked account by user ID and platform
func (s *store) GetLinkedAccountByUserID(userID string, platform Platform) (*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE user_id = $1 AND platform = $2", userID, platform)
	if err != nil {
		return nil, err
	}

	al, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, account.ErrNotFound
		}
		if errors.Is(err, pgx.ErrTooManyRows) {
			return nil, ErrDuplicateLinkedAccount
		}
		return nil, err
	}
	return al, nil
}

// GetLinkedAccountsByUserID returns every platform linked to userID.
func (s *store) GetLinkedAccountsByUserID(userID string) ([]*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
}

// DeleteLinkedAccount unlinks a platform from an account. It refuses if
// doing so would leave the account with no password and no other verified,
// login-enabled linked platform. If RowsAffected == 0, a follow-up lookup
// tells "no such link" apart from "the guard blocked it".
func (s *store) DeleteLinkedAccount(userID string, platform Platform) error {
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(27745, hashtext($1))", userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM linked_accounts WHERE user_id = $1 AND platform = $2 AND (
		EXISTS (
			SELECT 1 FROM accounts WHERE user_id = $1 AND hashed_secret IS NOT NULL
			AND COALESCE((SELECT password_auth FROM account_settings WHERE user_id = $1), true)
		)
		OR EXISTS (SELECT 1 FROM linked_accounts WHERE user_id = $1 AND platform != $2 AND verified = true AND login_enabled = true)
	)`, userID, platform)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if tag.RowsAffected() > 0 {
		return nil
	}
	if _, err := s.GetLinkedAccountByUserID(userID, platform); err != nil {
		return err
	}
	return account.ErrWouldLockAccount
}

// SetLinkedAccountLoginEnabled toggles login eligibility for a linked
// platform. Disabling shares DeleteLinkedAccount's guard; enabling requires
// the row to be Verified.
func (s *store) SetLinkedAccountLoginEnabled(userID string, platform Platform, enabled bool) error {
	ctx := context.Background()
	var tag pgconn.CommandTag

	if enabled {
		var err error
		tag, err = s.db.Exec(ctx, "UPDATE linked_accounts SET login_enabled = true, updated_at = current_timestamp WHERE user_id = $1 AND platform = $2 AND verified = true", userID, platform)
		if err != nil {
			return err
		}
	} else {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(27745, hashtext($1))", userID); err != nil {
			return err
		}
		tag, err = tx.Exec(ctx, `UPDATE linked_accounts SET login_enabled = false, updated_at = current_timestamp WHERE user_id = $1 AND platform = $2 AND (
			EXISTS (
				SELECT 1 FROM accounts WHERE user_id = $1 AND hashed_secret IS NOT NULL
				AND COALESCE((SELECT password_auth FROM account_settings WHERE user_id = $1), true)
			)
			OR EXISTS (SELECT 1 FROM linked_accounts WHERE user_id = $1 AND platform != $2 AND verified = true AND login_enabled = true)
		)`, userID, platform)
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	if tag.RowsAffected() > 0 {
		return nil
	}
	if _, err := s.GetLinkedAccountByUserID(userID, platform); err != nil {
		return err
	}
	if enabled {
		return ErrLinkedAccountUnverified
	}
	return account.ErrWouldLockAccount
}
