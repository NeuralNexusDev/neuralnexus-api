package auth

import (
	"context"
	"crypto/rand"
	"log"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// CREATE TABLE accounts (
// 	user_id UUID PRIMARY KEY NOT NULL,
// 	username TEXT NOT NULL,
// 	email TEXT NOT NULL,
// 	hashed_secret BYTEA NOT NULL,
// 	salt BYTEA NOT NULL,
// 	roles TEXT[]
// );

// -------------- Structs --------------

// Account struct
type Account struct {
	UserID       uuid.UUID `json:"user_id"`       // User ID
	Username     string    `json:"username"`      // Username
	Email        string    `json:"email"`         // Email
	HashedSecret []byte    `json:"hashed_secret"` // Hashed secret
	Salt         []byte    `json:"salt"`          // Salt
	Roles        []string  `json:"roles"`         // Roles
}

// NewAccount creates a new account
func NewAccount(username string, email string, password string) (Account, error) {
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

// -------------- Functions --------------

// CreateAccountInDB creates an account in the database
func CreateAccountInDB(account Account) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	_, err := db.Exec(context.Background(),
		"INSERT INTO accounts (user_id, username, email, hashed_secret, salt, roles) VALUES ($1, $2, $3, $4, $5, $6)",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		log.Println(err)
		return database.Response[Account]{
			Success: false,
			Message: "Unable to create account",
		}
	}

	return database.Response[Account]{
		Success: true,
		Data:    account,
	}
}

// GetAccountByID gets an account by ID
func GetAccountByID(userID uuid.UUID) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	row := db.QueryRow(context.Background(), "SELECT * FROM accounts WHERE user_id = $1", userID)

	account := Account{}
	err := row.Scan(&account.UserID, &account.Username, &account.Email, &account.HashedSecret, &account.Salt, &account.Roles)
	if err != nil {
		log.Println(err)
		return database.Response[Account]{
			Success: false,
			Message: "Unable to get account",
		}
	}

	return database.Response[Account]{
		Success: true,
		Data:    account,
	}
}

// GetAccountByUsername gets an account by username
func GetAccountByUsername(username string) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	row := db.QueryRow(context.Background(), "SELECT * FROM accounts WHERE username = $1", username)

	account := Account{}
	err := row.Scan(&account.UserID, &account.Username, &account.Email, &account.HashedSecret, &account.Salt, &account.Roles)
	if err != nil {
		log.Println(err)
		return database.Response[Account]{
			Success: false,
			Message: "Unable to get account",
		}
	}

	return database.Response[Account]{
		Success: true,
		Data:    account,
	}
}
