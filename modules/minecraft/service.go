package minecraft

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
)

// Alternatives:
//
//	https://api.mojang.com/users/profiles/minecraft/<name>   (occasional 403 due to Mojang misconfiguration)
//	https://api.mojang.com/minecraft/profile/lookup/name/<name>
const mojangLookupByName = "https://api.minecraftservices.com/minecraft/profile/lookup/name/"

// https://api.minecraftservices.com/minecraft/profile/lookup/<uuid>
const mojangLookupByUUID = "https://api.minecraftservices.com/minecraft/profile/lookup/"

// Alternatives:
//
//	https://api.mojang.com/profiles/minecraft
//	https://api.mojang.com/minecraft/profile/lookup/bulk/byname
const mojangLookupBulk = "https://api.minecraftservices.com/minecraft/profile/lookup/bulk/byname"

// https://sessionserver.mojang.com/session/minecraft/profile/<uuid>
const mojangLookupProfile = "https://sessionserver.mojang.com/session/minecraft/profile/"

// http://textures.minecraft.net/texture/<hash>
const mojangTextureURL = "http://textures.minecraft.net/texture/"

// https://api.geysermc.org/v2/xbox/xuid/<gamertag>
const geyserXUIDLookup = "https://api.geysermc.org/v2/xbox/xuid/"

// https://api.geysermc.org/v2/skin/<xuid>
const geyserSkinLookup = "https://api.geysermc.org/v2/skin/"

// https://api.geysermc.org/v2/xbox/gamertag/<xuid>
const geyserGamertagLookup = "https://api.geysermc.org/v2/xbox/gamertag/"

// Service - Minecraft player service
type Service interface {
	GetMojangPlayerByName(name string) (*Player, error)
	GetMojangPlayerByUUID(id string) (*Player, error)
	GetMojangPlayersByNames(names []string) ([]*Player, error)
	GetMojangProfile(id string, signed bool) (*Player, error)
	GetProfile(id string) (*Profile, error)
	GetProfileByName(name string) (*Profile, error)
	GetTextureContent(hash string) (*TextureResult, error)
	GetGeyserXUID(gamertag string) (*GeyserPlayer, error)
	GetGeyserSkin(xuid int64) (*GeyserSkin, error)
	GetGeyserProfile(xuid int64) (*GeyserProfile, error)
	GetGeyserProfileByGamertag(gamertag string) (*GeyserProfile, error)
	GetGeyserTextureContent(hash string) (*TextureResult, error)
}

// service - Minecraft player service implementation
type service struct {
	store                Store
	client               *http.Client
	lookupByName         string
	lookupByUUID         string
	lookupBulk           string
	lookupProfile        string
	lookupTexture        string
	geyserXUIDLookup     string
	geyserSkinLookup     string
	geyserGamertagLookup string
	nnTextureUrl         string
	nnGeyserTextureUrl   string
}

// NewService - Create a new Minecraft player service
func NewService(store Store, client *http.Client, nnTextureUrl string) Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &service{
		store:                store,
		client:               client,
		lookupByName:         mojangLookupByName,
		lookupByUUID:         mojangLookupByUUID,
		lookupBulk:           mojangLookupBulk,
		lookupProfile:        mojangLookupProfile,
		lookupTexture:        mojangTextureURL,
		geyserXUIDLookup:     geyserXUIDLookup,
		geyserSkinLookup:     geyserSkinLookup,
		geyserGamertagLookup: geyserGamertagLookup,
		nnTextureUrl:         nnTextureUrl,
		nnGeyserTextureUrl:   nnTextureUrl + "geyser/",
	}
}

// GetMojangPlayerByName gets a player by name, cache-first with Mojang fallback
func (s *service) GetMojangPlayerByName(name string) (*Player, error) {
	cached, err := s.store.GetPlayerFromCache(name)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// Cache miss — fetch from DB
	dbPlayer, _ := s.store.GetPlayerByName(name)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		if err := s.store.SetPlayerInCache(dbPlayer); err != nil {
			return nil, err
		}
		return dbPlayer, nil
	}

	// Stale entry — fetch from Mojang
	resp, err := s.client.Get(s.lookupByName + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPlayerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("mojang API error: " + resp.Status)
	}

	var player Player
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, err
	}

	if err := s.store.UpsertPlayer(&player, false); err != nil {
		return nil, err
	}
	if err := s.store.SetPlayerInCache(&player); err != nil {
		return nil, err
	}
	return &player, nil
}

