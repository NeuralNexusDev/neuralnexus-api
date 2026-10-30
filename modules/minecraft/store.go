package minecraft

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
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
	S3KeyPrefix        = "texture/"

	JavaPlayerNamesTable      = "player_names"
	JavaPlayerTexturesTable   = "player_textures"
	GeyserPlayerNamesTable    = "geyser_player_names"
	GeyserPlayerTexturesTable = "geyser_player_textures"
)

// Store - Minecraft player store
type Store interface {
	Texture() TextureStore
	Cache() CacheStore
	S3() S3Store
	Mojang() MojangStore
	Geyser() GeyserStore
}

type TextureStore interface {
	GetTextures(ctx context.Context, table, id, name string) (*TexturesValue, error)
	UpsertTextures(ctx context.Context, table string, value *TexturesValue) error
	UpsertTextureHash(ctx context.Context, hash string) error
}

type CacheStore interface {
	GetPlayerFromCache(ctx context.Context, key string) (*MojangPlayer, error)
	SetPlayerInCache(ctx context.Context, player *MojangPlayer) error

	GetProfileFromCache(ctx context.Context, id string) (*MojangProfile, error)
	SetProfileInCache(ctx context.Context, profile *MojangProfile) error

	GetSignedProfileFromCache(ctx context.Context, id string) (*MojangPlayer, error)
	SetSignedProfileInCache(ctx context.Context, player *MojangPlayer) error
}

type S3Store interface {
	IsTextureInS3(ctx context.Context, hash string) (bool, error)
	PutTextureInS3(ctx context.Context, hash string, body io.ReadCloser) error
}

type MojangStore interface {
	GetPlayerByUUID(ctx context.Context, id string) (*MojangPlayer, error)
	GetPlayerByName(ctx context.Context, name string) (*MojangPlayer, error)
	UpsertPlayer(ctx context.Context, player *MojangPlayer, updateProfile bool) error
	GetProfileByUUID(ctx context.Context, id string) (*MojangProfile, error)
}

type GeyserStore interface {
	GetGeyserPlayerByUUID(ctx context.Context, floodgateUUID string) (*GeyserPlayer, error)
	GetGeyserPlayerByGamertag(ctx context.Context, gamertag string) (*GeyserPlayer, error)
	GetGeyserPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error)
	UpsertGeyserPlayer(ctx context.Context, player *GeyserPlayer) error
}

// store - Minecraft player store implementation
type store struct {
	texture *textureStore
	cache   *cacheStore
	sThree  *s3store
	mojang  *mojangStore
	geyser  *geyserStore
}

func (s *store) Texture() TextureStore {
	return s.texture
}

func (s *store) Cache() CacheStore {
	return s.cache
}

func (s *store) S3() S3Store {
	return s.sThree
}

func (s *store) Mojang() MojangStore {
	return s.mojang
}

func (s *store) Geyser() GeyserStore {
	return s.geyser
}

type cacheStore struct {
	rdb *redis.Client
}

type s3store struct {
	s3     *s3.Client
	bucket string
}

type textureStore struct {
	cfg config.Minecraft
	db  *pgxpool.Pool
}

type mojangStore struct {
	cfg     config.Minecraft
	db      *pgxpool.Pool
	texture *textureStore
	cache   *cacheStore
}

type geyserStore struct {
	cfg     config.Minecraft
	db      *pgxpool.Pool
	texture TextureStore
}

// NewStore - Create a new Minecraft player store
func NewStore(cfg config.Minecraft, db *pgxpool.Pool, rdb *redis.Client, s3 *s3.Client, bucket string) Store {
	texture := &textureStore{cfg, db}
	cache := &cacheStore{rdb}
	return &store{
		texture,
		cache,
		&s3store{s3, bucket},
		&mojangStore{
			cfg:     cfg,
			db:      db,
			texture: texture,
			cache:   cache,
		},
		&geyserStore{
			cfg:     cfg,
			db:      db,
			texture: texture,
		},
	}
}

