package auth

import (
	"context"
	"errors"
	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log"
	"time"
)

// ErrNotFound is returned by lookup methods that translate a "no rows"
// result into a stable, driver-independent sentinel so callers can tell a
// genuine not-found apart from a real query/connection error.
var ErrNotFound = errors.New("not found")

// Store interface
type Store interface {
	Account() AccountStore
	AccountSettings() AccountSettingsStore
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

// AccountSettings gets the account settings store
func (s *store) AccountSettings() AccountSettingsStore {
	return AccountSettingsStore(s)
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
//  updated_at timestamp with time zone default current_timestamp
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

// ErrEmailAlreadyExists is returned by AddAccountToDB when another account
// already has this exact email.
var ErrEmailAlreadyExists = errors.New("account with this email already exists")

// ErrUsernameAlreadyExists is returned by AddAccountToDB/UpdateAccountInDB
// when another account already has this exact, non-empty username.
var ErrUsernameAlreadyExists = errors.New("account with this username already exists")

// translateAccountConstraintErr maps a Postgres unique-violation on the
// accounts table to the matching sentinel, so callers never see a raw,
// driver-specific error for a condition they're expected to recover from.
func translateAccountConstraintErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "accounts_email_key":
			return ErrEmailAlreadyExists
		case "accounts_username_key":
			return ErrUsernameAlreadyExists
		}
	}
	return err
}

// AddAccountToDB creates an account in the database
func (s *store) AddAccountToDB(account *Account) error {
	_, err := s.db.Exec(context.Background(),
		"INSERT INTO accounts (user_id, username, email, hashed_secret, salt, roles) VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6)",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return translateAccountConstraintErr(err)
	}
	return nil
}

// GetAccountByID gets an account by ID
func (s *store) GetAccountByID(userID string) (*Account, error) {
	rows, err := s.db.Query(context.Background(), "SELECT user_id, COALESCE(username, '') AS username, email, hashed_secret, salt, roles, updated_at FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return account, nil
}

// GetAccountByUsername gets an account by username
func (s *store) GetAccountByUsername(username string) (*Account, error) {
	rows, err := s.db.Query(context.Background(), "SELECT user_id, COALESCE(username, '') AS username, email, hashed_secret, salt, roles, updated_at FROM accounts WHERE username = $1", username)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return account, nil
}

// GetAccountByEmail gets an account by email
func (s *store) GetAccountByEmail(email string) (*Account, error) {
	rows, err := s.db.Query(context.Background(), "SELECT user_id, COALESCE(username, '') AS username, email, hashed_secret, salt, roles, updated_at FROM accounts WHERE email = $1", email)
	if err != nil {
		return nil, err
	}

	account, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Account])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return account, nil
}

// UpdateAccountInDB updates an account in the database
func (s *store) UpdateAccountInDB(account *Account) error {
	_, err := s.db.Exec(context.Background(),
		"UPDATE accounts SET username = NULLIF($2, ''), email = $3, hashed_secret = $4, salt = $5, roles = $6 WHERE user_id = $1",
		account.UserID, account.Username, account.Email, account.HashedSecret, account.Salt, account.Roles,
	)
	if err != nil {
		return translateAccountConstraintErr(err)
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

	// ExpiresAt == 0 means never-expires (TTL 0); a past ExpiresAt would
	// otherwise yield a negative TTL, which Redis rejects, so skip caching it.
	var ttl time.Duration
	if session.ExpiresAt != 0 {
		ttl = time.Until(time.Unix(session.ExpiresAt, 0))
		if ttl <= 0 {
			return nil
		}
	}

	_, err = s.rdb.Set(context.Background(), "session:"+session.ID, stringSession, ttl).Result()
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
//   verified BOOLEAN NOT NULL DEFAULT false,
//   login_enabled BOOLEAN NOT NULL DEFAULT false,
//   created_at timestamp with time zone default current_timestamp,
//   updated_at timestamp with time zone default current_timestamp,
//   FOREIGN KEY (user_id) REFERENCES accounts(user_id),
//   CONSTRAINT linked_accounts_unique UNIQUE (user_id, platform),
//   CONSTRAINT linked_accounts_platform_unique UNIQUE (platform, platform_id)
// );

// LinkAccountStore - Account Link Store
type LinkAccountStore interface {
	AddLinkedAccountToDB(la *LinkedAccount) error
	UpdateLinkedAccount(la *LinkedAccount) error
	GetLinkedAccountByPlatformID(platform Platform, platformID string) (*LinkedAccount, error)
	GetLinkedAccountByPlatformName(platform Platform, platformName string) (*LinkedAccount, error)
	GetLinkedAccountByUserID(userID string, platform Platform) (*LinkedAccount, error)
	GetLinkedAccountsByUserID(userID string) ([]*LinkedAccount, error)
	DeleteLinkedAccount(userID string, platform Platform) error
	SetLinkedAccountLoginEnabled(userID string, platform Platform, enabled bool) error
}

// ErrAlreadyLinked is returned by AddLinkedAccountToDB when a concurrent
// insert already linked this exact (platform, platform_id) pair first -
// the caller lost the race and should re-fetch via
// GetLinkedAccountByPlatformID and use the winner's row instead of
// treating this as a hard failure.
var ErrAlreadyLinked = errors.New("platform account already linked")

// ErrDuplicateLinkedAccount is returned by GetLinkedAccountByPlatformID
// when more than one linked_accounts row matches the same (platform,
// platform_id) pair. Unlike ErrAlreadyLinked, this isn't auto-recovered:
// which row is "correct" isn't knowable from this query alone, so it
// fails closed and needs a manual data fix instead of guessing.
var ErrDuplicateLinkedAccount = errors.New("multiple linked accounts found for platform ID")

// ErrWouldLockAccount means the change would leave the account with no
// password and no other usable login method.
var ErrWouldLockAccount = errors.New("this is the account's last usable login method; set a password or link another platform first")

// ErrLinkedAccountUnverified means login can't be enabled for a row that
// isn't Verified.
var ErrLinkedAccountUnverified = errors.New("this linked account is unverified and can't be enabled for login")

// AddLinkedAccountToDB adds a linked account to the database
func (s *store) AddLinkedAccountToDB(la *LinkedAccount) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO linked_accounts (user_id, platform, platform_username, platform_id, data, verified, login_enabled) VALUES ($1, $2, $3, $4, $5, $6, $7)", la.UserID, la.Platform, la.PlatformUsername, la.PlatformID, la.Data, la.Verified, la.LoginEnabled)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "linked_accounts_platform_unique" {
			return ErrAlreadyLinked
		}
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if errors.Is(err, pgx.ErrTooManyRows) {
			return nil, ErrDuplicateLinkedAccount
		}
		return nil, err
	}
	return al, nil
}

