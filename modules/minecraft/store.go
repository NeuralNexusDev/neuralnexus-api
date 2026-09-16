package minecraft

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	redisTTL           = 5 * time.Minute
	stalenessThreshold = 24 * time.Hour
)

// mojangTextureURL is prefixed onto a stored texture hash to reconstruct the
// URL shape Mojang returns, since only the hash is persisted in player_textures.
const mojangTextureURL = "https://textures.minecraft.net/texture/"

const (
	CachePlayer             = "player:"
	CachePropertiesSigned   = CachePlayer + "properties:signed:"
	CachePropertiesUnsigned = CachePlayer + "properties:unsigned:"
)

// Store - Minecraft player store
type Store interface {
	GetPlayerByUUID(id string, includeProfile bool) (*Player, error)
	GetPlayerByName(name string, includeProfile bool) (*Player, error)

	UpsertPlayer(player *Player, updateProfile bool) error
	UpsertTextures(value *TexturesValue) error
	UpsertTextureHash(hash *Texture) error

	GetPlayerFromCache(key string) (*Player, error)
	SetPlayerInCache(player *Player) error

	GetProfileFromCache(id string, signed bool) (*Player, error)
	SetProfileInCache(player *Player, signed bool) error
}

// store - Minecraft player store implementation
type store struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

// NewStore - Create a new Minecraft player store
func NewStore(db *pgxpool.Pool, rdb *redis.Client) Store {
	return &store{db: db, rdb: rdb}
}

// GetPlayerByUUID gets a player by UUID from the database
func (s *store) GetPlayerByUUID(id string, includeProfile bool) (*Player, error) {
	var rows pgx.Rows
	var err error
	if includeProfile {
		rows, err = s.db.Query(context.Background(),
			"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE id = $1", id)
	} else {
		rows, err = s.db.Query(context.Background(),
			"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE id = $1", id)
	}
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
	if err != nil {
		return nil, err
	}
	if includeProfile {
		if err := s.hydrateTextureProperties(player); err != nil {
			return nil, err
		}
	}
	return player, nil
}

// GetPlayerByName gets a player by name from the database
func (s *store) GetPlayerByName(name string, includeProfile bool) (*Player, error) {
	var rows pgx.Rows
	var err error
	if includeProfile {
		rows, err = s.db.Query(context.Background(),
			"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE name = $1", name)
	} else {
		rows, err = s.db.Query(context.Background(),
			"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE name = $1", name)
	}
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
	if err != nil {
		return nil, err
	}
	if includeProfile {
		if err := s.hydrateTextureProperties(player); err != nil {
			return nil, err
		}
	}
	return player, nil
}

