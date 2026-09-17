package minecraft

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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

// Service - Minecraft player service
type Service interface {
	GetPlayerByName(name string) (*Player, error)
	GetPlayerByUUID(id string) (*Player, error)
	GetPlayersByNames(names []string) ([]*Player, error)
	GetProfile(id string, signed bool) (*Player, error)
	GetTextureContent(hash string) (*TextureResult, error)
}

// service - Minecraft player service implementation
type service struct {
	store         Store
	client        *http.Client
	lookupByName  string
	lookupByUUID  string
	lookupBulk    string
	lookupProfile string
	lookupTexture string
	nnTextureUrl  string
}

// NewService - Create a new Minecraft player service
func NewService(store Store, client *http.Client, nnTextureUrl string) Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &service{
		store:         store,
		client:        client,
		lookupByName:  mojangLookupByName,
		lookupByUUID:  mojangLookupByUUID,
		lookupBulk:    mojangLookupBulk,
		lookupProfile: mojangLookupProfile,
		lookupTexture: mojangTextureURL,
		nnTextureUrl:  nnTextureUrl,
	}
}

// GetPlayerByName gets a player by name, cache-first with Mojang fallback
func (s *service) GetPlayerByName(name string) (*Player, error) {
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

// GetPlayerByUUID gets a player by UUID, cache-first with Mojang fallback
func (s *service) GetPlayerByUUID(id string) (*Player, error) {
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

// GetPlayersByNames gets players by name in batch, cache-first with Mojang fallback
// Mojang batch endpoint is capped at 10 names per request
func (s *service) GetPlayersByNames(names []string) ([]*Player, error) {
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

// GetProfile gets a full player profile including properties
func (s *service) GetProfile(id string, signed bool) (*Player, error) {
	cached, err := s.store.GetProfileFromCache(id, signed)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// Cache miss — fetch from DB
	if !signed {
		dbPlayer, _ := s.store.GetProfileByUUID(id)
		if dbPlayer == nil {
			return nil, ErrPlayerNotFound
		}
		if !dbPlayer.IsStale() {
			row, err := s.store.GetTextures(dbPlayer.ID)
			if err != nil {
				return nil, err
			}
			prop, err := row.Value(dbPlayer.Name, s.lookupTexture).ToProperty()
			if err != nil {
				log.Println("Failed to decode TexturesRow:\n\t", err)
			}
			if prop != nil {
				dbPlayer.Properties = append(dbPlayer.Properties, *prop)
			}
			if err := s.store.SetProfileInCache(dbPlayer, false); err != nil {
				return nil, err
			}
			return dbPlayer, nil
		}
	}

	// Stale entry — fetch from Mojang
	url := s.lookupProfile + id
	if signed {
		url += "?unsigned=false"
	}
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, ErrPlayerNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("mojang API error: " + resp.Status)
	}

	var player Player
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		return nil, err
	}

	// Upsert player
	if err := s.store.UpsertPlayer(&player, true); err != nil {
		return nil, err
	}

	// Extract and store textures
	value := player.ParseProperties()
	if value != nil {
		if err := s.store.UpsertTextureHash(value.Textures.SKIN.Hash()); err != nil {
			log.Println("Failed to store skin hash:\n\t", err)
		}

		if err := s.store.UpsertTextureHash(value.Textures.CAPE.Hash()); err != nil {
			log.Println("Failed to store cape hash:\n\t", err)
		}

		if err := s.store.UpsertTextures(value); err != nil {
			log.Println("Failed to store texture:\n\t", err)
		}
	}

	if err := s.store.SetProfileInCache(&player, signed); err != nil {
		return nil, err
	}
	return &player, nil
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
	if err := s.store.PutTextureInS3(hash, io.NopCloser(bytes.NewReader(data))); err != nil {
		log.Println("Failed to upload texture to S3:\n\t", err)
	} else if err := s.store.UpsertTextureHash(hash); err != nil {
		log.Println("Failed to store texture hash:\n\t", err)
	}

	return &TextureResult{Body: io.NopCloser(bytes.NewReader(data)), ContentType: contentType}, nil
}