// GetMojangPlayerByUUID gets a player by UUID, cache-first with Mojang fallback
func (s *service) GetMojangPlayerByUUID(id string) (*Player, error) {
	cached, err := s.store.GetPlayerFromCache(id)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// Cache miss — fetch from DB
	dbPlayer, _ := s.store.GetPlayerByUUID(id)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		if err := s.store.SetPlayerInCache(dbPlayer); err != nil {
			return nil, err
		}
		return dbPlayer, nil
	}

	// Stale entry — fetch from Mojang
	resp, err := s.client.Get(s.lookupByUUID + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrPlayerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("mojang API error: " + resp.Status)
	}

	var player Player
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, err
	}

	if err := s.store.UpsertPlayer(&player, false); err != nil {
		return nil, err
	}
	if err := s.store.SetPlayerInCache(&player); err != nil {
		return nil, err
	}
	return &player, nil
}

// GetMojangPlayersByNames gets players by name in batch, cache-first with Mojang fallback
// Mojang batch endpoint is capped at 10 names per request
func (s *service) GetMojangPlayersByNames(names []string) ([]*Player, error) {
	if len(names) == 0 {
		return nil, errors.New("no names provided")
	}
	if len(names) > 10 {
		return nil, errors.New("batch lookup is limited to 10 names")
	}

	// Check cache first, collect misses
	players := make([]*Player, 0, len(names))
	misses := make([]string, 0, len(names))

	for _, name := range names {
		player, err := s.store.GetPlayerFromCache(name)
		if err == nil {
			players = append(players, player)
		} else {
			// Cache miss — fetch from DB
			player, _ := s.store.GetPlayerByName(name)
			if player != nil && !player.IsStale() {
				if err := s.store.SetPlayerInCache(player); err != nil {
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

	resp, err := s.client.Post(s.lookupBulk, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("mojang API error: " + resp.Status)
	}

	var fetched []Player
	if err := json.NewDecoder(resp.Body).Decode(&fetched); err != nil {
		return nil, err
	}

	for i := range fetched {
		if err := s.store.UpsertPlayer(&fetched[i], false); err != nil {
			return nil, err
		}
		if err := s.store.SetPlayerInCache(&fetched[i]); err != nil {
			return nil, err
		}
		players = append(players, &fetched[i])
	}

	return players, nil
}

// GetMojangProfile gets a full player profile
func (s *service) GetMojangProfile(id string, signed bool) (*Player, error) {
	if signed {
		cached, err := s.store.GetSignedProfileFromCache(id)
		if err == nil {
			return cached, nil
		}
		if !errors.Is(err, redis.Nil) {
			return nil, err
		}
		player, _, err := s.fetchProfileFromMojang(id, true)
		return player, err
	}

	profile, err := s.resolveProfile(id)
	if err != nil {
		return nil, err
	}
	return profile.ToPlayer()
}

// GetProfile gets a player's profile with textures decoded as native JSON,
// with texture URLs pointing at our own CDN instead of Mojang's.
func (s *service) GetProfile(id string) (*Profile, error) {
	profile, err := s.resolveProfile(id)
	if err != nil {
		return nil, err
	}
	if profile.Textures == nil {
		return profile, nil
	}
	if profile.Textures.Textures.SKIN != nil {
		profile.Textures.Textures.SKIN.URL = s.nnTextureUrl + profile.Textures.Textures.SKIN.Hash()
	}
	if profile.Textures.Textures.CAPE != nil {
		profile.Textures.Textures.CAPE.URL = s.nnTextureUrl + profile.Textures.Textures.CAPE.Hash()
	}
	return profile, nil
}

// GetProfileByName resolves a Java username to its profile.
func (s *service) GetProfileByName(name string) (*Profile, error) {
	player, err := s.GetMojangPlayerByName(name)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(player.ID)
}

// resolveProfile gets a player's canonical Profile from cache or the
// database, fetching live from Mojang when needed.
func (s *service) resolveProfile(id string) (*Profile, error) {
	cached, err := s.store.GetProfileFromCache(id)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}

	dbProfile, err := s.store.GetProfileByUUID(id)
	if err != nil && !errors.Is(err, ErrPlayerNotFound) {
		log.Println("Failed to get profile from DB:\n\t", err)
	}
	if dbProfile != nil {
		if dbProfile.ProfileActions == nil {
			dbProfile.ProfileActions = []string{}
		}
		if !dbProfile.IsStale() {
			if err := s.store.SetProfileInCache(dbProfile); err != nil {
				return nil, err
			}
			return dbProfile, nil
		}
	}

	_, profile, err := s.fetchProfileFromMojang(id, false)
	return profile, err
}

// fetchProfileFromMojang fetches and persists a player's profile live from
// Mojang, returning both the raw Player and the decoded Profile.
func (s *service) fetchProfileFromMojang(id string, signed bool) (*Player, *Profile, error) {
	url := s.lookupProfile + id
	if signed {
		url += "?unsigned=false"
	}
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil, ErrPlayerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.New("mojang API error: " + resp.Status)
	}

	var player Player
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, nil, err
	}
	if player.ProfileActions == nil {
		player.ProfileActions = []string{}
	}

	if err := s.store.UpsertPlayer(&player, true); err != nil {
		return nil, nil, err
	}

	// Extract and store textures
	profile := player.ToProfile()
	if profile.Textures != nil {
		if hash := textureHash(profile.Textures.Textures.SKIN); hash != nil {
			if err := s.store.UpsertTextureHash(*hash); err != nil {
				log.Println("Failed to store skin hash:\n\t", err)
			}
		}

		if hash := textureHash(profile.Textures.Textures.CAPE); hash != nil {
			if err := s.store.UpsertTextureHash(*hash); err != nil {
				log.Println("Failed to store cape hash:\n\t", err)
			}
		}

		if err := s.store.UpsertTextures(profile.Textures); err != nil {
			log.Println("Failed to store texture:\n\t", err)
		}
	}

	if signed {
		if err := s.store.SetSignedProfileInCache(&player); err != nil {
			return nil, nil, err
		}
	} else {
		if err := s.store.SetProfileInCache(profile); err != nil {
			return nil, nil, err
		}
	}

	return &player, profile, nil
}

// GetGeyserXUID looks up a Bedrock player's Xbox XUID by gamertag.
func (s *service) GetGeyserXUID(gamertag string) (*GeyserPlayer, error) {
	dbPlayer, _ := s.store.GetGeyserPlayerByGamertag(gamertag)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		return dbPlayer, nil
	}

	// PathEscape, not raw concatenation: a gamertag can contain '#', '?', '/',
	// or spaces, which would otherwise corrupt the upstream request URL.
	resp, err := s.client.Get(s.geyserXUIDLookup + url.PathEscape(gamertag))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidGeyserRequest
	}
	// Geyser has no 404 here: an unknown gamertag is 200 with an empty object.
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("geyser API error: " + resp.Status)
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
		Gamertag: gamertag,
		XUID:     result.XUID,
		UUID:     xuidToUUID(result.XUID),
	}

	if err := s.store.UpsertGeyserPlayer(player); err != nil {
		return nil, err
	}
	return player, nil
}

