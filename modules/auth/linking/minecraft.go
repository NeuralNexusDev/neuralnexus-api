package linking

import (
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
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

// GetID returns the platform ID
func (m *MinecraftData) ID() string {
	return m.ID.String()
}

// GetUsername returns the platform username
func (m *MinecraftData) Username() string {
	return m.Username
}

// GetData returns the platform data
func (m *MinecraftData) JSONData() string {
	data, _ := json.Marshal(m)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (m *MinecraftData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformMinecraft, m.Username, m.ID.String(), m)
}
