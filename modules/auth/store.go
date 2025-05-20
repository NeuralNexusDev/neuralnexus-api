package auth

import (
	"context"
	"errors"
	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"log"
	"time"
)

// Store interface
type Store interface {
	Account() AccountStore
	Session() SessionStore
	LinkAccount() LinkAccountStore
	RateLimit() RateLimitStore
	OAuthToken() OAuthTokenStore
}

// store - primary store for auth
type store struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

// NewStore - Create a new store
func NewStore(db *pgxpool.Pool, rdb *redis.Client) Store {
	return &store{
		db:  db,
		rdb: rdb,
	}
}

// Account gets the account store
func (s *store) Account() AccountStore {
	return AccountStore(s)
}

// Session gets the session store
func (s *store) Session() SessionStore {
	return SessionStore(s)
}

// LinkAccount gets the linked account store
func (s *store) LinkAccount() LinkAccountStore {
	return LinkAccountStore(s)
}

// RateLimit gets the rate limit store
func (s *store) RateLimit() RateLimitStore {
	return RateLimitStore(s)
}

// OAuthToken gets the OAuth token store
func (s *store) OAuthToken() OAuthTokenStore {
	return OAuthTokenStore(s)
}

//CREATE TRIGGER update_accounts_modtime
//BEFORE UPDATE ON accounts
//FOR EACH ROW
//EXECUTE PROCEDURE update_modified_column();

// CREATE TABLE accounts (
// 	user_id BIGINT PRIMARY KEY NOT NULL,
// 	username TEXT UNIQUE,
// 	email TEXT UNIQUE,
// 	hashed_secret BYTEA,
// 	salt BYTEA,
// 	roles TEXT[],
//  updated_at timestamp with time zone default current_timestamp,
//  CONSTRAINT email_unique CHECK (email IS NOT NULL),
//  CONSTRAINT password_enforced CHECK (email IS NOT NULL OR hashed_secret IS NOT NULL)
// );

// AccountStore interface
type AccountStore interface {
	AddAccountToDB(account *Account) error
	GetAccountByID(userID string) (*Account, error)
	GetAccountByUsername(username string) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
	UpdateAccountInDB(account *Account) error
	DeleteAccountFromDB(userID string) error
}

// AddAccountToDB creates an account in the database
func (s *store) AddAccountToDB(account *Account) error {
	_, err := s.db.Exec(context.Background(),
		"INSERT INTO accounts (user_id, username, email, hashed_secret, salt, roles) VALUES ($1, $2, $3, $4, $5, $6)",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return err
	}
	return nil
}

