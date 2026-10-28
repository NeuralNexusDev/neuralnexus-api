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

// Store - Account Link Store
type Store interface {
	AddLinkedAccountToDB(ctx context.Context, la *LinkedAccount) error
	UpdateLinkedAccount(ctx context.Context, la *LinkedAccount) error
	GetLinkedAccountByPlatformID(ctx context.Context, platform Platform, platformID string) (*LinkedAccount, error)
	GetLinkedAccountByPlatformName(ctx context.Context, platform Platform, platformName string) (*LinkedAccount, error)
	GetLinkedAccountByUserID(ctx context.Context, userID string, platform Platform) (*LinkedAccount, error)
	GetLinkedAccountsByUserID(ctx context.Context, userID string) ([]*LinkedAccount, error)
	DeleteLinkedAccount(ctx context.Context, userID string, platform Platform) error
	SetLinkedAccountLoginEnabled(ctx context.Context, userID string, platform Platform, enabled bool) error
}

var ErrAlreadyLinked = errors.New("platform account already linked")

var ErrUserAlreadyLinkedToPlatform = errors.New("user already has a linked account on this platform")

var ErrDuplicateLinkedAccount = errors.New("multiple linked accounts found for platform ID")

var ErrLinkedAccountUnverified = errors.New("this linked account is unverified and can't be enabled for login")

// AddLinkedAccountToDB adds a linked account to the database
func (s *store) AddLinkedAccountToDB(ctx context.Context, la *LinkedAccount) error {
	_, err := s.db.Exec(ctx, "INSERT INTO linked_accounts (user_id, platform, platform_username, platform_id, data, verified, login_enabled) VALUES ($1, $2, $3, $4, $5, $6, $7)", la.UserID, la.Platform, la.PlatformUsername, la.PlatformID, la.Data, la.Verified, la.LoginEnabled)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "linked_accounts_platform_unique" {
			return ErrAlreadyLinked
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "linked_accounts_unique" {
			return ErrUserAlreadyLinkedToPlatform
		}
		return err
	}
	return nil
}

// UpdateLinkedAccount updates a linked account in the database
func (s *store) UpdateLinkedAccount(ctx context.Context, la *LinkedAccount) error {
	_, err := s.db.Exec(ctx, "UPDATE linked_accounts SET platform_username = $1, platform_id = $2, data = $3, updated_at = current_timestamp WHERE user_id = $4 AND platform = $5", la.PlatformUsername, la.PlatformID, la.Data, la.UserID, la.Platform)
	if err != nil {
		return err
	}
	return nil
}

// GetLinkedAccountByPlatformID gets a linked account by user ID and platform
func (s *store) GetLinkedAccountByPlatformID(ctx context.Context, platform Platform, platformID string) (*LinkedAccount, error) {
	rows, err := s.db.Query(ctx, "SELECT * FROM linked_accounts WHERE platform = $1 AND platform_id = $2", platform, platformID)
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
func (s *store) GetLinkedAccountByPlatformName(ctx context.Context, platform Platform, platformName string) (*LinkedAccount, error) {
	rows, err := s.db.Query(ctx, "SELECT * FROM linked_accounts WHERE platform = $1 AND platform_username = $2", platform, platformName)
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
func (s *store) GetLinkedAccountByUserID(ctx context.Context, userID string, platform Platform) (*LinkedAccount, error) {
	rows, err := s.db.Query(ctx, "SELECT * FROM linked_accounts WHERE user_id = $1 AND platform = $2", userID, platform)
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
func (s *store) GetLinkedAccountsByUserID(ctx context.Context, userID string) ([]*LinkedAccount, error) {
	rows, err := s.db.Query(ctx, "SELECT * FROM linked_accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
}

// DeleteLinkedAccount unlinks a platform from an account. It refuses if
// doing so would leave the account with no password and no other verified,
// login-enabled linked platform. If RowsAffected == 0, a follow-up lookup
// tells "no such link" apart from "the guard blocked it".
func (s *store) DeleteLinkedAccount(ctx context.Context, userID string, platform Platform) error {
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
	if _, err := s.GetLinkedAccountByUserID(ctx, userID, platform); err != nil {
		return err
	}
	return account.ErrWouldLockAccount
}

// SetLinkedAccountLoginEnabled toggles login eligibility for a linked
// platform. Disabling shares DeleteLinkedAccount's guard; enabling requires
// the row to be Verified.
func (s *store) SetLinkedAccountLoginEnabled(ctx context.Context, userID string, platform Platform, enabled bool) error {
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
	if _, err := s.GetLinkedAccountByUserID(ctx, userID, platform); err != nil {
		return err
	}
	if enabled {
		return ErrLinkedAccountUnverified
	}
	return account.ErrWouldLockAccount
}
