package linking

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
)

// NewOAuthTokenStore - Create a new OAuth token store
func NewOAuthTokenStore(db *pgxpool.Pool) OAuthTokenStore {
	return &store{db: db}
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authaccount.ErrNotFound
		}
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
