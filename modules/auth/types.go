package auth

import (
	"crypto/rand"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

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
func NewAccount(username, email, password string) (*Account, error) {
	user := &Account{
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
func NewPasswordLessAccount(username, email string) *Account {
	return &Account{
		UserID:   uuid.New(),
		Username: username,
		Email:    email,
	}
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

// Session struct
type Session struct {
	ID          uuid.UUID `json:"session_id" xml:"session_id" db:"session_id"`
	UserID      uuid.UUID `json:"user_id" xml:"user_id" db:"user_id"`
	Permissions []string  `json:"permissions" xml:"permissions" db:"permissions"`
	IssuedAt    int64     `json:"iat" xml:"iat" db:"iat"`
	LastUsedAt  int64     `json:"lua" xml:"lua" db:"lua"`
	ExpiresAt   int64     `json:"exp" xml:"exp" db:"exp"`
}

// NewSession creates a new session
func (a *Account) NewSession(expiresAt int64) *Session {
	permissions := []string{}
	for _, r := range a.Roles {
		role, err := GetRoleByName(r)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, p := range role.Permissions {
			permissions = append(permissions, p.Name+"|"+p.Value)
		}
	}

	return &Session{
		ID:          uuid.New(),
		UserID:      a.UserID,
		Permissions: permissions,
		IssuedAt:    time.Now().Unix(),
		LastUsedAt:  time.Now().Unix(),
		ExpiresAt:   expiresAt,
	}
}

// HasPermission checks if a session has a permission
func (s *Session) HasPermission(permission Scope) bool {
	for _, p := range s.Permissions {
		if p == permission.Name+"|"+permission.Value {
			return true
		}
	}
	return false
}

// IsExpired checks if a session is expired
func (s *Session) IsValid() bool {
	if s.ExpiresAt == 0 {
		return true
	}
	return time.Now().Unix() < s.ExpiresAt
}
