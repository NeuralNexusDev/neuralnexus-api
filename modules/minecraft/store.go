package minecraft

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
)

const (
	redisTTL           = 5 * time.Minute
	stalenessThreshold = 24 * time.Hour
)

const (
	CachePlayer        = "player:"
	CacheProfile       = CachePlayer + "profile:"
	CacheProfileSigned = CacheProfile + "signed:"
	S3KeyPrefix        = "texture/"

	JavaPlayerTexturesTable   = "player_textures"
	GeyserPlayerTexturesTable = "geyser_player_textures"
)

// Store - Minecraft player store
type Store interface {
	Cache
	S3
	GetPlayerByUUID(ctx context.Context, id string) (*Player, error)
	GetPlayerByName(ctx context.Context, name string) (*Player, error)
	GetProfileByUUID(ctx context.Context, id string) (*Profile, error)

	UpsertPlayer(ctx context.Context, player *Player, updateProfile bool) error

	GetTextures(ctx context.Context, table, id, name string) (*TexturesValue, error)
	UpsertTextures(ctx context.Context, table string, value *TexturesValue) error
	UpsertTextureHash(ctx context.Context, hash string) error

	GetGeyserPlayerByGamertag(ctx context.Context, gamertag string) (*GeyserPlayer, error)
	GetGeyserPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error)
	UpsertGeyserPlayer(ctx context.Context, player *GeyserPlayer) error
}

type Cache interface {
	GetPlayerFromCache(ctx context.Context, key string) (*Player, error)
	SetPlayerInCache(ctx context.Context, player *Player) error

	GetProfileFromCache(ctx context.Context, id string) (*Profile, error)
	SetProfileInCache(ctx context.Context, profile *Profile) error

	GetSignedProfileFromCache(ctx context.Context, id string) (*Player, error)
	SetSignedProfileInCache(ctx context.Context, player *Player) error
}

type S3 interface {
	IsTextureInS3(ctx context.Context, hash string) (bool, error)
	PutTextureInS3(ctx context.Context, hash string, body io.ReadCloser) error
}

// store - Minecraft player store implementation
type store struct {
	cache
	sThree
	cfg    config.Minecraft
	db     *pgxpool.Pool
	rdb    *redis.Client
	s3     *s3.Client
	bucket string
}

type cache struct {
	rdb *redis.Client
}

type sThree struct {
	s3     *s3.Client
	bucket string
}

// NewStore - Create a new Minecraft player store
func NewStore(cfg config.Minecraft, db *pgxpool.Pool, rdb *redis.Client, s3 *s3.Client, bucket string) Store {
	return &store{cfg: cfg, db: db, rdb: rdb, s3: s3, bucket: bucket}
}

// GetPlayerByUUID gets a player by UUID from the database
func (s *store) GetPlayerByUUID(ctx context.Context, id string) (*Player, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Player])
	if err != nil {
		return nil, playerNotFound(err)
	}
	return player, nil
}

// GetPlayerByName gets a player by name from the database
func (s *store) GetPlayerByName(ctx context.Context, name string) (*Player, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[Player])
	if err != nil {
		return nil, playerNotFound(err)
	}
	return player, nil
}

func playerNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPlayerNotFound
	}
	return err
}

// GetProfileByUUID gets a player's full profile from the database by UUID
func (s *store) GetProfileByUUID(ctx context.Context, id string) (*Profile, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[Player])
	if err != nil {
		return nil, playerNotFound(err)
	}

	textures, err := s.GetTextures(ctx, JavaPlayerTexturesTable, id, player.Name)
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

// GetTextures loads a player's most recently seen skin+cape from the
// database and decodes it into a TexturesValue, or nil if none is stored.
func (s *store) GetTextures(ctx context.Context, table, id, name string) (*TexturesValue, error) {
	rows, err := s.db.Query(ctx, `
		SELECT player_id, skin, model, cape, last_seen
		FROM `+table+
		` WHERE player_id = $1
		ORDER BY last_seen DESC
		LIMIT 1`, table, id)
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
	return row.Value(name, s.cfg.LookupTexture), nil
}

