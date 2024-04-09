package auth

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/argon2"
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

// -------------- Structs --------------

// Account struct
type Account struct {
	UserID       uuid.UUID `db:"user_id" validate:"required"`
	Username     string    `db:"username" validate:"required_without=Email"`
	Email        string    `db:"email" validate:"required_without=Username"`
	HashedSecret []byte    `db:"hashed_secret" validate:"required_without=Email"`
	Salt         []byte    `db:"salt"`
	Roles        []string  `db:"roles"`
	CreatedAt    time.Time `db:"created_at"`
}

// NewAccount creates a new account
func NewAccount(username, email, password string) (Account, error) {
	user := Account{
		UserID:   uuid.New(),
		Username: username,
		Email:    email,
	}
	err := user.HashPassword(password)
	if err != nil {
		return user, err
	}
	return user, nil
}

// NewPasswordLessAccount creates a new account without a password
func NewPasswordLessAccount(username, email string) Account {
	return Account{
		UserID:   uuid.New(),
		Username: username,
		Email:    email,
	}
}

// Hash password
func (user *Account) HashPassword(password string) error {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return err
	}
	hashedSecret := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	user.HashedSecret = hashedSecret
	user.Salt = salt
	return nil
}

// Validate password
func (user *Account) ValidateUser(password string) bool {
	hashedSecret := argon2.IDKey([]byte(password), []byte(user.Salt), 1, 64*1024, 4, 32)
	return string(hashedSecret) == string(user.HashedSecret)
}

// AddRole adds a role to an account
func (user *Account) AddRole(role string) {
	user.Roles = append(user.Roles, role)
}

// RemoveRole removes a role from an account
func (user *Account) RemoveRole(role string) {
	for i, r := range user.Roles {
		if r == role {
			user.Roles = append(user.Roles[:i], user.Roles[i+1:]...)
			break
		}
	}
}

// -------------- Functions --------------

// CreateAccountInDB creates an account in the database
func CreateAccountInDB(account Account) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(),
		"INSERT INTO accounts (user_id, username, email, hashed_secret, salt, roles) VALUES ($1, $2, $3, $4, $5, $6)",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to create account", err)
	}
	return database.SuccessResponse(account)
}

// GetAccountByID gets an account by ID
func GetAccountByID(userID uuid.UUID) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account", err)
	}

	var account *Account
	account, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account", err)
	}
	return database.SuccessResponse(*account)
}

// GetAccountByUsername gets an account by username
func GetAccountByUsername(username string) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM accounts WHERE username = $1", username)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account", err)
	}

	var account *Account
	account, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account", err)
	}
	return database.SuccessResponse(*account)
}

// GetAccountByEmail gets an account by email
func GetAccountByEmail(email string) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM accounts WHERE email = $1", email)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account", err)
	}

	var account *Account
	account, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account", err)
	}
	return database.SuccessResponse(*account)
}
