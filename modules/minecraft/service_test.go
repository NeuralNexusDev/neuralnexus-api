package minecraft

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
)

// mockStore implements the Store interface for unit testing the Service layer.
type mockStore struct {
	playersByName map[string]*Player
	playersByUUID map[string]*Player
	cache         map[string]*Player
	profilesCache map[string]*Player
	s3Textures    map[string]bool
}

func (m *mockStore) GetPlayerByUUID(id string) (*Player, error) {
	if p, ok := m.playersByUUID[id]; ok {
		return p, nil
	}
	return nil, ErrPlayerNotFound
}

func (m *mockStore) GetPlayerByName(name string) (*Player, error) {
	if p, ok := m.playersByName[name]; ok {
		return p, nil
	}
	return nil, ErrPlayerNotFound
}

func (m *mockStore) GetProfileByUUID(id string) (*Player, error) {
	if p, ok := m.playersByUUID[id]; ok {
		return p, nil
	}
	return nil, ErrPlayerNotFound
}

func (m *mockStore) GetTextures(id string) (*TexturesRow, error) {
	return nil, nil
}

func (m *mockStore) UpsertPlayer(player *Player, updateProfile bool) error {
	if m.playersByName == nil {
		m.playersByName = make(map[string]*Player)
	}
	if m.playersByUUID == nil {
		m.playersByUUID = make(map[string]*Player)
	}
	m.playersByName[player.Name] = player
	m.playersByUUID[player.ID] = player
	return nil
}

func (m *mockStore) UpsertTextures(value *TexturesValue) error {
	return nil
}

func (m *mockStore) UpsertTextureHash(hash string) error {
	return nil
}

func (m *mockStore) GetPlayerFromCache(key string) (*Player, error) {
	if m.cache != nil {
		if p, ok := m.cache[key]; ok {
			return p, nil
		}
	}
	return nil, redis.Nil
}

func (m *mockStore) SetPlayerInCache(player *Player) error {
	if m.cache == nil {
		m.cache = make(map[string]*Player)
	}
	m.cache[player.ID] = player
	m.cache[player.Name] = player
	return nil
}

func (m *mockStore) GetProfileFromCache(id string, signed bool) (*Player, error) {
	if m.profilesCache != nil {
		key := id
		if signed {
			key += "_signed"
		} else {
			key += "_unsigned"
		}
		if p, ok := m.profilesCache[key]; ok {
			return p, nil
		}
	}
	return nil, redis.Nil
}

func (m *mockStore) SetProfileInCache(player *Player, signed bool) error {
	if m.profilesCache == nil {
		m.profilesCache = make(map[string]*Player)
	}
	key := player.ID
	if signed {
		key += "_signed"
	} else {
		key += "_unsigned"
	}
	m.profilesCache[key] = player
	return nil
}

func (m *mockStore) IsTextureInS3(hash string) (bool, error) {
	if m.s3Textures != nil && m.s3Textures[hash] {
		return true, nil
	}
	return false, nil
}

func (m *mockStore) PutTextureInS3(hash string, body io.ReadCloser) error {
	if m.s3Textures == nil {
		m.s3Textures = make(map[string]bool)
	}
	m.s3Textures[hash] = true
	return nil
}

// --- Tests ---

func TestService_GetPlayerByName_CacheHit(t *testing.T) {
	store := &mockStore{
		cache: map[string]*Player{
			"jeb_": {ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"},
		},
	}

	svc := NewService(store, nil, "http://localhost/texture/")
	player, err := svc.GetPlayerByName("jeb_")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.Name != "jeb_" {
		t.Errorf("expected jeb_, got %s", player.Name)
	}
}

func TestService_GetPlayerByName_MojangFallback(t *testing.T) {
	// Mock Mojang API server
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Player{
			ID:   "853c80ef3c3749fdaa49938b674adae6",
			Name: "jeb_",
		})
	}))
	defer mojangServer.Close()

	store := &mockStore{}
	svc := NewService(store, mojangServer.Client(), "http://localhost/texture/")

	// Point service lookup to mock server URL
	s := svc.(*service)
	s.lookupByName = mojangServer.URL + "/"

	player, err := svc.GetPlayerByName("jeb_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.ID != "853c80ef3c3749fdaa49938b674adae6" {
		t.Errorf("expected matching UUID, got %s", player.ID)
	}
}

func TestService_GetPlayersByNames_Validation(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, nil, "http://localhost/texture/")

	// Test empty slice
	_, err := svc.GetPlayersByNames([]string{})
	if err == nil {
		t.Error("expected error for empty names list")
	}

	// Test slice exceeding batch cap of 10
	tooMany := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}
	_, err = svc.GetPlayersByNames(tooMany)
	if err == nil {
		t.Error("expected error for batch lookup exceeding 10 names")
	}
}

func TestService_GetTexture_FromS3(t *testing.T) {
	store := &mockStore{
		s3Textures: map[string]bool{"abc123hash": true},
	}

	svc := NewService(store, nil, "http://cdn.neuralnexus.dev/texture/")
	url, err := svc.GetTexture("abc123hash", false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedURL := "http://cdn.neuralnexus.dev/texture/abc123hash"
	if url != expectedURL {
		t.Errorf("expected %s, got %s", expectedURL, url)
	}
}
