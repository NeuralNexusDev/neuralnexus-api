package minecraft

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/goccy/go-json"
)

// Service - Minecraft player service
type Service interface {
	MojangService
	GeyserService

	GetProfile(ctx context.Context, id string) (*MojangProfile, error)
	GetProfileByName(ctx context.Context, name string) (*MojangProfile, error)

	GetGeyserProfile(ctx context.Context, id string) (*GeyserProfile, error)
	GetGeyserProfileByName(ctx context.Context, name string) (*GeyserProfile, error)

	GetTextureContent(ctx context.Context, hash string) (*TextureResult, error)
}

type MojangService interface {
	GetMojangPlayerByName(ctx context.Context, name string) (*MojangPlayer, error)
	GetMojangPlayerByUUID(ctx context.Context, id string) (*MojangPlayer, error)
	GetMojangPlayersByNames(ctx context.Context, names []string) ([]*MojangPlayer, error)
	GetMojangProfile(ctx context.Context, id string, signed bool) (*MojangPlayer, error)
}

type GeyserService interface {
	GetGeyserPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error)
	GetGeyserPlayerByGamertag(ctx context.Context, gamertag string) (*GeyserPlayer, error)
	GetGeyserPlayerByUUID(ctx context.Context, id string) (*GeyserPlayer, error)
	GetGeyserSkin(ctx context.Context, xuid int64) (*GeyserSkin, error)
}

type CommonService interface {
	GetProfile(ctx context.Context, id string) (*MojangProfile, error)
	GetProfileByName(ctx context.Context, name string) (*MojangProfile, error)
	GetTextureContent(ctx context.Context, hash string) (*TextureResult, error)
}

// service - Minecraft player service implementation
type service struct {
	mojangService
	geyserService
	textureService
}

type mojangService struct {
	cfg          config.Minecraft
	client       *http.Client
	s3TextureUrl string
	store        MojangStore
	texture      TextureStore
	cache        CacheStore
}

type geyserService struct {
	cfg     config.Minecraft
	client  *http.Client
	store   GeyserStore
	texture TextureStore
}

type textureService struct {
	cfg          config.Minecraft
	client       *http.Client
	s3TextureUrl string
	texture      TextureStore
	s3           S3Store
}

// NewService - Create a new Minecraft player service
func NewService(cfg config.Minecraft, store Store, client *http.Client, s3TextureUrl string) Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &service{
		mojangService: mojangService{
			cfg,
			client,
			s3TextureUrl,
			store.Mojang(),
			store.Texture(),
			store.Cache(),
		},
		geyserService: geyserService{
			cfg,
			client,
			store.Geyser(),
			store.Texture(),
		},
		textureService: textureService{
			cfg,
			client,
			s3TextureUrl,
			store.Texture(),
			store.S3(),
		},
	}
}

