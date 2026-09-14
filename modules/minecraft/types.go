package minecraft

import "errors"

var ErrPlayerNotFound = errors.New("player not found")

// Player - Minecraft player profile as returned by Mojang
type Player struct {
	ID             string     `json:"id"                   db:"id"`
	Name           string     `json:"name"                 db:"name"`
	Legacy         bool       `json:"legacy,omitempty"     db:"legacy"`
	Demo           bool       `json:"demo,omitempty"       db:"demo"`
	Properties     []Property `json:"properties,omitempty" db:"-"`
	ProfileActions []string   `json:"profileActions"       db:"profile_actions"`
	FirstSeen      int64      `json:"-"                    db:"first_seen"`
	LastSeen       int64      `json:"-"                    db:"last_seen"`
}

// Property - A player property as returned by Mojang
type Property struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

// TexturesValue - Decoded textures property
type TexturesValue struct {
	Timestamp         int64     `json:"timestamp"`
	ProfileID         string    `json:"profileId"`
	ProfileName       string    `json:"profileName"`
	SignatureRequired bool      `json:"signatureRequired,omitempty"`
	Textures          *Textures `json:"textures"`
}

// Textures - The player's textures
type Textures struct {
	SKIN *Texture `json:"SKIN,omitempty"`
	CAPE *Texture `json:"CAPE,omitempty"`
}

// Texture - A single texture entry (SKIN or CAPE)
type Texture struct {
	URL      string    `json:"url"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

// Metadata - Skin metadata (only present for Alex/slim model)
type Metadata struct {
	Model string `json:"model"`
}
