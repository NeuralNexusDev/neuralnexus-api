package auth

import (
	"crypto/rand"
	"log"
	"time"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	sess "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"golang.org/x/crypto/argon2"
)

// Account struct
type Account struct {
	UserID       string    `db:"user_id" validate:"required"`
	Username     string    `db:"username" validate:"required_without=Email"`
	Email        string    `db:"email" validate:"required_without=Username"`
	HashedSecret []byte    `db:"hashed_secret" validate:"required_without=Email"`
	Salt         []byte    `db:"salt"`
	Roles        []string  `db:"roles"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// NewAccount creates a new account
func NewAccount(username, email, password string) (*Account, error) {
	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	user := &Account{
		UserID:   id,
		Username: username,
		Email:    email,
	}
	err = user.HashPassword(password)
	if err != nil {
		return user, err
	}
	return user, nil
}

// NewPasswordLessAccount creates a new account without a password
func NewPasswordLessAccount(username, email string) (*Account, error) {
	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	return &Account{
		UserID:   id,
		Username: username,
		Email:    email,
	}, nil
}

// HashPassword hashes the password
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
	if user.HashedSecret == nil || user.Salt == nil {
		return false
	}
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

// NewSession creates a new session
func (a *Account) NewSession(expiresAt int64) (*sess.Session, error) {
	permissions := []string{}
	for _, r := range a.Roles {
		role, err := perms.GetRoleByName(r)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, p := range role.Permissions {
			permissions = append(permissions, p.Name+"|"+p.Value)
		}
	}

	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	return &sess.Session{
		ID:          id,
		UserID:      a.UserID,
		Permissions: permissions,
		IssuedAt:    time.Now().Unix(),
		LastUsedAt:  time.Now().Unix(),
		ExpiresAt:   expiresAt,
	}, nil
}