// GetGeyserSkin gets a Bedrock player's most recently converted skin by XUID.
func (s *service) GetGeyserSkin(xuid int64) (*GeyserSkin, error) {
	dbSkin, _ := s.store.GetGeyserSkin(xuid)
	if dbSkin != nil && !dbSkin.IsStale() {
		return dbSkin, nil
	}

	resp, err := s.client.Get(s.geyserSkinLookup + strconv.FormatInt(xuid, 10))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidGeyserRequest
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("geyser API error: " + resp.Status)
	}

	var skin GeyserSkin
	if err := json.NewDecoder(resp.Body).Decode(&skin); err != nil {
		return nil, err
	}
	// An unconverted (or nonexistent) player comes back as 200 with an empty object.
	if skin.Hash == "" {
		return nil, ErrSkinNotFound
	}

	if err := s.store.UpsertGeyserSkin(xuid, &skin); err != nil {
		return nil, err
	}
	return &skin, nil
}

// resolveGeyserPlayerByXUID is the reverse of GetGeyserXUID.
func (s *service) resolveGeyserPlayerByXUID(xuid int64) (*GeyserPlayer, error) {
	dbPlayer, _ := s.store.GetGeyserPlayerByXUID(xuid)
	if dbPlayer != nil && !dbPlayer.IsStale() {
		return dbPlayer, nil
	}

	resp, err := s.client.Get(s.geyserGamertagLookup + strconv.FormatInt(xuid, 10))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidGeyserRequest
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("geyser API error: " + resp.Status)
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

	player := &GeyserPlayer{Gamertag: result.Gamertag, XUID: xuid, UUID: xuidToUUID(xuid)}
	if err := s.store.UpsertGeyserPlayer(player); err != nil {
		return nil, err
	}
	return player, nil
}

