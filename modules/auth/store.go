package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CREATE TABLE accounts (
// 	user_id UUID PRIMARY KEY NOT NULL,
// 	username TEXT UNIQUE,
// 	email TEXT UNIQUE,
// 	hashed_secret BYTEA,
// 	salt BYTEA,
// 	roles TEXT[],
//  created_at timestamp with time zone default current_timestamp,
//  CONSTRAINT email_unique CHECK (email IS NOT NULL),
//  CONSTRAINT password_enforced CHECK (email IS NOT NULL OR hashed_secret IS NOT NULL)
// );

// AccountStore interface
type AccountStore interface {
	AddAccountToDB(account *Account) (*Account, error)
	GetAccountByID(userID uuid.UUID) (*Account, error)
	GetAccountByUsername(username string) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
	UpdateAccount(account *Account) (*Account, error)
	DeleteAccount(userID uuid.UUID) (*Account, error)
}

// acctStore - AccountStore implementation
type acctStore struct {
	db *pgxpool.Pool
}

// NewAccountStore - Create a new account store
func NewAccountStore(db *pgxpool.Pool) AccountStore {
	return &acctStore{
		db: db,
	}
}

// AddAccountToDB creates an account in the database
func (s *acctStore) AddAccountToDB(account *Account) (*Account, error) {
	_, err := s.db.Exec(context.Background(),
		"INSERT INTO accounts (user_id, username, email, hashed_secret, salt, roles) VALUES ($1, $2, $3, $4, $5, $6)",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return nil, err
	}
	return account, nil
}

// GetAccountByID gets an account by ID
func (s *acctStore) GetAccountByID(userID uuid.UUID) (*Account, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return nil, err
	}
	return account, nil
}

// GetAccountByUsername gets an account by username
func (s *acctStore) GetAccountByUsername(username string) (*Account, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM accounts WHERE username = $1", username)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return nil, err
	}
	return account, nil
}

// GetAccountByEmail gets an account by email
func (s *acctStore) GetAccountByEmail(email string) (*Account, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM accounts WHERE email = $1", email)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return nil, err
	}
	return account, nil
}

// UpdateAccount updates an account in the database
func (s *acctStore) UpdateAccount(account *Account) (*Account, error) {
	_, err := s.db.Exec(context.Background(),
		"UPDATE accounts SET username = $2, email = $3, hashed_secret = $4, salt = $5, roles = $6 WHERE user_id = $1",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return nil, err
	}
	return account, nil
}

// DeleteAccount deletes an account from the database
func (s *acctStore) DeleteAccount(userID uuid.UUID) (*Account, error) {
	_, err := s.db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	return &Account{UserID: userID}, nil
}
