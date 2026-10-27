package session

import (
	"context"
	"errors"
	"time"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
)

const sessionKeyPrefix = "session:"

// ErrNotFound is returned when the session being looked up is missing.
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

// Store - The session store interface
type Store interface {
	AddSessionToDB(ctx context.Context, session *Session) error
	GetSessionFromDB(ctx context.Context, id string) (*Session, error)
	UpdateSessionInDB(ctx context.Context, session *Session) error
	DeleteSessionInDB(ctx context.Context, id string) error
	AddSessionToCache(ctx context.Context, session *Session) error
	GetSessionFromCache(ctx context.Context, id string) (*Session, error)
	DeleteSessionFromCache(ctx context.Context, id string) error
}

// AddSessionToDB creates a session and inserts it into the database
func (s *store) AddSessionToDB(ctx context.Context, session *Session) error {
	defer s.ClearExpiredSessions(ctx)

	_, err := s.db.Exec(ctx,
		"INSERT INTO sessions (session_id, user_id, permissions, iat, lua, exp) VALUES ($1, $2, $3, $4, $5, $6)",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)
	if err != nil {
		return err
	}
	return nil
}

// GetSessionFromDB gets a session by ID
func (s *store) GetSessionFromDB(ctx context.Context, id string) (*Session, error) {
	defer s.ClearExpiredSessions(ctx)

	var session *Session
	rows, err := s.db.Query(ctx, "SELECT * FROM sessions WHERE session_id = $1", id)
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
func (s *store) DeleteSessionInDB(ctx context.Context, id string) error {
	defer s.ClearExpiredSessions(ctx)

	_, err := s.db.Exec(ctx, "DELETE FROM sessions WHERE session_id = $1", id)
	if err != nil {
		return err
	}
	return nil
}

// UpdateSessionInDB updates a session
func (s *store) UpdateSessionInDB(ctx context.Context, session *Session) error {
	defer s.ClearExpiredSessions(ctx)

	_, err := s.db.Exec(ctx,
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
func (s *store) ClearExpiredSessions(ctx context.Context) {
	_, err := s.db.Exec(ctx, "DELETE FROM sessions WHERE exp < $1 AND exp != 0", time.Now().Unix())
	if err != nil {
		reqctx.Log(ctx, "Unable to clear expired sessions:")
		reqctx.Log(ctx, err.Error())
	}
}

// -------------- Cache Functions --------------

// AddSessionToCache adds a session to the cache
func (s *store) AddSessionToCache(ctx context.Context, session *Session) error {
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

	_, err = s.rdb.Set(ctx, sessionKeyPrefix+session.ID, stringSession, ttl).Result()
	if err != nil {
		return err
	}
	return nil
}

// GetSessionFromCache gets a session from the cache
func (s *store) GetSessionFromCache(ctx context.Context, id string) (*Session, error) {
	var session Session
	stringSession, err := s.rdb.Get(ctx, sessionKeyPrefix+id).Result()
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
func (s *store) DeleteSessionFromCache(ctx context.Context, id string) error {
	_, err := s.rdb.Del(ctx, sessionKeyPrefix+id).Result()
	if err != nil {
		return err
	}
	return nil
}
