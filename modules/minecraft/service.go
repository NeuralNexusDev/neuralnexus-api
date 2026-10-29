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
	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
)

// Service - Minecraft player service
type Service interface {
	GetMojangPlayerByName(ctx context.Context, name string) (*Player, error)
	GetMojangPlayerByUUID(ctx context.Context, id string) (*Player, error)
	GetMojangPlayersByNames(ctx context.Context, names []string) ([]*Player, error)
	GetMojangProfile(ctx context.Context, id string, signed bool) (*Player, error)

	GetProfile(ctx context.Context, id string) (*Profile, error)
	GetProfileByName(ctx context.Context, name string) (*Profile, error)
	GetTextureContent(ctx context.Context, hash string) (*TextureResult, error)

	GetGeyserXUID(ctx context.Context, gamertag string) (*GeyserPlayer, error)
	GetGeyserSkin(ctx context.Context, xuid int64) (*GeyserSkin, error)
	GetGeyserProfile(ctx context.Context, xuid int64) (*GeyserProfile, error)
	GetGeyserProfileByGamertag(ctx context.Context, gamertag string) (*GeyserProfile, error)
}

type MojangService interface {
	GetPlayerByName(ctx context.Context, name string) (*Player, error)
	GetPlayerByUUID(ctx context.Context, id string) (*Player, error)
	GetPlayersByNames(ctx context.Context, names []string) ([]*Player, error)
	GetProfile(ctx context.Context, id string, signed bool) (*Player, error)
}

type GeyserService interface {
	GetPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error)
	GetPlayerByGamertag(ctx context.Context, gamertag string) (*GeyserPlayer, error)
	GetSkin(ctx context.Context, xuid int64) (*GeyserSkin, error)
	GetProfile(ctx context.Context, xuid int64) (*GeyserProfile, error)
	GetProfileByGamertag(ctx context.Context, gamertag string) (*GeyserProfile, error)
}

type CommonService interface {
	GetProfile(ctx context.Context, id string) (*Profile, error)
	GetProfileByName(ctx context.Context, name string) (*Profile, error)
	GetTextureContent(ctx context.Context, hash string) (*TextureResult, error)
}

// service - Minecraft player service implementation
type service struct {
	cfg          config.Minecraft
	store        Store
	client       *http.Client
	s3TextureUrl string
}

// NewService - Create a new Minecraft player service
func NewService(cfg config.Minecraft, store Store, client *http.Client, s3TextureUrl string) Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &service{
		store:        store,
		client:       client,
		cfg:          cfg,
		s3TextureUrl: s3TextureUrl,
	}
}

// get issues a GET that is cancelled with ctx.
func (s *service) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req)
}

// GetMojangPlayerByName gets a player by name, cache-first with Mojang fallback
func (s *service) GetMojangPlayerByName(ctx context.Context, name string) (*Player, error) {
	cached, err := s.store.GetPlayerFromCache(ctx, name)
	if err == nil {
		return cached, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	// Cache miss — fetch from DB
	dbPlayer, _ := s.store.GetPlayerByName(ctx, name)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		if err := s.store.SetPlayerInCache(context.WithoutCancel(ctx), dbPlayer); err != nil {
			return nil, err
		}
		return dbPlayer, nil
	}

	// Stale entry — fetch from Mojang
	resp, err := s.get(ctx, s.cfg.LookupByName+url.PathEscape(name))
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

	var player Player
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, err
	}

	if err := s.store.UpsertPlayer(context.WithoutCancel(ctx), &player, false); err != nil {
		return nil, err
	}
	if err := s.store.SetPlayerInCache(context.WithoutCancel(ctx), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// GetMojangPlayerByUUID gets a player by UUID, cache-first with Mojang fallback
func (s *service) GetMojangPlayerByUUID(ctx context.Context, id string) (*Player, error) {
	cached, err := s.store.GetPlayerFromCache(ctx, id)
	if err == nil {
		return cached, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	// Cache miss — fetch from DB
	dbPlayer, _ := s.store.GetPlayerByUUID(ctx, id)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		if err := s.store.SetPlayerInCache(context.WithoutCancel(ctx), dbPlayer); err != nil {
			return nil, err
		}
		return dbPlayer, nil
	}

	// Stale entry — fetch from Mojang
	resp, err := s.get(ctx, s.cfg.LookupByUUID+id)
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

	var player Player
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, err
	}

	if err := s.store.UpsertPlayer(context.WithoutCancel(ctx), &player, false); err != nil {
		return nil, err
	}
	if err := s.store.SetPlayerInCache(context.WithoutCancel(ctx), &player); err != nil {
		return nil, err
	}
	return &player, nil
}