// GetLinkedAccountByPlatformName gets a linked account by name and platform
func (s *store) GetLinkedAccountByPlatformName(platform Platform, platformName string) (*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE platform = $1 AND platform_username = $2", platform, platformName)
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
func (s *store) GetLinkedAccountByUserID(userID string, platform Platform) (*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE user_id = $1 AND platform = $2", userID, platform)
	if err != nil {
		return nil, err
	}

	al, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if errors.Is(err, pgx.ErrTooManyRows) {
			return nil, ErrDuplicateLinkedAccount
		}
		return nil, err
	}
	return al, nil
}

// GetLinkedAccountsByUserID returns every platform linked to userID.
func (s *store) GetLinkedAccountsByUserID(userID string) ([]*LinkedAccount, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
}

// DeleteLinkedAccount unlinks a platform from an account. It refuses if
// doing so would leave the account with no password and no other verified,
// login-enabled linked platform. If RowsAffected == 0, a follow-up lookup
// tells "no such link" apart from "the guard blocked it".
func (s *store) DeleteLinkedAccount(userID string, platform Platform) error {
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(27745, hashtext($1))", userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM linked_accounts WHERE user_id = $1 AND platform = $2 AND (
		EXISTS (
			SELECT 1 FROM accounts WHERE user_id = $1 AND hashed_secret IS NOT NULL
			AND COALESCE((SELECT password_auth FROM account_settings WHERE user_id = $1), true)
		)
		OR EXISTS (SELECT 1 FROM linked_accounts WHERE user_id = $1 AND platform != $2 AND verified = true AND login_enabled = true)
	)`, userID, platform)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if tag.RowsAffected() > 0 {
		return nil
	}
	if _, err := s.GetLinkedAccountByUserID(userID, platform); err != nil {
		return err
	}
	return ErrWouldLockAccount
}

// SetLinkedAccountLoginEnabled toggles login eligibility for a linked
// platform. Disabling shares DeleteLinkedAccount's guard; enabling requires
// the row to be Verified.
func (s *store) SetLinkedAccountLoginEnabled(userID string, platform Platform, enabled bool) error {
	ctx := context.Background()
	var tag pgconn.CommandTag

	if enabled {
		var err error
		tag, err = s.db.Exec(ctx, "UPDATE linked_accounts SET login_enabled = true, updated_at = current_timestamp WHERE user_id = $1 AND platform = $2 AND verified = true", userID, platform)
		if err != nil {
			return err
		}
	} else {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(27745, hashtext($1))", userID); err != nil {
			return err
		}
		tag, err = tx.Exec(ctx, `UPDATE linked_accounts SET login_enabled = false, updated_at = current_timestamp WHERE user_id = $1 AND platform = $2 AND (
			EXISTS (
				SELECT 1 FROM accounts WHERE user_id = $1 AND hashed_secret IS NOT NULL
				AND COALESCE((SELECT password_auth FROM account_settings WHERE user_id = $1), true)
			)
			OR EXISTS (SELECT 1 FROM linked_accounts WHERE user_id = $1 AND platform != $2 AND verified = true AND login_enabled = true)
		)`, userID, platform)
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	if tag.RowsAffected() > 0 {
		return nil
	}
	if _, err := s.GetLinkedAccountByUserID(userID, platform); err != nil {
		return err
	}
	if enabled {
		return ErrLinkedAccountUnverified
	}
	return ErrWouldLockAccount
}

//CREATE TRIGGER update_account_settings_modtime
//BEFORE UPDATE ON account_settings
//FOR EACH ROW
//EXECUTE PROCEDURE update_modified_column();

// CREATE TABLE account_settings (
//   user_id BIGINT PRIMARY KEY NOT NULL REFERENCES accounts(user_id),
//   password_auth BOOLEAN NOT NULL DEFAULT true,
//   updated_at timestamp with time zone default current_timestamp
// );

// AccountSettingsStore interface
type AccountSettingsStore interface {
	GetAccountSettings(userID string) (*AccountSettings, error)
	SetPasswordAuthEnabled(userID string, enabled bool) error
}

// ErrNoPasswordSet is returned by SetPasswordAuthEnabled(userID, true) when
// the account has no hashed_secret to enable login with.
var ErrNoPasswordSet = errors.New("this account has no password set")

// GetAccountSettings returns userID's settings, or the defaults if it has
// no account_settings row yet.
func (s *store) GetAccountSettings(userID string) (*AccountSettings, error) {
	rows, err := s.db.Query(context.Background(), "SELECT * FROM account_settings WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}

	settings, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[AccountSettings])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DefaultAccountSettings(userID), nil
		}
		return nil, err
	}
	return settings, nil
}

// SetPasswordAuthEnabled toggles whether userID's password can be used to
// log in. Disabling shares DeleteLinkedAccount/SetLinkedAccountLoginEnabled's
// guard: it's refused if the account has no other verified, login-enabled
// linked platform. Enabling is refused if the account has no password to
// enable in the first place.
func (s *store) SetPasswordAuthEnabled(userID string, enabled bool) error {
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(27745, hashtext($1))", userID); err != nil {
		return err
	}

	var tag pgconn.CommandTag
	if enabled {
		tag, err = tx.Exec(ctx, `INSERT INTO account_settings (user_id, password_auth)
			SELECT $1, true WHERE EXISTS (SELECT 1 FROM accounts WHERE user_id = $1 AND hashed_secret IS NOT NULL)
			ON CONFLICT (user_id) DO UPDATE SET password_auth = true, updated_at = current_timestamp`, userID)
	} else {
		tag, err = tx.Exec(ctx, `INSERT INTO account_settings (user_id, password_auth)
			SELECT $1, false WHERE EXISTS (SELECT 1 FROM linked_accounts WHERE user_id = $1 AND verified = true AND login_enabled = true)
			ON CONFLICT (user_id) DO UPDATE SET password_auth = false, updated_at = current_timestamp`, userID)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if tag.RowsAffected() > 0 {
		return nil
	}
	if enabled {
		return ErrNoPasswordSet
	}
	return ErrWouldLockAccount
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
	rediskey := "rl:" + key
	_, err := s.rdb.Incr(context.Background(), rediskey).Result()
	if err != nil {
		return err
	}
	// Only set the TTL if this increment created the key, so an existing
	// key's window doesn't reset on every request.
	_, err = s.rdb.ExpireNX(context.Background(), rediskey, time.Minute).Result()
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
	AccessToken  string    `json:"access_token" db:"access_token"`
	TokenType    string    `json:"token_type,omitempty" db:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty" db:"refresh_token"`
	Expiry       int64     `json:"expiry,omitempty" db:"expiry"`
	ExpiresIn    int64     `json:"expires_in,omitempty" db:"expires_in"`
	UserID       string    `json:"user_id" db:"user_id"`
	Platform     Platform  `json:"platform" db:"platform"`
	Scope        []string  `json:"scope" db:"scope"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// OAuthTokenStore interface
type OAuthTokenStore interface {
	AddOAuthTokenToDB(token *OAuthToken) error
	GetOAuthTokenByUserID(userID string, platform Platform) (*OAuthToken, error)
	UpdateOAuthToken(token *OAuthToken) error
	DeleteOAuthToken(userID string, platform Platform) error
}

// AddOAuthTokenToDB adds an OAuth token to the database
func (s *store) AddOAuthTokenToDB(token *OAuthToken) error {
	_, err := s.db.Exec(context.Background(),
		"INSERT INTO oauth_tokens (user_id, platform, access_token, token_type, refresh_token, expiry, expires_in, scope) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		token.UserID, token.Platform, token.AccessToken, token.TokenType, token.RefreshToken, token.Expiry, token.ExpiresIn, token.Scope)
	if err != nil {
		return err
	}
	return nil
}

// GetOAuthTokenByUserID gets an OAuth token by user ID and platform
func (s *store) GetOAuthTokenByUserID(userID string, platform Platform) (*OAuthToken, error) {
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
		token.UserID, token.AccessToken, token.TokenType, token.RefreshToken, token.Expiry, token.ExpiresIn, token.Scope, token.Platform)
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
