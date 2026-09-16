package minecraft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const (
	redisTTL           = 5 * time.Minute
	stalenessThreshold = 24 * time.Hour
)

const (
	CachePlayer             = "player:"
	CachePropertiesSigned   = CachePlayer + "properties:signed:"
	CachePropertiesUnsigned = CachePlayer + "properties:unsigned:"
	S3KeyPrefix             = "texture/"
)

// Store - Minecraft player store
type Store interface {
	GetPlayerByUUID(id string) (*Player, error)
	GetPlayerByName(name string) (*Player, error)
	GetProfileByUUID(id string) (*Player, error)
	GetTextures(id string) (*TexturesRow, error)

	UpsertPlayer(player *Player, updateProfile bool) error
	UpsertTextures(value *TexturesValue) error
	UpsertTextureHash(hash string) error

	GetPlayerFromCache(key string) (*Player, error)
	SetPlayerInCache(player *Player) error

	GetProfileFromCache(id string, signed bool) (*Player, error)
	SetProfileInCache(player *Player, signed bool) error

	IsTextureInS3(hash string) (bool, error)
	PutTextureInS3(hash string, body io.ReadCloser) error
}

// store - Minecraft player store implementation
type store struct {
	db  *pgxpool.Pool
	rdb *redis.Client
	s3  *s3.Client
}

// NewStore - Create a new Minecraft player store
func NewStore(db *pgxpool.Pool, rdb *redis.Client, s3 *s3.Client) Store {
	return &store{db: db, rdb: rdb, s3: s3}
}

// GetPlayerByUUID gets a player by UUID from the database
func (s *store) GetPlayerByUUID(id string) (*Player, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
}

// GetPlayerByName gets a player by name from the database
func (s *store) GetPlayerByName(name string) (*Player, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
}

// GetProfileByUUID gets a player's full profile from the database by UUID
func (s *store) GetProfileByUUID(id string) (*Player, error) {
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

// GetTextures get a player's most recent skin+cape from the database
func (s *store) GetTextures(id string) (*TexturesRow, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT player_id, skin, model, cape, last_seen
		FROM player_textures
		WHERE player_id = $1
		ORDER BY last_seen DESC
		LIMIT 1`, id)
	if err != nil {
		return nil, err
	}

	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[TexturesRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
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
	if skin != nil && skin.Metadata != nil {
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
func (s *store) UpsertTextureHash(hash string) error {
	_, err := s.db.Exec(context.Background(),
		"INSERT INTO textures (hash) VALUES ($1) ON CONFLICT (hash) DO NOTHING", hash)
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

	key := CachePropertiesUnsigned + id
	if signed {
		key = CachePropertiesSigned + id
	}
	val, err := s.rdb.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	var properties []Property
	if err := json.Unmarshal([]byte(val), &properties); err != nil {
		return nil, err
	}

	player.Properties = properties
	return player, nil
}

// SetProfileInCache sets a player profile in the cache
func (s *store) SetProfileInCache(player *Player, signed bool) error {
	data, err := json.Marshal(player.Properties)
	if err != nil {
		return err
	}

	key := CachePropertiesUnsigned + player.ID
	if signed {
		key = CachePropertiesSigned + player.ID
	}
	return s.rdb.Set(context.Background(), key, string(data), redisTTL).Err()
}

// IsTextureInS3 check if the texture is in S3
func (s *store) IsTextureInS3(hash string) (bool, error) {
	_, err := s.s3.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String("mca"),
		Key:    aws.String(S3KeyPrefix + hash),
	})

	if err != nil {
		var sue smithy.APIError
		if errors.As(err, &sue) {
			// Check for standard NotFound or NoSuchKey codes
			if sue.ErrorCode() == "NotFound" || sue.ErrorCode() == "NoSuchKey" {
				return false, nil
			}
		}
		// Some S3-compatible stores (or older configs) might return a 404 generic error
		// You can also check standard HTTP status code if needed, but the above is standard.
		return false, err
	}

	return true, nil
}

// PutTextureInS3 upload a texture to S3
func (s *store) PutTextureInS3(hash string, body io.ReadCloser) error {
	_, err := s.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String("mca"),
		Key:         aws.String(S3KeyPrefix + hash),
		Body:        body,
		ContentType: aws.String("image/png"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to s3: %w", err)
	}
	return nil
}
