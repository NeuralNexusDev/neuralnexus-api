package auth

import (
	"context"
	"crypto/rand"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	UserID       uuid.UUID `db:"user_id"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	HashedSecret []byte    `db:"hashed_secret"`
	Salt         []byte    `db:"salt"`
	Roles        []string  `db:"roles"`
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
	defer db.Close()

	_, err := db.Exec(context.Background(),
		"INSERT INTO accounts (user_id, username, email, hashed_secret, salt, roles) VALUES ($1, $2, $3, $4, $5, $6)",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to create account")
	}
	return database.SuccessResponse(account)
}

// GetAccountByID gets an account by ID
func GetAccountByID(userID uuid.UUID) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account")
	}

	var account *Account
	account, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account")
	}
	return database.SuccessResponse(*account)
}

// GetAccountByUsername gets an account by username
func GetAccountByUsername(username string) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM accounts WHERE username = $1", username)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account")
	}

	var account *Account
	account, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account")
	}
	return database.SuccessResponse(*account)
}

// GetAccountByEmail gets an account by email
func GetAccountByEmail(email string) database.Response[Account] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM accounts WHERE email = $1", email)
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account")
	}

	var account *Account
	account, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		return database.ErrorResponse[Account]("Unable to get account")
	}
	return database.SuccessResponse(*account)
}

// -------------- Routes --------------

// Path: /auth/login
// Method: POST
// Body: { "username | email": string, "password": string }

// LoginHandler handles the login route
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var login struct {
		Username string `json:"username" xml:"username" validate:"required_without=Email"`
		Email    string `json:"email" xml:"email" validate:"required_without=Username"`
		Password string `json:"password" xml:"password" validate:"required"`
	}
	err := responses.DecodeStruct(r, &login)
	if err != nil {
		responses.SendAndEncodeBadRequest(w, r, "Invalid request body")
		return
	}

	var account database.Response[Account]
	if login.Username != "" {
		account = GetAccountByUsername(login.Username)
	} else {
		account = GetAccountByEmail(login.Email)
	}
	if !account.Success {
		responses.SendAndEncodeBadRequest(w, r, "Invalid username or email")
		return
	}

	if !account.Data.ValidateUser(login.Password) {
		responses.SendAndEncodeBadRequest(w, r, "Invalid password")
		return
	}

	session := CreateSession(Session{
		UserID:      account.Data.UserID,
		Permissions: account.Data.Roles,
	})
	if !session.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Unable to create session")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusAccepted, session.Data)
}