// GetPlayerByUUID gets a player by UUID from the database
func (m *mojangStore) GetPlayerByUUID(ctx context.Context, id string) (*MojangPlayer, error) {
	rows, err := m.db.Query(ctx,
		"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[MojangPlayer])
	if err != nil {
		return nil, playerNotFound(err)
	}
	return player, nil
}

// GetPlayerByName gets a player by name from the database
func (m *mojangStore) GetPlayerByName(ctx context.Context, name string) (*MojangPlayer, error) {
	rows, err := m.db.Query(ctx,
		"SELECT id, name, legacy, demo, first_seen, last_seen FROM players WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByNameLax[MojangPlayer])
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
func (m *mojangStore) GetProfileByUUID(ctx context.Context, id string) (*MojangProfile, error) {
	rows, err := m.db.Query(ctx,
		"SELECT id, name, legacy, demo, profile_actions, first_seen, last_seen FROM players WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[MojangPlayer])
	if err != nil {
		return nil, playerNotFound(err)
	}

	textures, err := m.texture.GetTextures(ctx, JavaPlayerTexturesTable, id, player.Name)
	if err != nil {
		return nil, err
	}

	return &MojangProfile{
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

func upsertNameHistory(ctx context.Context, db *pgxpool.Pool, table, uuid, name string, now int64) error {
	_, err := db.Exec(ctx, `
		INSERT INTO `+table+` (player_id, name, first_seen, last_seen)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (player_id, name) DO UPDATE SET
			last_seen = EXCLUDED.last_seen
		`,
		uuid, name, now,
	)
	return err
}

// UpsertPlayer upserts a player into the database and updates name history
func (m *mojangStore) UpsertPlayer(ctx context.Context, player *MojangPlayer, updateProfile bool) error {
	now := time.Now().UnixMilli()

	var err error
	if updateProfile {
		_, err = m.db.Exec(ctx, `
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
		_, err = m.db.Exec(ctx, `
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
	return upsertNameHistory(ctx, m.db, JavaPlayerNamesTable, player.ID, player.Name, now)
}

// GetGeyserPlayerByUUID gets a Bedrock player from their floodgateUUID
func (g *geyserStore) GetGeyserPlayerByUUID(ctx context.Context, floodgateUUID string) (*GeyserPlayer, error) {
	rows, err := g.db.Query(ctx, `
		SELECT id, xuid, gamertag, first_seen, last_seen
		FROM geyser_players
		WHERE id = $1
		ORDER BY last_seen DESC
		LIMIT 1`, floodgateUUID)
	if err != nil {
		return nil, err
	}
	player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[GeyserPlayer])
	if err != nil {
		return nil, playerNotFound(err)
	}
	return player, nil
}

// GetGeyserPlayerByGamertag gets a Bedrock player from their gamertag
func (g *geyserStore) GetGeyserPlayerByGamertag(ctx context.Context, gamertag string) (*GeyserPlayer, error) {
	rows, err := g.db.Query(ctx, `
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

// GetGeyserPlayerByXUID gets a Bedrock player from their xuid
func (g *geyserStore) GetGeyserPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error) {
	rows, err := g.db.Query(ctx, `
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
func (g *geyserStore) UpsertGeyserPlayer(ctx context.Context, player *GeyserPlayer) error {
	now := time.Now().UnixMilli()
	_, err := g.db.Exec(ctx, `
		INSERT INTO geyser_players (id, xuid, gamertag, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (xuid) DO UPDATE SET
			gamertag  = EXCLUDED.gamertag,
			last_seen = EXCLUDED.last_seen
		`,
		player.ID, player.XUID, player.Name, now,
	)
	if err != nil {
		return err
	}
	return upsertNameHistory(ctx, g.db, GeyserPlayerNamesTable, player.ID, player.Name, now)
}

// GetTextures loads a player's most recently seen skin+cape from the
// database and decodes it into a TexturesValue, or nil if none is stored.
func (t *textureStore) GetTextures(ctx context.Context, table, id, name string) (*TexturesValue, error) {
	rows, err := t.db.Query(ctx, `
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
	return row.Value(name, t.cfg.LookupTexture), nil
}

// UpsertTextures upserts a player's skin and cape into the database
func (t *textureStore) UpsertTextures(ctx context.Context, table string, value *TexturesValue) error {
	skin := value.Textures.SKIN
	var model *Model
	if skin != nil && skin.Metadata != nil {
		model = &skin.Metadata.Model
	}
	cape := value.Textures.CAPE

	_, err := t.db.Exec(ctx, `
		INSERT INTO `+table+` (player_id, skin, model, cape, first_seen, last_seen)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (player_id, COALESCE(skin, ''), COALESCE(model, ''), COALESCE(cape, '')) DO UPDATE SET
			last_seen = EXCLUDED.last_seen
		`, value.ProfileID, skin.Hash(), model, cape.Hash(), value.Timestamp)
	return err
}

// UpsertTextureHash upserts a texture hash into the database
func (t *textureStore) UpsertTextureHash(ctx context.Context, hash string) error {
	_, err := t.db.Exec(ctx,
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
func (c *cacheStore) GetPlayerFromCache(ctx context.Context, key string) (*MojangPlayer, error) {
	val, err := c.rdb.Get(ctx, CachePlayer+key).Result()
	if err != nil {
		return nil, cacheMiss(err)
	}
	var player MojangPlayer
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetPlayerInCache sets a player in the cache under both uuid and name keys
func (c *cacheStore) SetPlayerInCache(ctx context.Context, player *MojangPlayer) error {
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
func (c *cacheStore) GetProfileFromCache(ctx context.Context, id string) (*MojangProfile, error) {
	val, err := c.rdb.Get(ctx, CacheProfile+id).Result()
	if err != nil {
		return nil, cacheMiss(err)
	}
	var profile MojangProfile
	if err := json.Unmarshal([]byte(val), &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// SetProfileInCache sets a player's profile in the cache
func (c *cacheStore) SetProfileInCache(ctx context.Context, profile *MojangProfile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, CacheProfile+profile.ID, string(data), redisTTL).Err()
}

// GetSignedProfileFromCache gets a player's signed profile from the cache
func (c *cacheStore) GetSignedProfileFromCache(ctx context.Context, id string) (*MojangPlayer, error) {
	val, err := c.rdb.Get(ctx, CacheProfileSigned+id).Result()
	if err != nil {
		return nil, cacheMiss(err)
	}
	var player MojangPlayer
	if err := json.Unmarshal([]byte(val), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// SetSignedProfileInCache sets a player's signed profile in the cache
func (c *cacheStore) SetSignedProfileInCache(ctx context.Context, player *MojangPlayer) error {
	data, err := json.Marshal(player)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, CacheProfileSigned+player.ID, string(data), redisTTL).Err()
}

// IsTextureInS3 check if the texture is in S3
func (s *s3store) IsTextureInS3(ctx context.Context, hash string) (bool, error) {
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
func (s *s3store) PutTextureInS3(ctx context.Context, hash string, body io.ReadCloser) error {
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
