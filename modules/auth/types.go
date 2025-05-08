package auth

import (
	"crypto/rand"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"

	"log"
	"os"
	"time"

	_ "unsafe"
)

// -------------- Account --------------

var pepper = []byte(os.Getenv("PEPPER"))

// Account struct
type Account struct {
	UserID       string    `db:"user_id" validate:"required" json:"user_id" xml:"user_id"`
	Username     string    `db:"username" json:"username" xml:"username"`
	Email        string    `db:"email" json:"-" xml:"-"`
	HashedSecret []byte    `db:"hashed_secret" json:"-" xml:"-"`
	Salt         []byte    `db:"salt" json:"-" xml:"-"`
	Roles        []string  `db:"roles" json:"roles" xml:"roles"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at" xml:"updated_at"`
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

// NewIDOnlyAccount creates a new account with only an ID
func NewIDOnlyAccount() (*Account, error) {
	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	return &Account{
		UserID: id,
	}, nil
}

//go:linkname deriveKey golang.org/x/crypto/argon2.deriveKey
//goland:noinspection GoUnusedParameter
func deriveKey(mode int, password, salt, secret, data []byte, time, memory uint32, threads uint8, keyLen uint32) []byte

//go:linkname argon2id golang.org/x/crypto/argon2.argon2id
var argon2id int

// IDKeyWithSecret Adds pepper support to the IDKey function
func IDKeyWithSecret(password, salt []byte, secret []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
	return deriveKey(argon2id, password, salt, secret, nil, time, memory, threads, keyLen)
}

// HashPassword hashes the password
func (user *Account) HashPassword(password string) error {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return err
	}
	hashedSecret := IDKeyWithSecret([]byte(password), salt, pepper, 1, 64*1024, 4, 32)
	user.HashedSecret = hashedSecret
	user.Salt = salt
	return nil
}

// ValidateUser validate the user's password
func (user *Account) ValidateUser(password string) bool {
	if user.HashedSecret == nil || user.Salt == nil {
		return false
	}
	hashedSecret := IDKeyWithSecret([]byte(password), user.Salt, pepper, 1, 64*1024, 4, 32)
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

// -------------- Session --------------

// NewSession creates a new session
func (user *Account) NewSession(expiresAt int64) (*Session, error) {
	var permissions []string
	for _, r := range user.Roles {
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
	return &Session{
		ID:          id,
		UserID:      user.UserID,
		Permissions: permissions,
		IssuedAt:    time.Now().Unix(),
		LastUsedAt:  time.Now().Unix(),
		ExpiresAt:   expiresAt,
	}, nil
}

// -------------- Account Linking --------------

// -------------- Structs --------------

// OAuthState used with the OAuth state URL parameter
type OAuthState struct {
	Platform    Platform `json:"platform"`
	Nonce       string   `json:"nonce"`
	RedirectURI string   `json:"redirect_uri"`
}

// LinkedAccount struct
type LinkedAccount struct {
	UserID           string      `db:"user_id" validate:"required"`
	Platform         Platform    `db:"platform" validate:"required"`
	PlatformUsername string      `db:"platform_username" validate:"required_without=PlatformID"`
	PlatformID       string      `db:"platform_id" validate:"required_without=PlatformUsername"`
	Data             interface{} `db:"data" validate:"required"`
	DataUpdatedAt    time.Time   `db:"updated_at"`
	CreatedAt        time.Time   `db:"created_at"`
}

// NewLinkedAccount creates a new linked account
func NewLinkedAccount(userID string, platform Platform, platformUsername, platformID string, data Data) *LinkedAccount {
	return &LinkedAccount{
		UserID:           userID,
		Platform:         platform,
		PlatformUsername: platformUsername,
		PlatformID:       platformID,
		Data:             data,
	}
}

// Data interface
type Data interface {
	PlatformID() string
	PlatformUsername() string
	PlatformData() string
	CreateLinkedAccount(string) *LinkedAccount
}

// -------------- Enums --------------

type Platform string

var (
	PlatformDiscord   Platform = "discord"
	PlatformMinecraft Platform = "minecraft"
	PlatformTwitch    Platform = "twitch"
)
