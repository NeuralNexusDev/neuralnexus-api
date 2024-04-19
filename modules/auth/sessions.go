package auth

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CREATE TABLE sessions (
// 	session_id UUID PRIMARY KEY NOT NULL,
// 	user_id UUID NOT NULL,
// 	permissions TEXT[] NOT NULL,
// 	iat BIGINT NOT NULL,
// 	lua BIGINT NOT NULL,
// 	exp BIGINT NOT NULL
// );

// -------------- Structs --------------

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

// -------------- DB Functions --------------

// AddSessionToDB creates a session and inserts it into the database
func AddSessionToDB(session *Session) (*Session, error) {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(),
		"INSERT INTO sessions (session_id, user_id, permissions, iat, lua, exp) VALUES ($1, $2, $3, $4, $5, $6)",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)

	defer ClearExpiredSessions()

	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetSessionFromDB gets a session by ID
func GetSessionFromDB(id uuid.UUID) (*Session, error) {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	var session *Session
	rows, err := db.Query(context.Background(), "SELECT * FROM sessions WHERE session_id = $1", id)
	if err != nil {
		return nil, err
	}

	defer ClearExpiredSessions()

	session, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Session])
	if err != nil {
		return nil, err
	}
	return session, nil
}

// DeleteSessionInDB deletes a session by ID
func DeleteSessionInDB(id uuid.UUID) (*Session, error) {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(), "DELETE FROM sessions WHERE session_id = $1", id)

	defer ClearExpiredSessions()

	if err != nil {
		return nil, err
	}
	return &Session{ID: id}, nil
}

// UpdateSessionInDB updates a session
func UpdateSessionInDB(session *Session) (*Session, error) {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(),
		"UPDATE sessions SET user_id = $2, permissions = $3, iat = $4, lua = $5, exp = $6 WHERE session_id = $1",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)

	defer ClearExpiredSessions()

	if err != nil {
		return nil, err
	}
	return session, nil
}

// Clear expired sessions
func ClearExpiredSessions() {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(), "DELETE FROM sessions WHERE exp < $1 AND exp != 0", time.Now().Unix())
	if err != nil {
		log.Println("Unable to clear expired sessions:")
		log.Println(err)
	}
}

// -------------- Cache Functions --------------

// AddSessionToCache adds a session to the cache
func AddSessionToCache(session *Session) (*Session, error) {
	rdb := database.GetRedis()
	defer rdb.Close()

	stringSession, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}

	_, err = rdb.Set(context.Background(), session.ID.String(), stringSession, time.Until(time.Unix(session.ExpiresAt, 0))).Result()
	if err != nil {
		return nil, err
	}
	return session, nil
}

// GetSessionFromCache gets a session from the cache
func GetSessionFromCache(id uuid.UUID) (*Session, error) {
	rdb := database.GetRedis()
	defer rdb.Close()

	var session Session
	stringSession, err := rdb.Get(context.Background(), id.String()).Result()
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
func DeleteSessionFromCache(id uuid.UUID) (*Session, error) {
	rdb := database.GetRedis()
	defer rdb.Close()

	_, err := rdb.Del(context.Background(), id.String()).Result()
	if err != nil {
		return nil, err
	}
	return &Session{ID: id}, nil
}

// -------------- Functions --------------

// GetSession gets a session by ID
func GetSession(id uuid.UUID) (*Session, error) {
	session, err := GetSessionFromCache(id)
	if err != nil {
		session, err = GetSessionFromDB(id)
		if err != nil {
			return nil, err
		}
		AddSessionToCache(session)
	}
	return session, nil
}

// UpdateSession updates a session
func UpdateSession(session *Session) (*Session, error) {
	session, err := UpdateSessionInDB(session)
	if err != nil {
		return nil, err
	}
	if session.ID != uuid.Nil {
		AddSessionToCache(session)
	}
	return session, nil
}

// DeleteSession deletes a session by ID
func DeleteSession(id uuid.UUID) (*Session, error) {
	session, err := DeleteSessionInDB(id)
	if err != nil {
		return nil, err
	}
	if session.ID != uuid.Nil {
		DeleteSessionFromCache(id)
	}
	return session, nil
}
