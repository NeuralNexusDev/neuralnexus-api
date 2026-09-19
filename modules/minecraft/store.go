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
	CachePlayer        = "player:"
	CacheProfile       = CachePlayer + "profile:"
	CacheProfileSigned = CacheProfile + "signed:"
	S3Bucket           = "mca"
	S3KeyPrefix        = "texture/"
)

// Store - Minecraft player store
type Store interface {
	GetPlayerByUUID(id string) (*Player, error)
	GetPlayerByName(name string) (*Player, error)
	GetProfileByUUID(id string) (*Profile, error)

	UpsertPlayer(player *Player, updateProfile bool) error
	UpsertTextures(value *TexturesValue) error
	UpsertTextureHash(hash string) error

	GetPlayerFromCache(key string) (*Player, error)
	SetPlayerInCache(player *Player) error

	GetProfileFromCache(id string) (*Profile, error)
	SetProfileInCache(profile *Profile) error

	GetSignedProfileFromCache(id string) (*Player, error)
	SetSignedProfileInCache(player *Player) error

	IsTextureInS3(hash string) (bool, error)
	PutTextureInS3(hash string, body io.ReadCloser) error

	GetGeyserPlayerByGamertag(gamertag string) (*GeyserPlayer, error)
	GetGeyserPlayerByXUID(xuid int64) (*GeyserPlayer, error)
	UpsertGeyserPlayer(player *GeyserPlayer) error

	GetGeyserSkin(xuid int64) (*GeyserSkin, error)
	UpsertGeyserSkin(xuid int64, skin *GeyserSkin) error
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
	// Lax: this query intentionally omits profile_actions, unlike
	// GetProfileByUUID, so Player.ProfileActions is left at its zero value.
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Player])
}

// GetPlayerByName gets a player by name from the database
func (s *store) GetPlayerByName(name string) (*Player, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	// Lax: see GetPlayerByUUID.
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Player])
}

