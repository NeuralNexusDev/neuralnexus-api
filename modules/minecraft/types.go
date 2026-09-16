package minecraft

import (
	"encoding/base64"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/goccy/go-json"
)

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

// ParseProperties parse the player's properties
func (p *Player) ParseProperties() *TexturesValue {
	for _, prop := range p.Properties {
		if prop.Name != TEXTURES {
			log.Println("Unknown property:\n\t" + prop.String())
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(prop.Value)
		if err != nil {
			log.Println("Failed to base64 decode texture property:\n\t", err)
			continue
		}
		var textures TexturesValue
		if err := json.Unmarshal(decoded, &textures); err != nil {
			log.Println("Failed to unmarshal texture property:\n\t", err)
			continue
		}
		return &textures
	}
	return nil
}

// IsStale returns true if the player's last_seen is older than the staleness threshold
func (p *Player) IsStale() bool {
	return time.Now().UnixMilli()-p.LastSeen > stalenessThreshold.Milliseconds()
}

// Property - A player property as returned by Mojang
type Property struct {
	Name      PropertyName `json:"name"`
	Value     string       `json:"value"`
	Signature string       `json:"signature,omitempty"`
}

func (p *Property) String() string {
	str := "{ Name: " + string(p.Name) + ", Value: " + p.Value
	if p.Signature != "" {
		str += ", " + p.Signature
	}
	str += " }"
	return str
}

// PropertyName type alias
type PropertyName string

// TEXTURES the only known in-use value returned from the Mojang API
const TEXTURES PropertyName = "textures"

// TexturesValue - Decoded textures property
type TexturesValue struct {
	Timestamp         int64    `json:"timestamp"`
	ProfileID         string   `json:"profileId"`
	ProfileName       string   `json:"profileName"`
	SignatureRequired bool     `json:"signatureRequired,omitempty"`
	Textures          Textures `json:"textures"`
}

// ToProperty Converts a TextureValue to base64'd JSON and stores it in a TEXTURES property
func (t *TexturesValue) ToProperty() (*Property, error) {
	if t == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(*t)
	if err != nil {
		return nil, err
	}
	return &Property{Name: TEXTURES, Value: base64.StdEncoding.EncodeToString(encoded)}, nil
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

// Hash extracts the texture hash from a Texture.URL
// e.g. http://textures.minecraft.net/texture/<hash> -> <hash>
func (t *Texture) Hash() string {
	if t == nil {
		return ""
	}
	idx := strings.LastIndex(t.URL, "/")
	if idx == -1 || idx == len(t.URL)-1 {
		return ""
	}
	return t.URL[idx+1:]
}

// Metadata - Skin metadata (only present for Alex/slim model)
type Metadata struct {
	Model Model `json:"model"`
}

// Model type alias
type Model string

// SLIM The only known value for Metadata.Model
const SLIM Model = "slim"

// TexturesRow represents a texture in the database
type TexturesRow struct {
	PlayerId string  `db:"player_id"`
	Skin     *string `db:"skin"`
	Model    *Model  `db:"model"`
	Cape     *string `db:"cape"`
	LastSeen int64   `db:"last_seen"`
}

// Value converts a TexturesRow to a TexturesValue
func (t *TexturesRow) Value(playerName, textureUrl string) *TexturesValue {
	if t == nil || (t.Skin == nil && t.Cape == nil) {
		return nil
	}

	var textures Textures
	if t.Skin != nil {
		textures.SKIN = &Texture{URL: textureUrl + *t.Skin}
		if t.Model != nil && *t.Model == SLIM {
			textures.SKIN.Metadata = &Metadata{Model: SLIM}
		}
	}
	if t.Cape != nil {
		textures.CAPE = &Texture{URL: textureUrl + *t.Cape}
	}

	return &TexturesValue{
		Timestamp:   t.LastSeen,
		ProfileID:   t.PlayerId,
		ProfileName: playerName,
		Textures:    textures,
	}
}