// GetGeyserProfile gets a Bedrock player's full profile (identity + skin) by
// XUID. A missing skin leaves Skin nil.
func (s *service) GetGeyserProfile(xuid int64) (*GeyserProfile, error) {
	player, err := s.resolveGeyserPlayerByXUID(xuid)
	if err != nil {
		return nil, err
	}
	skin, err := s.GetGeyserSkin(xuid)
	if err != nil && !errors.Is(err, ErrSkinNotFound) {
		return nil, err
	}
	return &GeyserProfile{UUID: player.UUID, XUID: player.XUID, Gamertag: player.Gamertag, Skin: skin}, nil
}

// GetGeyserProfileByGamertag resolves a Bedrock player's full profile by gamertag.
func (s *service) GetGeyserProfileByGamertag(gamertag string) (*GeyserProfile, error) {
	player, err := s.GetGeyserXUID(gamertag)
	if err != nil {
		return nil, err
	}
	skin, err := s.GetGeyserSkin(player.XUID)
	if err != nil && !errors.Is(err, ErrSkinNotFound) {
		return nil, err
	}
	return &GeyserProfile{UUID: player.UUID, XUID: player.XUID, Gamertag: player.Gamertag, Skin: skin}, nil
}

// GetTextureContent returns the texture's bytes and content type, fetching from
// Mojang exactly once on a cache miss instead of round-tripping back through S3.
func (s *service) GetTextureContent(hash string) (*TextureResult, error) {
	present, err := s.store.IsTextureInS3(hash)
	if err != nil {
		return nil, err
	}

	if present {
		return s.serveFromS3(hash)
	}
	return s.fetchAndArchive(hash)
}

// serveFromS3 fetches an already-archived texture straight from the CDN/S3
func (s *service) serveFromS3(hash string) (*TextureResult, error) {
	resp, err := s.client.Get(s.nnTextureUrl + hash)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrTextureNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("bad status code from S3: %d", resp.StatusCode)
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
func (s *service) fetchAndArchive(hash string) (*TextureResult, error) {
	resp, err := s.client.Get(s.lookupTexture + hash)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrTextureNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code from remote URL: %d", resp.StatusCode)
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
	if err := s.store.PutTextureInS3(hash, bytesReadCloser{bytes.NewReader(data)}); err != nil {
		log.Println("Failed to upload texture to S3:\n\t", err)
	} else if err := s.store.UpsertTextureHash(hash); err != nil {
		log.Println("Failed to store texture hash:\n\t", err)
	}

	return &TextureResult{Body: io.NopCloser(bytes.NewReader(data)), ContentType: contentType}, nil
}

// GetGeyserTextureContent returns a Bedrock skin's bytes and content type,
// archiving to S3 on a miss.
func (s *service) GetGeyserTextureContent(hash string) (*TextureResult, error) {
	present, err := s.store.IsGeyserTextureInS3(hash)
	if err != nil {
		return nil, err
	}

	if present {
		return s.serveGeyserFromS3(hash)
	}
	return s.fetchAndArchiveGeyserTexture(hash)
}

// serveGeyserFromS3 fetches an already-archived Bedrock skin straight from the CDN/S3
func (s *service) serveGeyserFromS3(hash string) (*TextureResult, error) {
	resp, err := s.client.Get(s.nnGeyserTextureUrl + hash)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrTextureNotFound
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("bad status code from S3: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}
	return &TextureResult{Body: resp.Body, ContentType: contentType}, nil
}

// fetchAndArchiveGeyserTexture fetches a Bedrock skin's bytes from the URL
// embedded in its GeyserSkin.Value (Geyser hosts converted skins itself,
// not on Mojang's texture CDN), archives it to S3, and returns a second
// reader over the same bytes to serve the client.
func (s *service) fetchAndArchiveGeyserTexture(hash string) (*TextureResult, error) {
	skin, err := s.store.GetGeyserSkinByHash(hash)
	if err != nil {
		return nil, err
	}
	if skin == nil {
		return nil, ErrTextureNotFound
	}
	skinURL := skin.SkinURL()
	if skinURL == "" {
		return nil, ErrTextureNotFound
	}

	resp, err := s.client.Get(skinURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrTextureNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code from remote URL: %d", resp.StatusCode)
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
	if err := s.store.PutGeyserTextureInS3(hash, bytesReadCloser{bytes.NewReader(data)}); err != nil {
		log.Println("Failed to upload Geyser texture to S3:\n\t", err)
	}

	return &TextureResult{Body: io.NopCloser(bytes.NewReader(data)), ContentType: contentType}, nil
}
