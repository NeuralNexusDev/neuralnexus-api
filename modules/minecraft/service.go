package minecraft

import (
	"errors"
	"net/http"
	"strings"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
)

// Primary endpoint — no known 403 issue
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

// Service - Minecraft player service
type Service interface {
	GetPlayerByName(name string) (*Player, error)
	GetPlayerByUUID(id string) (*Player, error)
	GetPlayersByNames(names []string) ([]*Player, error)
}

// service - Minecraft player service implementation
type service struct {
	store        Store
	client       *http.Client
	lookupByName string
	lookupByUUID string
	lookupBulk   string
}

// NewService - Create a new Minecraft player service
func NewService(store Store, client *http.Client) Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &service{
		store:        store,
		client:       client,
		lookupByName: mojangLookupByName,
		lookupByUUID: mojangLookupByUUID,
		lookupBulk:   mojangLookupBulk,
	}
}

// GetPlayerByName gets a player by name, cache-first with Mojang fallback
func (s *service) GetPlayerByName(name string) (*Player, error) {
	cachePlayer, err := s.store.GetPlayerFromCache(name)
	if err == nil {
		return cachePlayer, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// Cache miss — fetch from Mojang
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

	if err := s.store.UpsertPlayer(&player); err != nil {
		return nil, err
	}
	if err := s.store.SetPlayerInCache(&player); err != nil {
		return nil, err
	}
	return &player, nil
}

// GetPlayerByUUID gets a player by UUID, cache-first with Mojang fallback
func (s *service) GetPlayerByUUID(id string) (*Player, error) {
	cachePlayer, err := s.store.GetPlayerFromCache(id)
	if err == nil {
		return cachePlayer, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// Cache miss — fetch from Mojang
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

	if err := s.store.UpsertPlayer(&player); err != nil {
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
			misses = append(misses, name)
		}
	}

	if len(misses) == 0 {
		return players, nil
	}

	// Fetch misses from Mojang
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
		if err := s.store.UpsertPlayer(&fetched[i]); err != nil {
			return nil, err
		}
		if err := s.store.SetPlayerInCache(&fetched[i]); err != nil {
			return nil, err
		}
		players = append(players, &fetched[i])
	}

	return players, nil
}