// GetAccountByID gets an account by ID
func (s *store) GetAccountByID(userID string) (*Account, error) {
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
func (s *store) GetAccountByUsername(username string) (*Account, error) {
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
func (s *store) GetAccountByEmail(email string) (*Account, error) {
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

// UpdateAccountInDB updates an account in the database
func (s *store) UpdateAccountInDB(account *Account) error {
	_, err := s.db.Exec(context.Background(),
		"UPDATE accounts SET username = $2, email = $3, hashed_secret = $4, salt = $5, roles = $6 WHERE user_id = $1",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return err
	}
	return nil
}

// DeleteAccountFromDB deletes an account from the database
func (s *store) DeleteAccountFromDB(userID string) error {
	_, err := s.db.Exec(context.Background(), "DELETE FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return err
	}
	return nil
}

// CREATE TABLE sessions (
// 	session_id BIGINT PRIMARY KEY NOT NULL,
// 	user_id BIGINT NOT NULL,
// 	permissions TEXT[] NOT NULL,
// 	iat BIGINT NOT NULL,
// 	lua BIGINT NOT NULL,
// 	exp BIGINT NOT NULL,
//  FOREIGN KEY (user_id) REFERENCES accounts(user_id)
// );

// SessionStore interface
type SessionStore interface {
	AddSessionToDB(session *Session) error
	GetSessionFromDB(id string) (*Session, error)
	UpdateSessionInDB(session *Session) error
	DeleteSessionInDB(id string) error
	AddSessionToCache(session *Session) error
	GetSessionFromCache(id string) (*Session, error)
	DeleteSessionFromCache(id string) error
}

// AddSessionToDB creates a session and inserts it into the database
func (s *store) AddSessionToDB(session *Session) error {
	defer s.ClearExpiredSessions()

	_, err := s.db.Exec(context.Background(),
		"INSERT INTO sessions (session_id, user_id, permissions, iat, lua, exp) VALUES ($1, $2, $3, $4, $5, $6)",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)
	if err != nil {
		return err
	}
	return nil
}

// GetSessionFromDB gets a session by ID
func (s *store) GetSessionFromDB(id string) (*Session, error) {
	defer s.ClearExpiredSessions()

	var session *Session
	rows, err := s.db.Query(context.Background(), "SELECT * FROM sessions WHERE session_id = $1", id)
	if err != nil {
		return nil, err
	}

	session, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Session])
	if err != nil {
		return nil, err
	}
	return session, nil
}

// DeleteSessionInDB deletes a session by ID
func (s *store) DeleteSessionInDB(id string) error {
	defer s.ClearExpiredSessions()

	_, err := s.db.Exec(context.Background(), "DELETE FROM sessions WHERE session_id = $1", id)
	if err != nil {
		return err
	}
	return nil
}

// UpdateSessionInDB updates a session
func (s *store) UpdateSessionInDB(session *Session) error {
	defer s.ClearExpiredSessions()

	_, err := s.db.Exec(context.Background(),
		"UPDATE sessions SET user_id = $2, permissions = $3, iat = $4, lua = $5, exp = $6 WHERE session_id = $1",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)
	if err != nil {
		return err
	}
	return nil
}

// ClearExpiredSessions clear expired sessions
// TODO: Add this to interface and handle the error outside of it
func (s *store) ClearExpiredSessions() {
	_, err := s.db.Exec(context.Background(), "DELETE FROM sessions WHERE exp < $1 AND exp != 0", time.Now().Unix())
	if err != nil {
		log.Println("Unable to clear expired sessions:")
		log.Println(err)
	}
}

// -------------- Cache Functions --------------

// AddSessionToCache adds a session to the cache
func (s *store) AddSessionToCache(session *Session) error {
	stringSession, err := json.Marshal(session)
	if err != nil {
		return err
	}

	_, err = s.rdb.Set(context.Background(), "session:"+session.ID, stringSession, time.Until(time.Unix(session.ExpiresAt, 0))).Result()
	if err != nil {
		return err
	}
	return nil
}

// GetSessionFromCache gets a session from the cache
func (s *store) GetSessionFromCache(id string) (*Session, error) {
	var session Session
	stringSession, err := s.rdb.Get(context.Background(), "session:"+id).Result()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(stringSession), &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// DeleteSessionFromCache deletes a session from the cache
func (s *store) DeleteSessionFromCache(id string) error {
	_, err := s.rdb.Del(context.Background(), "session:"+id).Result()
	if err != nil {
		return err
	}
	return nil
}

//CREATE TRIGGER update_linked_accounts_modtime
//BEFORE UPDATE ON linked_accounts
//FOR EACH ROW
//EXECUTE PROCEDURE update_modified_column();

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
	AddLinkedAccountToDB(la *LinkedAccount) error
	UpdateLinkedAccount(la *LinkedAccount) error
	GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error)
	GetLinkedAccountByUserID(userID string, platform string) (*LinkedAccount, error)
}

// AddLinkedAccountToDB adds a linked account to the database
func (s *store) AddLinkedAccountToDB(la *LinkedAccount) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO linked_accounts (user_id, platform, platform_username, platform_id, data) VALUES ($1, $2, $3, $4, $5)", la.UserID, la.Platform, la.PlatformUsername, la.PlatformID, la.Data)
	if err != nil {
		return err
	}
	return nil
}

// UpdateLinkedAccount updates a linked account in the database
func (s *store) UpdateLinkedAccount(la *LinkedAccount) error {
	_, err := s.db.Exec(context.Background(), "UPDATE linked_accounts SET platform_username = $1, platform_id = $2, data = $3, updated_at = current_timestamp WHERE user_id = $4 AND platform = $5", la.PlatformUsername, la.PlatformID, la.Data, la.UserID, la.Platform)
	if err != nil {
		return err
	}
	return nil
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

// RateLimitStore interface
type RateLimitStore interface {
	GetRateLimit(key string) (int, error)
	SetRateLimit(key string, val int) error
	IncrementRateLimit(key string) error
}

// GetRateLimit gets the rate limit for a key
func (s *store) GetRateLimit(key string) (int, error) {
	val, err := s.rdb.Get(context.Background(), "rl:"+key).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = s.SetRateLimit(key, 1)
			if err != nil {
				return 0, err
			}
			return 1, nil
		}
		return 0, err
	}
	return val, nil
}