// get issues a GET that is cancelled with ctx.
func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// GetMojangPlayerByName gets a player by name, cache-first with Mojang fallback
func (m *mojangService) GetMojangPlayerByName(ctx context.Context, name string) (*MojangPlayer, error) {
	cached, err := m.cache.GetPlayerFromCache(ctx, name)
	if err == nil {
		return cached, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	// Cache miss — fetch from DB
	dbPlayer, _ := m.store.GetPlayerByName(ctx, name)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		if err := m.cache.SetPlayerInCache(context.WithoutCancel(ctx), dbPlayer); err != nil {
			return nil, err
		}
		return dbPlayer, nil
	}

	// Stale entry — fetch from Mojang
	resp, err := get(ctx, m.client, m.cfg.LookupByName+url.PathEscape(name))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPlayerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrMojangAPI.Error(), resp.Status)
		return nil, ErrMojangAPI
	}

	var player MojangPlayer
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, err
	}

	if err := m.store.UpsertPlayer(context.WithoutCancel(ctx), &player, false); err != nil {
		return nil, err
	}
	if err := m.cache.SetPlayerInCache(context.WithoutCancel(ctx), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// GetMojangPlayerByUUID gets a player by UUID, cache-first with Mojang fallback
func (m *mojangService) GetMojangPlayerByUUID(ctx context.Context, id string) (*MojangPlayer, error) {
	cached, err := m.cache.GetPlayerFromCache(ctx, id)
	if err == nil {
		return cached, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	// Cache miss — fetch from DB
	dbPlayer, _ := m.store.GetPlayerByUUID(ctx, id)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		if err := m.cache.SetPlayerInCache(context.WithoutCancel(ctx), dbPlayer); err != nil {
			return nil, err
		}
		return dbPlayer, nil
	}

	// Stale entry — fetch from Mojang
	resp, err := get(ctx, m.client, m.cfg.LookupByUUID+id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPlayerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrMojangAPI.Error(), resp.Status)
		return nil, ErrMojangAPI
	}

	var player MojangPlayer
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, err
	}

	if err := m.store.UpsertPlayer(context.WithoutCancel(ctx), &player, false); err != nil {
		return nil, err
	}
	if err := m.cache.SetPlayerInCache(context.WithoutCancel(ctx), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// GetMojangPlayersByNames gets players by name in batch, cache-first with Mojang fallback
// Mojang batch endpoint is capped at 10 names per request
func (m *mojangService) GetMojangPlayersByNames(ctx context.Context, names []string) ([]*MojangPlayer, error) {
	if len(names) == 0 {
		return nil, ErrNoNamesProvided
	}
	if len(names) > 10 {
		return nil, ErrBatchLimit
	}

	// Check cache first, collect misses
	players := make([]*MojangPlayer, 0, len(names))
	misses := make([]string, 0, len(names))

	for _, name := range names {
		player, err := m.cache.GetPlayerFromCache(ctx, name)
		if err == nil {
			players = append(players, player)
		} else {
			// Cache miss — fetch from DB
			player, _ := m.store.GetPlayerByName(ctx, name)
			if player != nil && !player.IsStale() {
				if err := m.cache.SetPlayerInCache(context.WithoutCancel(ctx), player); err != nil {
					return nil, err
				}
				players = append(players, player)
				continue
			}

			// Stale entry — fetch from Mojang
			misses = append(misses, name)
		}
	}

	if len(misses) == 0 {
		return players, nil
	}

	// Fetch stale entries from Mojang
	body, err := json.Marshal(misses)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.cfg.LookupBulk, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrMojangAPI.Error(), resp.Status)
		return nil, ErrMojangAPI
	}

	var fetched []MojangPlayer
	if err := json.NewDecoder(resp.Body).Decode(&fetched); err != nil {
		return nil, err
	}

	for i := range fetched {
		if err := m.store.UpsertPlayer(context.WithoutCancel(ctx), &fetched[i], false); err != nil {
			return nil, err
		}
		if err := m.cache.SetPlayerInCache(context.WithoutCancel(ctx), &fetched[i]); err != nil {
			return nil, err
		}
		players = append(players, &fetched[i])
	}

	return players, nil
}

// GetMojangProfile gets a full player profile
func (m *mojangService) GetMojangProfile(ctx context.Context, id string, signed bool) (*MojangPlayer, error) {
	if signed {
		cached, err := m.cache.GetSignedProfileFromCache(ctx, id)
		if err == nil {
			return cached, nil
		}
		if err != ErrCacheMiss {
			return nil, err
		}
		player, _, err := m.fetchProfileFromMojang(ctx, id, true)
		return player, err
	}

	profile, err := m.resolveProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	return profile.ToPlayer()
}

// GetProfile gets a player's profile with textures decoded as native JSON,
// with texture URLs pointing at our own CDN instead of Mojang's.
func (m *mojangService) GetProfile(ctx context.Context, id string) (*MojangProfile, error) {
	profile, err := m.resolveProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	if profile.Textures == nil {
		return profile, nil
	}
	if hash := profile.Textures.Textures.SKIN.Hash(); hash != "" {
		profile.Textures.Textures.SKIN.URL = m.s3TextureUrl + S3KeyPrefix + hash
	}
	if hash := profile.Textures.Textures.CAPE.Hash(); hash != "" {
		profile.Textures.Textures.CAPE.URL = m.s3TextureUrl + S3KeyPrefix + hash
	}
	return profile, nil
}

