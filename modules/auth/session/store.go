package sess

import (
	"context"
	"log"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// CREATE TABLE sessions (
// 	session_id UUID PRIMARY KEY NOT NULL,
// 	user_id UUID NOT NULL,
// 	permissions TEXT[] NOT NULL,
// 	iat BIGINT NOT NULL,
// 	lua BIGINT NOT NULL,
// 	exp BIGINT NOT NULL,
//  FOREIGN KEY (user_id) REFERENCES accounts(user_id)
// );

// SessionStore interface
type SessionStore interface {
	AddSessionToDB(session *Session) (*Session, error)
	GetSessionFromDB(id uuid.UUID) (*Session, error)
	UpdateSessionInDB(session *Session) (*Session, error)
	DeleteSessionInDB(id uuid.UUID) (*Session, error)
	AddSessionToCache(session *Session) (*Session, error)
	GetSessionFromCache(id uuid.UUID) (*Session, error)
	DeleteSessionFromCache(id uuid.UUID) (*Session, error)
}

// sessStore - SessionStore implementation
type sessStore struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

// NewSessionStore - Create a new session store
func NewSessionStore(db *pgxpool.Pool, rdb *redis.Client) SessionStore {
	return &sessStore{
		db:  db,
		rdb: rdb,
	}
}

// AddSessionToDB creates a session and inserts it into the database
func (s *sessStore) AddSessionToDB(session *Session) (*Session, error) {
	defer s.ClearExpiredSessions()

	_, err := s.db.Exec(context.Background(),
		"INSERT INTO sessions (session_id, user_id, permissions, iat, lua, exp) VALUES ($1, $2, $3, $4, $5, $6)",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetSessionFromDB gets a session by ID
func (s *sessStore) GetSessionFromDB(id uuid.UUID) (*Session, error) {
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
func (s *sessStore) DeleteSessionInDB(id uuid.UUID) (*Session, error) {
	defer s.ClearExpiredSessions()

	_, err := s.db.Exec(context.Background(), "DELETE FROM sessions WHERE session_id = $1", id)
	if err != nil {
		return nil, err
	}
	return &Session{ID: id}, nil
}

// UpdateSessionInDB updates a session
func (s *sessStore) UpdateSessionInDB(session *Session) (*Session, error) {
	defer s.ClearExpiredSessions()

	_, err := s.db.Exec(context.Background(),
		"UPDATE sessions SET user_id = $2, permissions = $3, iat = $4, lua = $5, exp = $6 WHERE session_id = $1",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// Clear expired sessions
func (s *sessStore) ClearExpiredSessions() {
	_, err := s.db.Exec(context.Background(), "DELETE FROM sessions WHERE exp < $1 AND exp != 0", time.Now().Unix())
	if err != nil {
		log.Println("Unable to clear expired sessions:")
		log.Println(err)
	}
}

// -------------- Cache Functions --------------

// AddSessionToCache adds a session to the cache
func (s *sessStore) AddSessionToCache(session *Session) (*Session, error) {
	stringSession, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}

	_, err = s.rdb.Set(context.Background(), session.ID.String(), stringSession, time.Until(time.Unix(session.ExpiresAt, 0))).Result()
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetSessionFromCache gets a session from the cache
func (s *sessStore) GetSessionFromCache(id uuid.UUID) (*Session, error) {
	var session Session
	stringSession, err := s.rdb.Get(context.Background(), id.String()).Result()
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
func (s *sessStore) DeleteSessionFromCache(id uuid.UUID) (*Session, error) {
	_, err := s.rdb.Del(context.Background(), id.String()).Result()
	if err != nil {
		return nil, err
	}
	return &Session{ID: id}, nil
}
