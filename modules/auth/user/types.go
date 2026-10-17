// Package user is the API-facing user: what the users routes return and what
// an admin can change, with no email, hashed secret or salt on either side.
package user

import "time"

// User is the API-facing view of an account.
type User struct {
	UserID    string    `json:"user_id" xml:"user_id"`
	Username  string    `json:"username" xml:"username"`
	Roles     []string  `json:"roles" xml:"roles"`
	UpdatedAt time.Time `json:"updated_at" xml:"updated_at"`
}

// UserUpdate holds the fields an admin can set on a user through the API.
type UserUpdate struct {
	Username string   `json:"username" xml:"username"`
	Roles    []string `json:"roles" xml:"roles"`
}