// GetProfileByName resolves a Java username to its profile.
func (m *mojangService) GetProfileByName(ctx context.Context, name string) (*MojangProfile, error) {
	player, err := m.GetMojangPlayerByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return m.GetProfile(ctx, player.ID)
}

// resolveProfile gets a player's canonical MojangProfile from cacheStore or the
// database, fetching live from Mojang when needed.
func (m *mojangService) resolveProfile(ctx context.Context, id string) (*MojangProfile, error) {
	cached, err := m.cache.GetProfileFromCache(ctx, id)
	if err == nil {
		return cached, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	dbProfile, err := m.store.GetProfileByUUID(ctx, id)
	if err != nil && err != ErrPlayerNotFound {
		reqctx.Log(ctx, "Failed to get profile from DB:\n\t", err.Error())
	}
	if dbProfile != nil {
		if dbProfile.ProfileActions == nil {
			dbProfile.ProfileActions = []string{}
		}
		if !dbProfile.IsStale() {
			if err := m.cache.SetProfileInCache(context.WithoutCancel(ctx), dbProfile); err != nil {
				return nil, err
			}
			return dbProfile, nil
		}
	}

	_, profile, err := m.fetchProfileFromMojang(ctx, id, false)
	return profile, err
}

// fetchProfileFromMojang fetches and persists a player's profile live from
// Mojang, returning both the raw MojangPlayer and the decoded MojangProfile.
func (m *mojangService) fetchProfileFromMojang(ctx context.Context, id string, signed bool) (*MojangPlayer, *MojangProfile, error) {
	u := m.cfg.LookupProfile + id
	if signed {
		u += "?unsigned=false"
	}
	resp, err := get(ctx, m.client, u)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil, ErrPlayerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrMojangAPI.Error(), resp.Status)
		return nil, nil, ErrMojangAPI
	}

	var player MojangPlayer
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, nil, err
	}
	if player.ProfileActions == nil {
		player.ProfileActions = []string{}
	}

	if err := m.store.UpsertPlayer(context.WithoutCancel(ctx), &player, true); err != nil {
		return nil, nil, err
	}

	// Extract and store textures
	profile := player.ToProfile(ctx)
	if profile.Textures != nil {
		if hash := profile.Textures.Textures.SKIN.Hash(); hash != "" {
			if err := m.texture.UpsertTextureHash(context.WithoutCancel(ctx), hash); err != nil {
				reqctx.Log(ctx, "Failed to store skin hash:\n\t", err.Error())
			}
		}

		if hash := profile.Textures.Textures.CAPE.Hash(); hash != "" {
			if err := m.texture.UpsertTextureHash(context.WithoutCancel(ctx), hash); err != nil {
				reqctx.Log(ctx, "Failed to store cape hash:\n\t", err.Error())
			}
		}

		if err := m.texture.UpsertTextures(context.WithoutCancel(ctx), JavaPlayerTexturesTable, profile.Textures); err != nil {
			reqctx.Log(ctx, "Failed to store texture:\n\t", err.Error())
		}
	}

	if signed {
		if err := m.cache.SetSignedProfileInCache(context.WithoutCancel(ctx), &player); err != nil {
			return nil, nil, err
		}
	} else {
		if err := m.cache.SetProfileInCache(context.WithoutCancel(ctx), profile); err != nil {
			return nil, nil, err
		}
	}

	return &player, profile, nil
}

