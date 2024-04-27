package accountlinking

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CREATE TRIGGER update_linked_accounts_modtime
// BEFORE UPDATE ON linked_accounts
// FOR EACH ROW
// EXECUTE PROCEDURE update_modified_column();

// CREATE TABLE linked_accounts (
//   user_id BIGINT NOT NULL,
//   platform TEXT NOT NULL,
//   platform_username TEXT NOT NULL,
//   platform_id TEXT NOT NULL,
//   data JSONB NOT NULL,
//   created_at timestamp with time zone default current_timestamp,
//   updated_at timestamp with time zone default current_timestamp,
//   FOREIGN KEY (user_id) REFERENCES accounts(user_id),
//   CONSTRAINT linked_accounts_unique UNIQUE (user_id, platform)
// );

// LinkAccountStore - Account Link Store
type LinkAccountStore interface {
	AddLinkedAccountToDB(la *LinkedAccount) (*LinkedAccount, error)
	UpdateLinkedAccount(la *LinkedAccount) (*LinkedAccount, error)
	GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error)
	GetLinkedAccountByUserID(userID string, platform string) (*LinkedAccount, error)
}

// store - Account Link Store PG implementation
type store struct {
	db *pgxpool.Pool
}

// NewStore - Create a new Account Link store
func NewStore(db *pgxpool.Pool) LinkAccountStore {
	return &store{db: db}
}

// AddLinkedAccountToDB adds a linked account to the database
func (s *store) AddLinkedAccountToDB(la *LinkedAccount) (*LinkedAccount, error) {
	_, err := s.db.Exec(context.Background(), "INSERT INTO linked_accounts (user_id, platform, platform_username, platform_id, data) VALUES ($1, $2, $3, $4, $5)", la.UserID, la.Platform, la.PlatformUsername, la.PlatformID, la.Data)
	if err != nil {
		return nil, err
	}
	return la, nil
}

// UpdateLinkedAccount updates a linked account in the database
func (s *store) UpdateLinkedAccount(la *LinkedAccount) (*LinkedAccount, error) {
	_, err := s.db.Exec(context.Background(), "UPDATE linked_accounts SET platform_username = $1, platform_id = $2, data = $3, updated_at = current_timestamp WHERE user_id = $4 AND platform = $5", la.PlatformUsername, la.PlatformID, la.Data, la.UserID, la.Platform)
	if err != nil {
		return nil, err
	}
	return la, nil
}

// GetLinkedAccountByPlatformID gets a linked account by user ID and platform
func (s *store) GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE platform = $1 AND platform_id = $2", platform, platformID)
	if err != nil {
		return nil, err
	}

	al, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		return nil, err
	}
	return al, nil
}

// GetLinkedAccountByUserID gets a linked account by user ID and platform
func (s *store) GetLinkedAccountByUserID(userID string, platform string) (*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE user_id = $1 AND platform = $2", userID, platform)
	if err != nil {
		return nil, err
	}

	al, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		return nil, err
	}
	return al, nil
}
