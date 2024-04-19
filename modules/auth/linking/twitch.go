package accountlinking

import (
	"encoding/json"

	"github.com/google/uuid"
)

// -------------- Structs --------------

// TwitchData struct
type TwitchData struct {
	ID              string `json:"id" validate:"required"`
	Login           string `json:"login" validate:"required"`
	DisplayName     string `json:"display_name" validate:"required"`
	Type            string `json:"type,omitempty"`
	BroadcasterType string `json:"broadcaster_type,omitempty"`
	Description     string `json:"description,omitempty"`
	ProfileImageURL string `json:"profile_image_url,omitempty"`
	OfflineImageURL string `json:"offline_image_url,omitempty"`
	ViewCount       int    `json:"view_count,omitempty"`
	Email           string `json:"email,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
}

// PlatformID returns the platform ID
func (t TwitchData) PlatformID() string {
	return t.ID
}

// PlatformUsername returns the platform username
func (t TwitchData) PlatformUsername() string {
	return t.Login
}

// PlatformData returns the platform data
func (t TwitchData) PlatformData() string {
	data, _ := json.Marshal(t)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (t TwitchData) CreateLinkedAccount(userID uuid.UUID) *LinkedAccount {
	return NewLinkedAccount(userID, PlatformTwitch, t.Login, t.ID, t)
}
