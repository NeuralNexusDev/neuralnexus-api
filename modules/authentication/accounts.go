package authentication

import (
	"crypto/rand"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// -------------- Structs --------------

// Account struct
type Account struct {
	UserID       uuid.UUID `json:"user_id"`       // User ID
	Username     string    `json:"username"`      // Username
	Email        string    `json:"email"`         // Email
	HashedSecret string    `json:"hashed_secret"` // Hashed secret
	Salt         string    `json:"salt"`          // Salt
	Roles        []Role    `json:"roles"`         // Roles
}

// NewAccount creates a new account
func NewAccount(username string, email string, password string) (Account, error) {
	user := Account{
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
	user.HashedSecret = string(hashedSecret)
	user.Salt = string(salt)
	return nil
}

// Validate password
func (user *Account) ValidateUser(password string) bool {
	hashedSecret := argon2.IDKey([]byte(password), []byte(user.Salt), 1, 64*1024, 4, 32)
	return user.HashedSecret == string(hashedSecret)
}