// GetGeyserPlayerByXUID looks up a Bedrock player's by Xbox XUID.
func (g *geyserService) GetGeyserPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error) {
	dbPlayer, _ := g.store.GetGeyserPlayerByXUID(ctx, xuid)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		return dbPlayer, nil
	}

	result, err := g.queryGeyserAPI[GeyserGamertag](ctx, g.cfg.GeyserGamertagLookup, strconv.FormatInt(xuid, 10))
	if err != nil {
		return nil, err
	}
	if result.Gamertag == "" {
		return nil, ErrPlayerNotFound
	}

	player := &GeyserPlayer{
		ID:   xuidToUUID(xuid),
		XUID: xuid,
		Name: result.Gamertag,
	}

	if err := g.store.UpsertGeyserPlayer(context.WithoutCancel(ctx), player); err != nil {
		return nil, err
	}
	return player, nil
}

// GetGeyserPlayerByGamertag looks up a Bedrock player by gamertag.
func (g *geyserService) GetGeyserPlayerByGamertag(ctx context.Context, gamertag string) (*GeyserPlayer, error) {
	dbPlayer, _ := g.store.GetGeyserPlayerByGamertag(ctx, gamertag)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		return dbPlayer, nil
	}

	result, err := g.queryGeyserAPI[GeyserXUID](ctx, g.cfg.GeyserXUIDLookup, url.PathEscape(gamertag))
	if err != nil {
		return nil, err
	}
	if result.XUID == 0 {
		return nil, ErrPlayerNotFound
	}

	player := &GeyserPlayer{
		ID:   xuidToUUID(result.XUID),
		Name: gamertag,
		XUID: result.XUID,
	}

	if err := g.store.UpsertGeyserPlayer(context.WithoutCancel(ctx), player); err != nil {
		return nil, err
	}
	return player, nil
}

// GetGeyserPlayerByUUID looks up a Bedrock player by Floodgate uuid.
func (g *geyserService) GetGeyserPlayerByUUID(ctx context.Context, id string) (*GeyserPlayer, error) {
	xuid, err := uuidToXUID(id)
	if err != nil {
		return nil, err
	}
	dbPlayer, _ := g.store.GetGeyserPlayerByUUID(ctx, id)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		return dbPlayer, nil
	}
	return g.GetGeyserPlayerByXUID(ctx, xuid)
}

func (g *geyserService) queryGeyserAPI[T any](ctx context.Context, uri string, value string) (*T, error) {
	resp, err := get(ctx, g.client, uri+value)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidGeyserRequest
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
		reqctx.Log(ctx, ErrGeyserAPI.Error(), resp.Status)
		return nil, ErrGeyserAPI
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetGeyserSkin gets a Bedrock player's most recently converted skin by XUID.
func (g *geyserService) GetGeyserSkin(ctx context.Context, xuid int64) (*GeyserSkin, error) {
	player, err := g.GetGeyserPlayerByXUID(ctx, xuid)
	if err != nil {
		return nil, err
	}

	var textures *TexturesValue
	if textures, err = g.texture.GetTextures(ctx, GeyserPlayerTexturesTable, player.ID, player.Name); err != nil {
		return nil, err
	}

	var skin GeyserSkin
	if !player.IsStale() && textures != nil {
		err := skin.FromTexturesValue(ctx, textures)
		return &skin, err
	}

	resp, err := get(ctx, g.client, g.cfg.GeyserSkinLookup+strconv.FormatInt(xuid, 10))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidGeyserRequest
	}
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrGeyserAPI.Error(), resp.Status)
		return nil, ErrGeyserAPI
	}

	if err := json.NewDecoder(resp.Body).Decode(&skin); err != nil {
		return nil, err
	}
	// An unconverted (or nonexistent) player comes back as 200 with an empty object.
	if skin.Hash == "" {
		return nil, ErrSkinNotFound
	}

	textures = ParseTextureValue(ctx, skin.Value)
	if textures != nil {
		if err := g.texture.UpsertTextures(context.WithoutCancel(ctx), GeyserPlayerTexturesTable, textures); err != nil {
			return nil, err
		}
	}
	return &skin, nil
}

// skinStepFailure reports a skin-lookup failure for a xuid that was already
// resolved. It must not wrap ErrInvalidGeyserRequest, which the handlers map to 400.
func skinStepFailure(ctx context.Context, err error) error {
	if err == ErrInvalidGeyserRequest {
		reqctx.Log(ctx, ErrGeyserAPI.Error(), "- skin lookup rejected a resolved xuid")
		return ErrGeyserAPI
	}
	return err
}