// hydrateTextureProperties loads the player's most recent skin/cape hashes from
// player_textures and reconstructs a TEXTURES property matching the shape a live
// Mojang response would have, so archived profiles work with ParseProperties and
// SetProfileInCache the same as freshly-fetched ones. No-op if none are stored.
func (s *store) hydrateTextureProperties(player *Player) error {
	rows, err := s.db.Query(context.Background(), `
		SELECT skin, model, cape, last_seen
		FROM player_textures
		WHERE player_id = $1
		ORDER BY last_seen DESC
		LIMIT 1
		`, player.ID)
	if err != nil {
		return err
	}

	type textureRow struct {
		Skin     *string
		Model    *string
		Cape     *string
		LastSeen int64
	}
	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByPos[textureRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if row.Skin == nil && row.Cape == nil {
		return nil
	}

	var textures Textures
	if row.Skin != nil {
		textures.SKIN = &Texture{URL: mojangTextureURL + *row.Skin}
		if row.Model != nil && Model(*row.Model) == SLIM {
			textures.SKIN.Metadata = &Metadata{Model: SLIM}
		}
	}
	if row.Cape != nil {
		textures.CAPE = &Texture{URL: mojangTextureURL + *row.Cape}
	}

	value := TexturesValue{
		Timestamp:   row.LastSeen,
		ProfileID:   player.ID,
		ProfileName: player.Name,
		Textures:    textures,
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	player.Properties = []Property{
		{Name: TEXTURES, Value: base64.StdEncoding.EncodeToString(encoded)},
	}
	return nil
}

// UpsertPlayer upserts a player into the database and updates name history
func (s *store) UpsertPlayer(player *Player, updateProfile bool) error {
	now := time.Now().UnixMilli()

	var err error
	if updateProfile {
		_, err = s.db.Exec(context.Background(), `
			INSERT INTO players (id, name, legacy, demo, profile_actions, first_seen, last_seen)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			ON CONFLICT (id) DO UPDATE SET
				name            = EXCLUDED.name,
				legacy          = EXCLUDED.legacy,
				demo            = EXCLUDED.demo,
				profile_actions = EXCLUDED.profile_actions,
				last_seen       = EXCLUDED.last_seen
			`,
			player.ID, player.Name, player.Legacy, player.Demo, player.ProfileActions, now,
		)
	} else {
		_, err = s.db.Exec(context.Background(), `
			INSERT INTO players (id, name, legacy, demo, profile_actions, first_seen, last_seen)
			VALUES ($1, $2, false, false, '{}', $3, $3)
			ON CONFLICT (id) DO UPDATE SET
				name      = EXCLUDED.name,
				last_seen = EXCLUDED.last_seen
			`,
			player.ID, player.Name, now,
		)
	}
	if err != nil {
		return err
	}

	// Upsert name history
	_, err = s.db.Exec(context.Background(), `
		INSERT INTO player_names (player_id, name, first_seen, last_seen)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (player_id, name) DO UPDATE SET
			last_seen = EXCLUDED.last_seen
		`,
		player.ID, player.Name, now,
	)
	return err
}

// UpsertTextures upserts a player's skin and cape into the database
func (s *store) UpsertTextures(value *TexturesValue) error {
	skin := value.Textures.SKIN
	var model *Model
	if skin.Metadata != nil {
		model = &skin.Metadata.Model
	}
	cape := value.Textures.CAPE

	_, err := s.db.Exec(context.Background(), `
		INSERT INTO player_textures (player_id, skin, model, cape, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT ON CONSTRAINT player_textures_unique DO UPDATE SET
			last_seen = EXCLUDED.last_seen
		`, value.ProfileID, skin.Hash(), model, cape.Hash(), value.Timestamp)
	return err
}

// UpsertTextureHash upserts a texture hash into the database
func (s *store) UpsertTextureHash(texture *Texture) error {
	_, err := s.db.Exec(context.Background(),
		"INSERT INTO textures (hash) VALUES ($1) ON CONFLICT (hash) DO NOTHING", texture.Hash())
	return err
}

// GetPlayerFromCache gets a player from the cache by key (uuid or name)
func (s *store) GetPlayerFromCache(key string) (*Player, error) {
	val, err := s.rdb.Get(context.Background(), CachePlayer+key).Result()
	if err != nil {
		return nil, err
	}
	var player Player
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetPlayerInCache sets a player in the cache under both uuid and name keys
func (s *store) SetPlayerInCache(player *Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return err
	}
	blob := string(data)

	if err := s.rdb.Set(context.Background(), CachePlayer+player.ID, blob, redisTTL).Err(); err != nil {
		return err
	}
	return s.rdb.Set(context.Background(), CachePlayer+player.Name, blob, redisTTL).Err()
}

// GetProfileFromCache gets a player profile from the cache
func (s *store) GetProfileFromCache(id string, signed bool) (*Player, error) {
	player, err := s.GetPlayerFromCache(id)
	if err != nil {
		return nil, err
	}

	val, err := s.rdb.Get(context.Background(), CachePropertiesUnsigned+id).Result()
	if err != nil {
		return nil, err
	}
	var properties []Property
	if err := json.Unmarshal([]byte(val), &properties); err != nil {
		return nil, err
	}

	var signatures []PropertySignature
	if signed {
		sigVal, err := s.rdb.Get(context.Background(), CachePropertiesSigned+id).Result()
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(sigVal), &signatures); err != nil {
			return nil, err
		}
	}

	for i, prop := range properties {
		if prop.Name != TEXTURES {
			continue
		}
		if signed {
			for _, sig := range signatures {
				if sig.Name == prop.Name {
					properties[i].Signature = sig.Signature
					break
				}
			}
			decoded, err := base64.StdEncoding.DecodeString(prop.Value)
			if err != nil {
				return nil, err
			}
			var textures TexturesValue
			if err := json.Unmarshal(decoded, &textures); err != nil {
				return nil, err
			}
			textures.SignatureRequired = true
			reEncoded, err := json.Marshal(textures)
			if err != nil {
				return nil, err
			}
			properties[i].Value = base64.StdEncoding.EncodeToString(reEncoded)
		}
	}

	player.Properties = properties
	return player, nil
}

// SetProfileInCache sets a player profile in the cache
func (s *store) SetProfileInCache(player *Player, signed bool) error {
	properties := make([]Property, len(player.Properties))
	signatures := make([]PropertySignature, 0)

	for i, prop := range player.Properties {
		properties[i] = Property{Name: prop.Name, Value: prop.Value}

		if signed {
			if prop.Name == TEXTURES {
				decoded, err := base64.StdEncoding.DecodeString(prop.Value)
				if err != nil {
					return err
				}
				var textures TexturesValue
				if err := json.Unmarshal(decoded, &textures); err != nil {
					return err
				}
				textures.SignatureRequired = false
				reEncoded, err := json.Marshal(textures)
				if err != nil {
					return err
				}
				properties[i].Value = base64.StdEncoding.EncodeToString(reEncoded)
			}

			if prop.Signature != "" {
				signatures = append(signatures, PropertySignature{
					Name:      prop.Name,
					Signature: prop.Signature,
				})
			}
		}
	}

	propsData, err := json.Marshal(properties)
	if err != nil {
		return err
	}
	if err := s.rdb.Set(context.Background(), CachePropertiesUnsigned+player.ID, string(propsData), redisTTL).Err(); err != nil {
		return err
	}

	if signed && len(signatures) > 0 {
		sigData, err := json.Marshal(signatures)
		if err != nil {
			return err
		}
		if err := s.rdb.Set(context.Background(), CachePropertiesSigned+player.ID, string(sigData), redisTTL).Err(); err != nil {
			return err
		}
	}

	return nil
}

// IsStale returns true if the player's last_seen is older than the staleness threshold
func (p *Player) IsStale() bool {
	return time.Now().UnixMilli()-p.LastSeen > stalenessThreshold.Milliseconds()
}
