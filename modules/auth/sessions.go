package auth

import (
	"context"
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
func (a *Account) NewSession(expiresAt int64) Session {
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

	return Session{
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

// -------------- Functions --------------

// CreateSession creates a session and inserts it into the database
func CreateSession(session Session) database.Response[Session] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(),
		"INSERT INTO sessions (session_id, user_id, permissions, iat, lua, exp) VALUES ($1, $2, $3, $4, $5, $6)",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)
	if err != nil {
		return database.Response[Session]{
			Success: false,
			Message: "Unable to insert session",
		}
	}

	return database.Response[Session]{
		Success: true,
		Data:    session,
	}
}

// GetSession gets a session by ID
func GetSession(id uuid.UUID) database.Response[Session] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	var session *Session
	rows, err := db.Query(context.Background(),
		"SELECT session_id, user_id, permissions, iat, lua, exp FROM sessions WHERE session_id = $1",
		id,
	)
	if err != nil {
		log.Println(err)
		return database.Response[Session]{
			Success: false,
			Message: "Unable to retreive session",
		}
	}

	session, err = pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Session])
	if err != nil {
		log.Println(err)
		return database.Response[Session]{
			Success: false,
			Message: "Unable to retreive session",
		}
	}

	return database.Response[Session]{
		Success: true,
		Data:    *session,
	}
}

// DeleteSession deletes a session by ID
func DeleteSession(id uuid.UUID) database.Response[Session] {
	db := database.GetDB("nedatabaseuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(),
		"DELETE FROM sessions WHERE session_id = $1",
		id,
	)
	if err != nil {
		return database.Response[Session]{
			Success: false,
			Message: "Unable to delete session",
		}
	}

	return database.Response[Session]{
		Success: true,
	}
}

// UpdateSession updates a session
func UpdateSession(session Session) database.Response[Session] {
	db := database.GetDB("neuralnexus")
	_, err := db.Exec(context.Background(),
		"UPDATE sessions SET user_id = $2, permissions = $3, iat = $4, lua = $5, exp = $6 WHERE session_id = $1",
		session.ID, session.UserID, session.Permissions, session.IssuedAt, session.LastUsedAt, session.ExpiresAt,
	)
	if err != nil {
		return database.Response[Session]{
			Success: false,
			Message: "Unable to update session",
		}
	}

	return database.Response[Session]{
		Success: true,
		Data:    session,
	}
}
