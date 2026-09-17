package minecraft

import (
	"encoding/base64"
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"github.com/goccy/go-json"
)

var ErrPlayerNotFound = errors.New("player not found")

// ErrTextureNotFound - the requested texture does not exist upstream
var ErrTextureNotFound = errors.New("texture not found")

// TextureResult - the bytes and content type of fetched texture, ready to stream to a client
type TextureResult struct {
	Body        io.ReadCloser
	ContentType string
}

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

// MarshalJSON customizes Player's JSON output so ProfileActions is omitted
// only when it was never populated (nil) — a genuinely empty slice (fetched,
// zero actions) still serializes as []. Plain `omitempty` can't tell those
// apart since both have len == 0.
func (p *Player) MarshalJSON() ([]byte, error) {
	type Alias Player
	if p.ProfileActions == nil {
		return json.Marshal(struct {
			*Alias
			ProfileActions json.RawMessage `json:"profileActions,omitempty"`
		}{Alias: (*Alias)(p)})
	}
	return json.Marshal((*Alias)(p))
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

// ToProfile converts a Player fetched live from Mojang into our canonical,
// decoded shape for storage and caching.
func (p *Player) ToProfile() *Profile {
	return &Profile{
		ID:             p.ID,
		Name:           p.Name,
		Legacy:         p.Legacy,
		Demo:           p.Demo,
		ProfileActions: p.ProfileActions,
		Textures:       p.ParseProperties(),
	}
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

// withURL returns a copy of the Texture with its URL rewritten to
// baseURL+hash, or nil if t is nil.
func (t *Texture) withURL(baseURL string) *Texture {
	if t == nil {
		return nil
	}
	rewritten := *t
	rewritten.URL = baseURL + t.Hash()
	return &rewritten
}

// Metadata - Skin metadata (only present for Alex/slim model)
type Metadata struct {
	Model Model `json:"model"`
}

// Model type alias
type Model string

// SLIM The only known value for Metadata.Model
const SLIM Model = "slim"

// Profile - a player's full profile, decoded and canonical.
type Profile struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Legacy         bool           `json:"legacy,omitempty"`
	Demo           bool           `json:"demo,omitempty"`
	ProfileActions []string       `json:"profileActions,omitempty"`
	Textures       *TexturesValue `json:"textures,omitempty"`
	FirstSeen      int64          `json:"-"`
	LastSeen       int64          `json:"-"`
}

// IsStale returns true if the profile's last_seen is older than the staleness threshold
func (p *Profile) IsStale() bool {
	return time.Now().UnixMilli()-p.LastSeen > stalenessThreshold.Milliseconds()
}

// ToPlayer converts a Profile into Mojang's raw session-server mirror shape.
func (p *Profile) ToPlayer() (*Player, error) {
	prop, err := p.Textures.ToProperty()
	if err != nil {
		return nil, err
	}
	player := &Player{
		ID:             p.ID,
		Name:           p.Name,
		Legacy:         p.Legacy,
		Demo:           p.Demo,
		ProfileActions: p.ProfileActions,
	}
	if prop != nil {
		player.Properties = []Property{*prop}
	}
	return player, nil
}

// WithTextureURL returns a copy of the Profile with texture URLs rewritten to baseURL+hash.
func (p *Profile) WithTextureURL(baseURL string) *Profile {
	if p.Textures == nil {
		return p
	}
	rewritten := *p
	textures := *p.Textures
	textures.Textures.SKIN = textures.Textures.SKIN.withURL(baseURL)
	textures.Textures.CAPE = textures.Textures.CAPE.withURL(baseURL)
	rewritten.Textures = &textures
	return &rewritten
}

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