// GetProfileByUUID gets a player's full profile from the database by UUID
func (s *store) GetProfileByUUID(id string) (*Profile, error) {
	rows, err := s.db.Query(context.Background(),
		"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
	if err != nil {
		return nil, err
	}

	textures, err := s.getTextures(id, player.Name)
	if err != nil {
		return nil, err
	}

	return &Profile{
		ID:             player.ID,
		Name:           player.Name,
		Legacy:         player.Legacy,
		Demo:           player.Demo,
		ProfileActions: player.ProfileActions,
		Textures:       textures,
		FirstSeen:      player.FirstSeen,
		LastSeen:       player.LastSeen,
	}, nil
}

// getTextures loads a player's most recently seen skin+cape from the
// database and decodes it into a TexturesValue, or nil if none is stored.
func (s *store) getTextures(id, name string) (*TexturesValue, error) {
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
	return row.Value(name, mojangTextureURL), nil
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
			VALUES ($1, $2, false, false, '[]', $3, $3)
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

	// player_textures_unique is an expression index (COALESCE(skin/model/cape, ''))
	// rather than a plain-column constraint, so it can't be targeted by name
	// via ON CONFLICT ON CONSTRAINT — the conflict target must repeat the
	// same expressions instead.
	_, err := s.db.Exec(context.Background(), `
		INSERT INTO player_textures (player_id, skin, model, cape, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, '')) DO UPDATE SET
			last_seen = EXCLUDED.last_seen
		`, value.ProfileID, textureHash(skin), model, textureHash(cape), value.Timestamp)
	return err
}

// textureHash returns t's hash, or nil when absent or unparsable — an
// empty string is never a valid skin/cape reference (see the
// player_textures_skin_not_empty / _cape_not_empty DB constraints).
func textureHash(t *Texture) *string {
	hash := t.Hash()
	if hash == "" {
		return nil
	}
	return &hash
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

// GetProfileFromCache gets a player's profile from the cache
func (s *store) GetProfileFromCache(id string) (*Profile, error) {
	val, err := s.rdb.Get(context.Background(), CacheProfile+id).Result()
	if err != nil {
		return nil, err
	}
	var profile Profile
	if err := json.Unmarshal([]byte(val), &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// SetProfileInCache sets a player's profile in the cache
func (s *store) SetProfileInCache(profile *Profile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return s.rdb.Set(context.Background(), CacheProfile+profile.ID, string(data), redisTTL).Err()
}

// GetSignedProfileFromCache gets a player's signed profile from the cache
func (s *store) GetSignedProfileFromCache(id string) (*Player, error) {
	val, err := s.rdb.Get(context.Background(), CacheProfileSigned+id).Result()
	if err != nil {
		return nil, err
	}
	var player Player
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetSignedProfileInCache sets a player's signed profile in the cache
func (s *store) SetSignedProfileInCache(player *Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return err
	}
	return s.rdb.Set(context.Background(), CacheProfileSigned+player.ID, string(data), redisTTL).Err()
}

// IsTextureInS3 check if the texture is in S3
func (s *store) IsTextureInS3(hash string) (bool, error) {
	_, err := s.s3.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(S3Bucket),
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

// GetGeyserPlayerByGamertag gets a Bedrock player's gamertag->XUID mapping.
// Gamertags aren't unique long-term (a released one can be reused), so
// ORDER BY + LIMIT 1 picks the most recently seen match deterministically.
func (s *store) GetGeyserPlayerByGamertag(gamertag string) (*GeyserPlayer, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT xuid, gamertag, first_seen, last_seen
		FROM geyser_players
		WHERE gamertag = $1
		ORDER BY last_seen DESC
		LIMIT 1`, gamertag)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[GeyserPlayer])
	if err != nil {
		return nil, err
	}
	player.UUID = xuidToUUID(player.XUID)
	return player, nil
}

// GetGeyserPlayerByXUID gets a Bedrock player's gamertag->XUID mapping by its
// stable key (xuid is the table's primary key, so no reuse-collision handling needed).
func (s *store) GetGeyserPlayerByXUID(xuid int64) (*GeyserPlayer, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT xuid, gamertag, first_seen, last_seen
		FROM geyser_players
		WHERE xuid = $1`, xuid)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[GeyserPlayer])
	if err != nil {
		return nil, err
	}
	player.UUID = xuidToUUID(player.XUID)
	return player, nil
}

// UpsertGeyserPlayer upserts a Bedrock player's gamertag->XUID mapping into the database
func (s *store) UpsertGeyserPlayer(player *GeyserPlayer) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(context.Background(), `
		INSERT INTO geyser_players (xuid, gamertag, first_seen, last_seen)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (xuid) DO UPDATE SET
			gamertag  = EXCLUDED.gamertag,
			last_seen = EXCLUDED.last_seen
		`,
		player.XUID, player.Gamertag, now,
	)
	return err
}

// GetGeyserSkin gets a Bedrock player's most recently seen converted skin from the database
func (s *store) GetGeyserSkin(xuid int64) (*GeyserSkin, error) {
	rows, err := s.db.Query(context.Background(), `
		SELECT hash, is_steve, COALESCE(signature, '') AS signature, texture_id, value, first_seen, last_seen
		FROM geyser_player_textures
		WHERE xuid = $1
		ORDER BY last_seen DESC
		LIMIT 1`, xuid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[GeyserSkin])
}

// UpsertGeyserSkin upserts a Bedrock player's converted skin into the database
func (s *store) UpsertGeyserSkin(xuid int64, skin *GeyserSkin) error {
	now := time.Now().UnixMilli()
	var signature *string
	if skin.Signature != "" {
		signature = &skin.Signature
	}
	_, err := s.db.Exec(context.Background(), `
		INSERT INTO geyser_player_textures (xuid, hash, is_steve, signature, texture_id, value, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (xuid, hash) DO UPDATE SET
			is_steve   = EXCLUDED.is_steve,
			signature  = EXCLUDED.signature,
			texture_id = EXCLUDED.texture_id,
			value      = EXCLUDED.value,
			last_seen  = EXCLUDED.last_seen
		`,
		xuid, skin.Hash, skin.IsSteve, signature, skin.TextureID, skin.Value, now,
	)
	return err
}

// PutTextureInS3 upload a texture to S3
func (s *store) PutTextureInS3(hash string, body io.ReadCloser) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(S3Bucket),
		Key:         aws.String(S3KeyPrefix + hash),
		Body:        body,
		ContentType: aws.String("image/png"),
	}
	if lr, ok := body.(interface{ Len() int }); ok {
		input.ContentLength = aws.Int64(int64(lr.Len()))
	}

	_, err := s.s3.PutObject(context.Background(), input)
	if err != nil {
		return fmt.Errorf("failed to upload to s3: %w", err)
	}
	return nil
}