// GetMojangPlayersByNames gets players by name in batch, cache-first with Mojang fallback
// Mojang batch endpoint is capped at 10 names per request
func (s *service) GetMojangPlayersByNames(ctx context.Context, names []string) ([]*Player, error) {
	if len(names) == 0 {
		return nil, ErrNoNamesProvided
	}
	if len(names) > 10 {
		return nil, ErrBatchLimit
	}

	// Check cache first, collect misses
	players := make([]*Player, 0, len(names))
	misses := make([]string, 0, len(names))

	for _, name := range names {
		player, err := s.store.GetPlayerFromCache(ctx, name)
		if err == nil {
			players = append(players, player)
		} else {
			// Cache miss — fetch from DB
			player, _ := s.store.GetPlayerByName(ctx, name)
			if player != nil && !player.IsStale() {
				if err := s.store.SetPlayerInCache(context.WithoutCancel(ctx), player); err != nil {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.LookupBulk, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrMojangAPI.Error(), resp.Status)
		return nil, ErrMojangAPI
	}

	var fetched []Player
	if err := json.NewDecoder(resp.Body).Decode(&fetched); err != nil {
		return nil, err
	}

	for i := range fetched {
		if err := s.store.UpsertPlayer(context.WithoutCancel(ctx), &fetched[i], false); err != nil {
			return nil, err
		}
		if err := s.store.SetPlayerInCache(context.WithoutCancel(ctx), &fetched[i]); err != nil {
			return nil, err
		}
		players = append(players, &fetched[i])
	}

	return players, nil
}

// GetMojangProfile gets a full player profile
func (s *service) GetMojangProfile(ctx context.Context, id string, signed bool) (*Player, error) {
	if signed {
		cached, err := s.store.GetSignedProfileFromCache(ctx, id)
		if err == nil {
			return cached, nil
		}
		if err != ErrCacheMiss {
			return nil, err
		}
		player, _, err := s.fetchProfileFromMojang(ctx, id, true)
		return player, err
	}

	profile, err := s.resolveProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	return profile.ToPlayer()
}

// GetProfile gets a player's profile with textures decoded as native JSON,
// with texture URLs pointing at our own CDN instead of Mojang's.
func (s *service) GetProfile(ctx context.Context, id string) (*Profile, error) {
	profile, err := s.resolveProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	if profile.Textures == nil {
		return profile, nil
	}
	if hash := profile.Textures.Textures.SKIN.Hash(); hash != "" {
		profile.Textures.Textures.SKIN.URL = s.s3TextureUrl + S3KeyPrefix + hash
	}
	if hash := profile.Textures.Textures.CAPE.Hash(); hash != "" {
		profile.Textures.Textures.CAPE.URL = s.s3TextureUrl + S3KeyPrefix + hash
	}
	return profile, nil
}

// GetProfileByName resolves a Java username to its profile.
func (s *service) GetProfileByName(ctx context.Context, name string) (*Profile, error) {
	player, err := s.GetMojangPlayerByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, player.ID)
}

// resolveProfile gets a player's canonical Profile from cache or the
// database, fetching live from Mojang when needed.
func (s *service) resolveProfile(ctx context.Context, id string) (*Profile, error) {
	cached, err := s.store.GetProfileFromCache(ctx, id)
	if err == nil {
		return cached, nil
	}
	if err != ErrCacheMiss {
		return nil, err
	}

	dbProfile, err := s.store.GetProfileByUUID(ctx, id)
	if err != nil && err != ErrPlayerNotFound {
		reqctx.Log(ctx, "Failed to get profile from DB:\n\t", err.Error())
	}
	if dbProfile != nil {
		if dbProfile.ProfileActions == nil {
			dbProfile.ProfileActions = []string{}
		}
		if !dbProfile.IsStale() {
			if err := s.store.SetProfileInCache(context.WithoutCancel(ctx), dbProfile); err != nil {
				return nil, err
			}
			return dbProfile, nil
		}
	}

	_, profile, err := s.fetchProfileFromMojang(ctx, id, false)
	return profile, err
}

// fetchProfileFromMojang fetches and persists a player's profile live from
// Mojang, returning both the raw Player and the decoded Profile.
func (s *service) fetchProfileFromMojang(ctx context.Context, id string, signed bool) (*Player, *Profile, error) {
	url := s.cfg.LookupProfile + id
	if signed {
		url += "?unsigned=false"
	}
	resp, err := s.get(ctx, url)
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

	var player Player
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, nil, err
	}
	if player.ProfileActions == nil {
		player.ProfileActions = []string{}
	}

	if err := s.store.UpsertPlayer(context.WithoutCancel(ctx), &player, true); err != nil {
		return nil, nil, err
	}

	// Extract and store textures
	profile := player.ToProfile(ctx)
	if profile.Textures != nil {
		if hash := profile.Textures.Textures.SKIN.Hash(); hash != "" {
			if err := s.store.UpsertTextureHash(context.WithoutCancel(ctx), hash); err != nil {
				reqctx.Log(ctx, "Failed to store skin hash:\n\t", err.Error())
			}
		}

		if hash := profile.Textures.Textures.CAPE.Hash(); hash != "" {
			if err := s.store.UpsertTextureHash(context.WithoutCancel(ctx), hash); err != nil {
				reqctx.Log(ctx, "Failed to store cape hash:\n\t", err.Error())
			}
		}

		if err := s.store.UpsertTextures(context.WithoutCancel(ctx), JavaPlayerTexturesTable, profile.Textures); err != nil {
			reqctx.Log(ctx, "Failed to store texture:\n\t", err.Error())
		}
	}

	if signed {
		if err := s.store.SetSignedProfileInCache(context.WithoutCancel(ctx), &player); err != nil {
			return nil, nil, err
		}
	} else {
		if err := s.store.SetProfileInCache(context.WithoutCancel(ctx), profile); err != nil {
			return nil, nil, err
		}
	}

	return &player, profile, nil
}

