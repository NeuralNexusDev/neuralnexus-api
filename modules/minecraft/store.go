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
	GetPlayerByUUID(id string) (*MCPlayer, error)
	GetPlayerByName(name string) (*MCPlayer, error)
	UpsertPlayer(player *MCPlayer) error
	GetPlayerFromCache(key string) (*MCPlayer, error)
	SetPlayerInCache(player *MCPlayer) error
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
func (s *store) GetPlayerByUUID(id string) (*MCPlayer, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[MCPlayer])
	if err != nil {
		return nil, err
	}
	return player, nil
}

// GetPlayerByName gets a player by name from the database
func (s *store) GetPlayerByName(name string) (*MCPlayer, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[MCPlayer])
	if err != nil {
		return nil, err
	}
	return player, nil
}

// UpsertPlayer upserts a player into the database and updates name history
func (s *store) UpsertPlayer(player *MCPlayer) error {
	now := time.Now().UnixMilli()

	_, err := s.db.Exec(context.Background(), `
		INSERT INTO players (id, name, legacy, demo, profile_actions, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (id) DO UPDATE SET
			name           = EXCLUDED.name,
			legacy         = EXCLUDED.legacy,
			demo           = EXCLUDED.demo,
			profile_actions = EXCLUDED.profile_actions,
			last_seen      = EXCLUDED.last_seen
		`,
		player.ID, player.Name, player.Legacy, player.Demo, player.ProfileActions, now,
	)
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

// GetPlayerFromCache gets a player from the cache by key (uuid or name)
func (s *store) GetPlayerFromCache(key string) (*MCPlayer, error) {
	val, err := s.rdb.Get(context.Background(), "player:"+key).Result()
	if err != nil {
		return nil, err
	}
	var player MCPlayer
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetPlayerInCache sets a player in the cache under both uuid and name keys
func (s *store) SetPlayerInCache(player *MCPlayer) error {
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