// SetRateLimit sets the rate limit for a key
func (s *store) SetRateLimit(key string, val int) error {
	_, err := s.rdb.Set(context.Background(), "rl:"+key, val, time.Minute).Result()
	if err != nil {
		return err
	}
	return nil
}

// IncrementRateLimit increments the rate limit for a key
func (s *store) IncrementRateLimit(key string) error {
	ttl, err := s.rdb.TTL(context.Background(), "rl:"+key).Result()
	if err != nil {
		return err
	}
	_, err = s.rdb.IncrBy(context.Background(), "rl:"+key, 1).Result()
	if err != nil {
		return err
	}
	_, err = s.rdb.Expire(context.Background(), "rl:"+key, ttl).Result()
	if err != nil {
		return err
	}
	return nil
}

//CREATE TRIGGER update_oauth_tokens_modtime
//BEFORE UPDATE ON oauth_tokens
//FOR EACH ROW
//EXECUTE PROCEDURE update_modified_column();

//CREATE TABLE oauth_tokens (
//	user_id BIGINT NOT NULL,
//	platform TEXT NOT NULL,
//	access_token TEXT NOT NULL,
//  token_type TEXT,
//  refresh_token TEXT,
//	expiry BIGINT,
//  expires_in BIGINT,
//  scope TEXT[],
//  created_at timestamp with time zone default current_timestamp,
//  updated_at timestamp with time zone default current_timestamp,
//  FOREIGN KEY (user_id) REFERENCES accounts(user_id),
//  CONSTRAINT oauth_tokens_unique UNIQUE (user_id, platform)
//);

// OAuthToken OAuth2 token with scope
type OAuthToken struct {
	*oauth2.Token
	AccessToken  string    `json:"access_token" db:"access_token"`
	TokenType    string    `json:"token_type,omitempty" db:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty" db:"refresh_token"`
	Expiry       time.Time `json:"expiry,omitempty" db:"expiry"`
	ExpiresIn    int64     `json:"expires_in,omitempty" db:"expires_in"`
	UserID       string    `json:"user_id" db:"user_id"`
	Scope        []string  `json:"scope" db:"scope"`
}

// OAuthTokenStore interface
type OAuthTokenStore interface {
	AddOAuthTokenToDB(token *OAuthToken) error
	GetOAuthTokenByUserID(userID string, platform string) (*OAuthToken, error)
	UpdateOAuthToken(token *OAuthToken) error
	DeleteOAuthToken(userID string, platform Platform) error
}

// AddOAuthTokenToDB adds an OAuth token to the database
func (s *store) AddOAuthTokenToDB(token *OAuthToken) error {
	_, err := s.db.Exec(context.Background(),
		"INSERT INTO oauth_tokens (user_id, platform, access_token, token_type, refresh_token, expiry, expires_in, scope) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		token.UserID, token.TokenType, token.AccessToken, token.RefreshToken, token.Expiry.Unix(), token.ExpiresIn, token.Scope)
	if err != nil {
		return err
	}
	return nil
}

// GetOAuthTokenByUserID gets an OAuth token by user ID and platform
func (s *store) GetOAuthTokenByUserID(userID string, platform string) (*OAuthToken, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM oauth_tokens WHERE user_id = $1 AND platform = $2", userID, platform)
	if err != nil {
		return nil, err
	}

	token, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[OAuthToken])
	if err != nil {
		return nil, err
	}
	return token, nil
}

// UpdateOAuthToken updates an OAuth token in the database
func (s *store) UpdateOAuthToken(token *OAuthToken) error {
	_, err := s.db.Exec(context.Background(),
		"UPDATE oauth_tokens SET access_token = $2, token_type = $3, refresh_token = $4, expiry = $5, expires_in = $6, scope = $7 WHERE user_id = $1 AND platform = $8",
		token.UserID, token.AccessToken, token.TokenType, token.RefreshToken, token.Expiry.Unix(), token.ExpiresIn, token.Scope)
	if err != nil {
		return err
	}
	return nil
}

// DeleteOAuthToken deletes an OAuth token from the database
func (s *store) DeleteOAuthToken(userID string, platform Platform) error {
	_, err := s.db.Exec(context.Background(), "DELETE FROM oauth_tokens WHERE user_id = $1 AND platform = $2", userID, platform)
	if err != nil {
		return err
	}
	return nil
}