// GetGeyserXUID looks up a Bedrock player's Xbox XUID by gamertag.
func (s *service) GetGeyserXUID(ctx context.Context, gamertag string) (*GeyserPlayer, error) {
	dbPlayer, _ := s.store.GetGeyserPlayerByGamertag(ctx, gamertag)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		return dbPlayer, nil
	}

	// PathEscape, not raw concatenation: a gamertag can contain '#', '?', '/',
	// or spaces, which would otherwise corrupt the upstream request URL.
	resp, err := s.get(ctx, s.cfg.GeyserXUIDLookup+url.PathEscape(gamertag))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidGeyserRequest
	}
	// Geyser has no 404 here: an unknown gamertag is 200 with an empty object.
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrGeyserAPI.Error(), resp.Status)
		return nil, ErrGeyserAPI
	}

	var result struct {
		XUID int64 `json:"xuid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.XUID == 0 {
		return nil, ErrPlayerNotFound
	}

	player := &GeyserPlayer{
		ID:       xuidToUUID(result.XUID),
		Gamertag: gamertag,
		XUID:     result.XUID,
	}

	if err := s.store.UpsertGeyserPlayer(context.WithoutCancel(ctx), player); err != nil {
		return nil, err
	}
	return player, nil
}

// GetGeyserSkin gets a Bedrock player's most recently converted skin by XUID.
func (s *service) GetGeyserSkin(ctx context.Context, xuid int64) (*GeyserSkin, error) {
	// TODO: Simplify
	profile, err := s.GetGeyserProfile(ctx, xuid)
	if err != nil {
		return nil, err
	}

	player, err := s.GetGeyserXUID(ctx, profile.Gamertag)
	if err != nil {
		return nil, err
	}
	var textures *TexturesValue
	if textures, err = s.store.GetTextures(ctx, GeyserPlayerTexturesTable, player.ID, player.Gamertag); err != nil {
		return nil, err
	}

	var skin GeyserSkin
	if !player.IsStale() && textures != nil {
		skin.FromTexturesValue(ctx, textures)
		return &skin, err
	}

	resp, err := s.get(ctx, s.cfg.GeyserSkinLookup+strconv.FormatInt(xuid, 10))
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
		if err := s.store.UpsertTextures(context.WithoutCancel(ctx), GeyserPlayerTexturesTable, textures); err != nil {
			return nil, err
		}
	}
	return &skin, nil
}

// resolveGeyserPlayerByXUID is the reverse of GetGeyserXUID.
func (s *service) resolveGeyserPlayerByXUID(ctx context.Context, xuid int64) (*GeyserPlayer, error) {
	dbPlayer, _ := s.store.GetGeyserPlayerByXUID(ctx, xuid)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		return dbPlayer, nil
	}

	resp, err := s.get(ctx, s.cfg.GeyserGamertagLookup+strconv.FormatInt(xuid, 10))
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

	var result struct {
		Gamertag string `json:"gamertag"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Gamertag == "" {
		return nil, ErrPlayerNotFound
	}

	player := &GeyserPlayer{ID: xuidToUUID(xuid), XUID: xuid, Gamertag: result.Gamertag}
	if err := s.store.UpsertGeyserPlayer(context.WithoutCancel(ctx), player); err != nil {
		return nil, err
	}
	return player, nil
}

