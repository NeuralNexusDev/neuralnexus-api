package minecraft

import (
	"encoding/json"
	"errors"
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
	putBodies     map[string][]byte
	putErr        error
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
	if m.putErr != nil {
		return m.putErr
	}
	if m.s3Textures == nil {
		m.s3Textures = make(map[string]bool)
	}
	m.s3Textures[hash] = true

	if m.putBodies == nil {
		m.putBodies = make(map[string][]byte)
	}
	data, _ := io.ReadAll(body)
	m.putBodies[hash] = data
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

func TestService_GetTextureContent_S3Hit(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("cached-texture-bytes"))
	}))
	defer cdn.Close()

	store := &mockStore{s3Textures: map[string]bool{"abc123hash": true}}
	svc := NewService(store, cdn.Client(), cdn.URL+"/")

	result, err := svc.GetTextureContent("abc123hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "cached-texture-bytes" {
		t.Errorf("expected cached-texture-bytes, got %s", body)
	}
	if result.ContentType != "image/png" {
		t.Errorf("expected image/png, got %s", result.ContentType)
	}
	if len(store.putBodies) != 0 {
		t.Error("did not expect PutTextureInS3 to be called on a cache hit")
	}
}

func TestService_GetTextureContent_S3Hit_NotFound(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cdn.Close()

	store := &mockStore{s3Textures: map[string]bool{"abc123hash": true}}
	svc := NewService(store, cdn.Client(), cdn.URL+"/")

	_, err := svc.GetTextureContent("abc123hash")
	if !errors.Is(err, ErrTextureNotFound) {
		t.Errorf("expected ErrTextureNotFound, got %v", err)
	}
}

func TestService_GetTextureContent_MissPath_FetchesOnceAndArchives(t *testing.T) {
	var mojangHits int
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mojangHits++
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fresh-texture-bytes"))
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupTexture = mojang.URL + "/"

	result, err := svc.GetTextureContent("newhash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "fresh-texture-bytes" {
		t.Errorf("expected fresh-texture-bytes, got %s", body)
	}
	if mojangHits != 1 {
		t.Errorf("expected exactly 1 Mojang fetch, got %d", mojangHits)
	}
	if string(store.putBodies["newhash"]) != "fresh-texture-bytes" {
		t.Errorf("expected PutTextureInS3 to receive the fetched bytes, got %q", store.putBodies["newhash"])
	}
}

func TestService_GetTextureContent_MissPath_MojangError(t *testing.T) {
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupTexture = mojang.URL + "/"

	_, err := svc.GetTextureContent("newhash")
	if err == nil {
		t.Fatal("expected error on non-200 from Mojang")
	}
	if len(store.putBodies) != 0 {
		t.Error("PutTextureInS3 should not be called when the Mojang fetch fails")
	}
}

func TestService_GetTextureContent_MissPath_ArchiveFailureStillServesClient(t *testing.T) {
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fresh-texture-bytes"))
	}))
	defer mojang.Close()

	store := &mockStore{putErr: errors.New("s3 unavailable")}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupTexture = mojang.URL + "/"

	result, err := svc.GetTextureContent("newhash")
	if err != nil {
		t.Fatalf("expected client to still be served when archival fails, got error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "fresh-texture-bytes" {
		t.Errorf("expected fresh-texture-bytes, got %s", body)
	}
}
