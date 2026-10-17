package auth

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	authsession "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
)

// Store interface
type Store interface {
	Account() authaccount.AccountStore
	AccountSettings() authaccount.AccountSettingsStore
	Session() authsession.SessionStore
	LinkAccount() linking.LinkAccountStore
	OAuthToken() linking.OAuthTokenStore
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
func (s *store) Account() authaccount.AccountStore {
	return authaccount.NewStore(s.db)
}

// AccountSettings gets the account settings store
func (s *store) AccountSettings() authaccount.AccountSettingsStore {
	return authaccount.NewStore(s.db)
}

// Session gets the session store
func (s *store) Session() authsession.SessionStore {
	return authsession.NewStore(s.db, s.rdb)
}

// LinkAccount gets the linked account store
func (s *store) LinkAccount() linking.LinkAccountStore {
	return linking.NewLinkAccountStore(s.db)
}

// OAuthToken gets the OAuth token store
func (s *store) OAuthToken() linking.OAuthTokenStore {
	return linking.NewOAuthTokenStore(s.db)
}