// skinStepFailure reports a skin-lookup failure for an xuid that was already
// resolved. It must not wrap ErrInvalidGeyserRequest, which the handlers map to 400.
func skinStepFailure(ctx context.Context, err error) error {
	if err == ErrInvalidGeyserRequest {
		reqctx.Log(ctx, ErrGeyserAPI.Error(), "- skin lookup rejected a resolved xuid")
		return ErrGeyserAPI
	}
	return err
}

// GetGeyserProfile gets a Bedrock player's full profile (identity + skin) by
// XUID. A missing skin leaves Skin nil.
func (s *service) GetGeyserProfile(ctx context.Context, xuid int64) (*GeyserProfile, error) {
	player, err := s.resolveGeyserPlayerByXUID(ctx, xuid)
	if err != nil {
		return nil, err
	}
	skin, err := s.GetGeyserSkin(ctx, player.XUID)
	if err != nil {
		if err != ErrSkinNotFound {
			return nil, skinStepFailure(ctx, err)
		}
		return nil, err
	}
	textures := ParseTextureValue(ctx, skin.Value)
	if textures != nil {
		if err := s.store.UpsertTextures(context.WithoutCancel(ctx), GeyserPlayerTexturesTable, textures); err != nil {
			return nil, err
		}
	}
	return &GeyserProfile{ID: player.ID, XUID: player.XUID, Gamertag: player.Gamertag, Textures: textures}, nil
}

// GetGeyserProfileByGamertag resolves a Bedrock player's full profile by gamertag.
func (s *service) GetGeyserProfileByGamertag(ctx context.Context, gamertag string) (*GeyserProfile, error) {
	player, err := s.GetGeyserXUID(ctx, gamertag)
	if err != nil {
		return nil, err
	}
	skin, err := s.GetGeyserSkin(ctx, player.XUID)
	if err != nil {
		if err != ErrSkinNotFound {
			return nil, skinStepFailure(ctx, err)
		}
		return nil, err
	}
	textures := ParseTextureValue(ctx, skin.Value)
	if textures != nil {
		if err := s.store.UpsertTextures(context.WithoutCancel(ctx), GeyserPlayerTexturesTable, textures); err != nil {
			return nil, err
		}
	}
	return &GeyserProfile{ID: player.ID, XUID: player.XUID, Gamertag: player.Gamertag, Textures: textures}, nil
}

// GetTextureContent returns the texture's bytes and content type, fetching from
// Mojang exactly once on a cache miss instead of round-tripping back through S3.
func (s *service) GetTextureContent(ctx context.Context, hash string) (*TextureResult, error) {
	present, err := s.store.IsTextureInS3(ctx, hash)
	if err != nil {
		return nil, err
	}

	if present {
		return s.serveFromS3(ctx, hash)
	}
	return s.fetchAndArchive(ctx, hash)
}

// serveFromS3 fetches an already-archived texture straight from the CDN/S3
func (s *service) serveFromS3(ctx context.Context, hash string) (*TextureResult, error) {
	resp, err := s.get(ctx, s.s3TextureUrl+S3KeyPrefix+url.PathEscape(hash))
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

// bytesReadCloser adapts a *bytes.Reader to io.ReadCloser while keeping Len()
// visible, so the store can set an explicit Content-Length instead of the
// SDK re-buffering to compute it.
type bytesReadCloser struct {
	*bytes.Reader
}

func (bytesReadCloser) Close() error { return nil }

// fetchAndArchive fetches a texture from Mojang once, archives it to S3, and
// returns a second reader over the same bytes to serve the client — no
// re-fetch through S3 for the request that just caused the miss.
func (s *service) fetchAndArchive(ctx context.Context, hash string) (*TextureResult, error) {
	resp, err := s.get(ctx, s.cfg.LookupTexture+url.PathEscape(hash))
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

	// Archival failure shouldn't fail the client's request — degrade gracefully.
	if err := s.store.PutTextureInS3(context.WithoutCancel(ctx), hash, bytesReadCloser{bytes.NewReader(data)}); err != nil {
		reqctx.Log(ctx, "Failed to upload texture to S3:\n\t", err.Error())
	} else if err := s.store.UpsertTextureHash(context.WithoutCancel(ctx), hash); err != nil {
		reqctx.Log(ctx, "Failed to store texture hash:\n\t", err.Error())
	}

	return &TextureResult{Body: io.NopCloser(bytes.NewReader(data)), ContentType: contentType}, nil
}