func (g *geyserService) getGeyserProfileFromPlayer(ctx context.Context, player *GeyserPlayer) (*GeyserProfile, error) {
	skin, err := g.GetGeyserSkin(ctx, player.XUID)
	if err != nil {
		if err != ErrSkinNotFound {
			return nil, skinStepFailure(ctx, err)
		}
		return nil, err
	}
	textures := ParseTextureValue(ctx, skin.Value)
	if textures != nil {
		if err := g.texture.UpsertTextures(context.WithoutCancel(ctx), GeyserPlayerTexturesTable, textures); err != nil {
			return nil, err
		}
	}
	return &GeyserProfile{
		ID:       player.ID,
		XUID:     player.XUID,
		Name:     player.Name,
		Textures: textures,
	}, nil
}

// GetGeyserProfile resolves a Bedrock player's full profile by Floodgate UUID
func (s *service) GetGeyserProfile(ctx context.Context, id string) (*GeyserProfile, error) {
	player, err := s.GetGeyserPlayerByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.getGeyserProfileFromPlayer(ctx, player)
}

// GetGeyserProfileByXUID resolves a Bedrock player's full profile by XUID
func (s *service) GetGeyserProfileByXUID(ctx context.Context, xuid int64) (*GeyserProfile, error) {
	player, err := s.GetGeyserPlayerByXUID(ctx, xuid)
	if err != nil {
		return nil, err
	}
	return s.getGeyserProfileFromPlayer(ctx, player)
}

// GetGeyserProfileByName resolves a Bedrock player's full profile by gamertag.
func (s *service) GetGeyserProfileByName(ctx context.Context, gamertag string) (*GeyserProfile, error) {
	player, err := s.GetGeyserPlayerByGamertag(ctx, gamertag)
	if err != nil {
		return nil, err
	}
	return s.getGeyserProfileFromPlayer(ctx, player)
}

// bytesReadCloser adapts a *bytes.Reader to io.ReadCloser while keeping Len()
// visible, so the store can set an explicit Content-Length instead of the
// SDK re-buffering to compute it.
type bytesReadCloser struct {
	*bytes.Reader
}

func (bytesReadCloser) Close() error { return nil }

// GetTextureContent returns the texture's bytes and content type, fetching from
// Mojang exactly once on a cache miss instead of round-tripping back through S3.
func (t *textureService) GetTextureContent(ctx context.Context, hash string) (*TextureResult, error) {
	present, err := t.s3.IsTextureInS3(ctx, hash)
	if err != nil {
		return nil, err
	}

	if present {
		resp, err := get(ctx, t.client, t.s3TextureUrl+S3KeyPrefix+url.PathEscape(hash))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, ErrTextureNotFound
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			reqctx.Log(ctx, ErrBadStatusS3.Error(), strconv.Itoa(resp.StatusCode))
			return nil, ErrBadStatusS3
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/png"
		}
		return &TextureResult{Body: resp.Body, ContentType: contentType}, nil
	}
	resp, err := get(ctx, t.client, t.cfg.LookupTexture+url.PathEscape(hash))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrTextureNotFound
	}
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrBadStatusRemote.Error(), strconv.Itoa(resp.StatusCode))
		return nil, ErrBadStatusRemote
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	if err := t.s3.PutTextureInS3(context.WithoutCancel(ctx), hash, bytesReadCloser{bytes.NewReader(data)}); err != nil {
		reqctx.Log(ctx, "Failed to upload texture to S3:\n\t", err.Error())
	} else if err := t.texture.UpsertTextureHash(context.WithoutCancel(ctx), hash); err != nil {
		reqctx.Log(ctx, "Failed to store texture hash:\n\t", err.Error())
	}

	return &TextureResult{Body: io.NopCloser(bytes.NewReader(data)), ContentType: contentType}, nil
}
