package minecraft

import "errors"

var ErrPlayerNotFound = errors.New("player not found")

// MCPlayer - Minecraft player profile as returned by Mojang
type MCPlayer struct {
	ID             string             `json:"id"                       db:"id"`
	Name           string             `json:"name"                     db:"name"`
	Legacy         bool               `json:"legacy,omitempty"         db:"legacy"`
	Demo           bool               `json:"demo,omitempty"           db:"demo"`
	Properties     []MCPlayerProperty `json:"properties,omitempty"     db:"-"`
	ProfileActions []string           `json:"profileActions,omitempty" db:"profile_actions"`
	FirstSeen      int64              `json:"-"                        db:"first_seen"`
	LastSeen       int64              `json:"-"                        db:"last_seen"`
}

// MCPlayerProperty - A player property as returned by Mojang
type MCPlayerProperty struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Signature string `json:"signature,omitempty"`
}

// MCPlayerTextures - Decoded textures property
type MCPlayerTextures struct {
	Timestamp         int64                      `json:"timestamp"`
	ProfileID         string                     `json:"profileId"`
	ProfileName       string                     `json:"profileName"`
	SignatureRequired bool                       `json:"signatureRequired,omitempty"`
	Textures          map[string]MCPlayerTexture `json:"textures"`
}

// MCPlayerTexture - A single texture entry (SKIN or CAPE)
type MCPlayerTexture struct {
	URL      string                   `json:"url"`
	Metadata *MCPlayerTextureMetadata `json:"metadata,omitempty"`
}

// MCPlayerTextureMetadata - Skin metadata (only present for Alex/slim model)
type MCPlayerTextureMetadata struct {
	Model string `json:"model"`
}
