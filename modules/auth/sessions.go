package auth

import (
	"context"
	"log"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/google/uuid"
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
	ID          uuid.UUID `json:"session_id"`  // Session ID
	UserID      uuid.UUID `json:"user_id"`     // User ID
	Permissions []string  `json:"permissions"` // Permissions -- Roles squashed into an array
	IssuedAt    int64     `json:"iat"`         // Created at
	LastUsedAt  int64     `json:"lua"`         // Last used at
	ExpiresAt   int64     `json:"exp"`         // Expires at -- set to 0 for no expiration
}

// NewSession creates a new session
func (a *Account) NewSession(expiresAt int64) Session {
	permissions := []string{}
	for _, r := range a.Roles {
		role, err := GetRoleByName(r)
		if err != nil {
			continue
		}
		permissions = append(permissions, role.Permissions...)
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
func (s *Session) HasPermission(permission string) bool {
	for _, p := range s.Permissions {
		if p == permission {
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
	var session Session
	err := db.QueryRow(context.Background(),
		"SELECT session_id, user_id, permissions, iat, lua, exp FROM sessions WHERE session_id = $1",
		id,
	).Scan(&session.ID, &session.UserID, &session.Permissions, &session.IssuedAt, &session.LastUsedAt, &session.ExpiresAt)
	if err != nil {
		log.Println(err)
		return database.Response[Session]{
			Success: false,
			Message: "Unable to retreive session",
		}
	}

	return database.Response[Session]{
		Success: true,
		Data:    session,
	}
}

// DeleteSession deletes a session by ID
func DeleteSession(id uuid.UUID) database.Response[Session] {
	db := database.GetDB("nedatabaseuralnexus")
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
