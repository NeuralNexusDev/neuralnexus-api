package session

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

// ErrNotFound when a session lookup finds nothing.
var ErrNotFound = errors.New("not found")

type store struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

// NewStore - Create a new session store
func NewStore(db *pgxpool.Pool, rdb *redis.Client) Store {
	return &store{
		db:  db,
		rdb: rdb,
	}
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

// Store - The session store interface
type Store interface {
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
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

	_, err = s.rdb.Set(context.Background(), sessionKeyPrefix+session.ID, stringSession, ttl).Result()
	if err != nil {
		return err
	}
	return nil
}

// GetSessionFromCache gets a session from the cache
func (s *store) GetSessionFromCache(id string) (*Session, error) {
	var session Session
	stringSession, err := s.rdb.Get(context.Background(), sessionKeyPrefix+id).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
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
	_, err := s.rdb.Del(context.Background(), sessionKeyPrefix+id).Result()
	if err != nil {
		return err
	}
	return nil
}
