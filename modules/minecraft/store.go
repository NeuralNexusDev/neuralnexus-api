package minecraft

import (
	"context"
	"time"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const cacheTTL = 24 * time.Hour

// Store - Minecraft player store
type Store interface {
	GetPlayerByUUID(id string) (*Player, error)
	GetPlayerByName(name string) (*Player, error)

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
func (s *store) GetPlayerByUUID(id string) (*Player, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
	if err != nil {
		return nil, err
	}
	return player, nil
}

// GetPlayerByName gets a player by name from the database
func (s *store) GetPlayerByName(name string) (*Player, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
	if err != nil {
		return nil, err
	}
	return player, nil
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
	val, err := s.rdb.Get(context.Background(), "player:"+key).Result()
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

	if err := s.rdb.Set(context.Background(), "player:"+player.ID, blob, cacheTTL).Err(); err != nil {
		return err
	}
	return s.rdb.Set(context.Background(), "player:"+player.Name, blob, cacheTTL).Err()
}

// GetProfileFromCache gets a player profile from the cache
func (s *store) GetProfileFromCache(id string, signed bool) (*Player, error) {
	key := "player:profile:unsigned:" + id
	if signed {
		key = "player:profile:signed:" + id
	}
	val, err := s.rdb.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	var player Player
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetProfileInCache sets a player profile in the cache
func (s *store) SetProfileInCache(player *Player, signed bool) error {
	key := "player:profile:unsigned:" + player.ID
	if signed {
		key = "player:profile:signed:" + player.ID
	}
	data, err := json.Marshal(player)
	if err != nil {
		return err
	}
	return s.rdb.Set(context.Background(), key, string(data), cacheTTL).Err()
}
