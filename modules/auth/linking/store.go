package accountlinking

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CREATE TABLE linked_accounts (
//   user_id UUID NOT NULL,
//   platform TEXT NOT NULL,
//   platform_username TEXT NOT NULL,
//   platform_id TEXT NOT NULL,
//   data JSONB NOT NULL,
//   data_updated_at timestamp with time zone default current_timestamp,
//   created_at timestamp with time zone default current_timestamp,
//   FOREIGN KEY (user_id) REFERENCES accounts(user_id),
//   CONSTRAINT linked_accounts_unique UNIQUE (user_id, platform)
// );

// LinkAccountStore - Account Link Store
type LinkAccountStore interface {
	AddLinkedAccountToDB(la *LinkedAccount) (*LinkedAccount, error)
	GetLinkedAccountByPlatformID(platform, platformID string) (*LinkedAccount, error)
	GetLinkedAccountByUserID(userID uuid.UUID, platform string) (*LinkedAccount, error)
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

// GetLinkedAccountByPlatformID gets a linked account by user ID and platform
func (s *store) GetLinkedAccountByPlatformID(platform, platformID string) (*LinkedAccount, error) {
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
func (s *store) GetLinkedAccountByUserID(userID uuid.UUID, platform string) (*LinkedAccount, error) {
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