// UpsertPlayer upserts a player into the database and updates name history
func (s *store) UpsertPlayer(ctx context.Context, player *Player, updateProfile bool) error {
	now := time.Now().UnixMilli()

	var err error
	if updateProfile {
		_, err = s.db.Exec(ctx, `
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
		_, err = s.db.Exec(ctx, `
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
	_, err = s.db.Exec(ctx, `
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
func (s *store) UpsertTextures(ctx context.Context, table string, value *TexturesValue) error {
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
	_, err := s.db.Exec(ctx, `
		INSERT INTO $1 (player_id, skin, model, cape, first_seen, last_seen)
		VALUES ($2, $3, $4, $5, $6, $6)
		ON CONFLICT (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, '')) DO UPDATE SET
			last_seen = EXCLUDED.last_seen
		`, table, value.ProfileID, textureHash(skin), model, textureHash(cape), value.Timestamp)
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
func (s *store) UpsertTextureHash(ctx context.Context, hash string) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO textures (hash) VALUES ($1) ON CONFLICT (hash) DO NOTHING", hash)
	return err
}

func cacheMiss(err error) error {
	if errors.Is(err, redis.Nil) {
		return ErrCacheMiss
	}
	return err
}

// GetPlayerFromCache gets a player from the cache by key (uuid or name)
func (c *cache) GetPlayerFromCache(ctx context.Context, key string) (*Player, error) {
	val, err := c.rdb.Get(ctx, CachePlayer+key).Result()
	if err != nil {
		return nil, cacheMiss(err)
	}
	var player Player
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetPlayerInCache sets a player in the cache under both uuid and name keys
func (c *cache) SetPlayerInCache(ctx context.Context, player *Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return err
	}
	blob := string(data)

	if err := c.rdb.Set(ctx, CachePlayer+player.ID, blob, redisTTL).Err(); err != nil {
		return err
	}
	return c.rdb.Set(ctx, CachePlayer+player.Name, blob, redisTTL).Err()
}

// GetProfileFromCache gets a player's profile from the cache
func (c *cache) GetProfileFromCache(ctx context.Context, id string) (*Profile, error) {
	val, err := c.rdb.Get(ctx, CacheProfile+id).Result()
	if err != nil {
		return nil, cacheMiss(err)
	}
	var profile Profile
	if err := json.Unmarshal([]byte(val), &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// SetProfileInCache sets a player's profile in the cache
func (c *cache) SetProfileInCache(ctx context.Context, profile *Profile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, CacheProfile+profile.ID, string(data), redisTTL).Err()
}

// GetSignedProfileFromCache gets a player's signed profile from the cache
func (c *cache) GetSignedProfileFromCache(ctx context.Context, id string) (*Player, error) {
	val, err := c.rdb.Get(ctx, CacheProfileSigned+id).Result()
	if err != nil {
		return nil, cacheMiss(err)
	}
	var player Player
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetSignedProfileInCache sets a player's signed profile in the cache
func (c *cache) SetSignedProfileInCache(ctx context.Context, player *Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, CacheProfileSigned+player.ID, string(data), redisTTL).Err()
}

// GetGeyserPlayerByGamertag gets a Bedrock player's gamertag->XUID mapping.
func (s *store) GetGeyserPlayerByGamertag(ctx context.Context, gamertag string) (*GeyserPlayer, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, xuid, gamertag, first_seen, last_seen
		FROM geyser_players
		WHERE gamertag = $1
		ORDER BY last_seen DESC
		LIMIT 1`, gamertag)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[GeyserPlayer])
	if err != nil {
		return nil, playerNotFound(err)
	}
	return player, nil
}

// GetGeyserPlayerByXUID gets a Bedrock player's player info by XUID
func (s *store) GetGeyserPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, xuid, gamertag, first_seen, last_seen
		FROM geyser_players
		WHERE xuid = $1`, xuid)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[GeyserPlayer])
	if err != nil {
		return nil, playerNotFound(err)
	}
	player.ID = xuidToUUID(player.XUID)
	return player, nil
}

// UpsertGeyserPlayer upserts a Bedrock player's info
func (s *store) UpsertGeyserPlayer(ctx context.Context, player *GeyserPlayer) error {
	now := time.Now().UnixMilli()
	_, err := s.db.Exec(ctx, `
		INSERT INTO geyser_players (id, xuid, gamertag, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (xuid) DO UPDATE SET
			gamertag  = EXCLUDED.gamertag,
			last_seen = EXCLUDED.last_seen
		`,
		player.ID, player.XUID, player.Gamertag, now,
	)
	return err
}

// IsTextureInS3 check if the texture is in S3
func (s *sThree) IsTextureInS3(ctx context.Context, hash string) (bool, error) {
	_, err := s.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(S3KeyPrefix + hash),
	})
	if err != nil {
		var sue smithy.APIError
		if errors.As(err, &sue) {
			if sue.ErrorCode() == "NotFound" || sue.ErrorCode() == "NoSuchKey" {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

// PutTextureInS3 upload a texture to S3
func (s *sThree) PutTextureInS3(ctx context.Context, hash string, body io.ReadCloser) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(S3KeyPrefix + hash),
		Body:        body,
		ContentType: aws.String("image/png"),
	}
	if lr, ok := body.(interface{ Len() int }); ok {
		input.ContentLength = aws.Int64(int64(lr.Len()))
	}

	_, err := s.s3.PutObject(ctx, input)
	if err != nil {
		reqctx.Log(ctx, ErrUploadS3.Error(), err.Error())
		return ErrUploadS3
	}
	return nil
}
