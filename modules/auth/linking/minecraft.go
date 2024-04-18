package accountlinking

import (
	"encoding/json"

	"github.com/google/uuid"
)

// -------------- Structs --------------

// MinecraftData struct
type MinecraftData struct {
	ID       uuid.UUID `json:"id" validate:"required"`
	Username string    `json:"username" validate:"required"`
	Skins    []Skin    `json:"skins" validate:"required"`
	Capes    []Cape    `json:"capes" validate:"required"`
}

// Skin struct
type Skin struct {
	ID      uuid.UUID `json:"id" validate:"required"`
	State   string    `json:"state" validate:"required"`
	URL     string    `json:"url" validate:"required"`
	Variant string    `json:"variant" validate:"required"`
	Alias   string    `json:"alias" validate:"required"`
}

// Cape struct
type Cape struct{}

// PlatformID returns the platform ID
func (m MinecraftData) PlatformID() string {
	return m.ID.String()
}

// PlatformUsername returns the platform username
func (m MinecraftData) PlatformUsername() string {
	return m.Username
}

// PlatformData returns the platform data
func (m MinecraftData) PlatformData() string {
	data, _ := json.Marshal(m)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (m MinecraftData) CreateLinkedAccount(userID uuid.UUID) LinkedAccount {
	return NewLinkedAccount(userID, PlatformMinecraft, m.Username, m.ID.String(), m)
}
