package minecraft

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// mockStore implements the Store interface for unit testing the Service layer.
type mockStore struct {
	playersByName         map[string]*Player
	playersByUUID         map[string]*Player
	profilesByUUID        map[string]*Profile
	cache                 map[string]*Player
	profilesCache         map[string]*Profile
	signedProfilesCache   map[string]*Player
	s3Textures            map[string]bool
	putBodies             map[string][]byte
	putErr                error
	upsertedTextureHashes []string
	geyserPlayers         map[string]*GeyserPlayer
	geyserPlayersByXUID   map[int64]*GeyserPlayer
	geyserSkins           map[int64]*GeyserSkin
	geyserSkinsByHash     map[string]*GeyserSkin
	geyserS3Textures      map[string]bool
	geyserPutBodies       map[string][]byte
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

func (m *mockStore) GetProfileByUUID(id string) (*Profile, error) {
	if p, ok := m.profilesByUUID[id]; ok {
		return p, nil
	}
	return nil, ErrPlayerNotFound
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
	m.upsertedTextureHashes = append(m.upsertedTextureHashes, hash)
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

func (m *mockStore) GetProfileFromCache(id string) (*Profile, error) {
	if m.profilesCache != nil {
		if p, ok := m.profilesCache[id]; ok {
			return p, nil
		}
	}
	return nil, redis.Nil
}

func (m *mockStore) SetProfileInCache(profile *Profile) error {
	if m.profilesCache == nil {
		m.profilesCache = make(map[string]*Profile)
	}
	m.profilesCache[profile.ID] = profile
	return nil
}

func (m *mockStore) GetSignedProfileFromCache(id string) (*Player, error) {
	if m.signedProfilesCache != nil {
		if p, ok := m.signedProfilesCache[id]; ok {
			return p, nil
		}
	}
	return nil, redis.Nil
}

func (m *mockStore) SetSignedProfileInCache(player *Player) error {
	if m.signedProfilesCache == nil {
		m.signedProfilesCache = make(map[string]*Player)
	}
	m.signedProfilesCache[player.ID] = player
	return nil
}

// GetGeyserPlayerByGamertag mirrors the real store's contract: UUID is
// derived from XUID on read, not trusted from whatever the test fixture set.
func (m *mockStore) GetGeyserPlayerByGamertag(gamertag string) (*GeyserPlayer, error) {
	if p, ok := m.geyserPlayers[gamertag]; ok {
		p.UUID = xuidToUUID(p.XUID)
		return p, nil
	}
	return nil, ErrPlayerNotFound
}

func (m *mockStore) GetGeyserPlayerByXUID(xuid int64) (*GeyserPlayer, error) {
	if p, ok := m.geyserPlayersByXUID[xuid]; ok {
		p.UUID = xuidToUUID(p.XUID)
		return p, nil
	}
	return nil, ErrPlayerNotFound
}

func (m *mockStore) UpsertGeyserPlayer(player *GeyserPlayer) error {
	if m.geyserPlayers == nil {
		m.geyserPlayers = make(map[string]*GeyserPlayer)
	}
	if m.geyserPlayersByXUID == nil {
		m.geyserPlayersByXUID = make(map[int64]*GeyserPlayer)
	}
	m.geyserPlayers[player.Gamertag] = player
	m.geyserPlayersByXUID[player.XUID] = player
	return nil
}

func (m *mockStore) GetGeyserSkin(xuid int64) (*GeyserSkin, error) {
	if s, ok := m.geyserSkins[xuid]; ok {
		return s, nil
	}
	return nil, ErrSkinNotFound
}

func (m *mockStore) GetGeyserSkinByHash(hash string) (*GeyserSkin, error) {
	return m.geyserSkinsByHash[hash], nil
}

func (m *mockStore) UpsertGeyserSkin(xuid int64, skin *GeyserSkin) error {
	if m.geyserSkins == nil {
		m.geyserSkins = make(map[int64]*GeyserSkin)
	}
	if m.geyserSkinsByHash == nil {
		m.geyserSkinsByHash = make(map[string]*GeyserSkin)
	}
	m.geyserSkins[xuid] = skin
	m.geyserSkinsByHash[skin.Hash] = skin
	return nil
}

func (m *mockStore) IsGeyserTextureInS3(hash string) (bool, error) {
	if m.geyserS3Textures != nil && m.geyserS3Textures[hash] {
		return true, nil
	}
	return false, nil
}

func (m *mockStore) PutGeyserTextureInS3(hash string, body io.ReadCloser) error {
	if m.putErr != nil {
		return m.putErr
	}
	if m.geyserS3Textures == nil {
		m.geyserS3Textures = make(map[string]bool)
	}
	m.geyserS3Textures[hash] = true

	if m.geyserPutBodies == nil {
		m.geyserPutBodies = make(map[string][]byte)
	}
	data, _ := io.ReadAll(body)
	m.geyserPutBodies[hash] = data
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

func TestService_GetMojangPlayerByName_CacheHit(t *testing.T) {
	store := &mockStore{
		cache: map[string]*Player{
			"jeb_": {ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"},
		},
	}

	svc := NewService(store, nil, "http://localhost/texture/")
	player, err := svc.GetMojangPlayerByName("jeb_")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.Name != "jeb_" {
		t.Errorf("expected jeb_, got %s", player.Name)
	}
}

func TestService_GetMojangPlayerByName_MojangFallback(t *testing.T) {
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

	player, err := svc.GetMojangPlayerByName("jeb_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.ID != "853c80ef3c3749fdaa49938b674adae6" {
		t.Errorf("expected matching UUID, got %s", player.ID)
	}
}

func TestService_GetMojangPlayersByNames_Validation(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, nil, "http://localhost/texture/")

	// Test empty slice
	_, err := svc.GetMojangPlayersByNames([]string{})
	if err == nil {
		t.Error("expected error for empty names list")
	}

	// Test slice exceeding batch cap of 10
	tooMany := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}
	_, err = svc.GetMojangPlayersByNames(tooMany)
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

func TestService_GetTextureContent_MissPath_MojangNotFound(t *testing.T) {
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupTexture = mojang.URL + "/"

	_, err := svc.GetTextureContent("newhash")
	if !errors.Is(err, ErrTextureNotFound) {
		t.Errorf("expected ErrTextureNotFound, got %v", err)
	}
	if len(store.putBodies) != 0 {
		t.Error("PutTextureInS3 should not be called when the texture doesn't exist")
	}
}

func TestService_GetTextureContent_MissPath_MojangOutage(t *testing.T) {
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
	if errors.Is(err, ErrTextureNotFound) {
		t.Error("a 500 from Mojang should not be reported as ErrTextureNotFound")
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

func TestService_GetMojangProfile_DBHit_NilProfileActionsNormalized(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {ID: id, Name: "jeb_", LastSeen: time.Now().UnixMilli()},
		},
	}

	svc := NewService(store, nil, "http://localhost/texture/")
	player, err := svc.GetMojangProfile(id, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.ProfileActions == nil {
		t.Error("expected ProfileActions to be normalized to a non-nil empty slice, got nil")
	}
	if len(player.ProfileActions) != 0 {
		t.Errorf("expected no profile actions, got %v", player.ProfileActions)
	}
}

func TestService_GetMojangProfile_DBHit_MirrorsAsBase64Property(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {
				ID: id, Name: "jeb_", LastSeen: time.Now().UnixMilli(),
				Textures: &TexturesValue{
					ProfileID: id, ProfileName: "jeb_",
					Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + "abc123hash"}},
				},
			},
		},
	}

	svc := NewService(store, nil, "http://localhost/texture/")
	player, err := svc.GetMojangProfile(id, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(player.Properties) != 1 || player.Properties[0].Name != TEXTURES {
		t.Fatalf("expected a single textures property mirroring Mojang's shape, got %v", player.Properties)
	}
	if _, err := base64.StdEncoding.DecodeString(player.Properties[0].Value); err != nil {
		t.Errorf("expected the property value to be base64, got %q: %v", player.Properties[0].Value, err)
	}
}

func TestService_GetMojangProfile_DBHit_NoStoredTextures(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {ID: id, Name: "jeb_", LastSeen: time.Now().UnixMilli()},
		},
	}

	svc := NewService(store, nil, "http://localhost/texture/")
	player, err := svc.GetMojangProfile(id, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(player.Properties) != 0 {
		t.Errorf("expected no properties when the player has no stored textures, got %v", player.Properties)
	}
}

func TestService_GetMojangProfile_Unsigned_StaleDBEntry_FetchesAndMirrors(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		textures := TexturesValue{
			ProfileID:   id,
			ProfileName: "jeb_",
			Textures: Textures{
				SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123hash"},
			},
		}
		prop, _ := textures.ToProperty()
		json.NewEncoder(w).Encode(Player{
			ID:         id,
			Name:       "jeb_",
			Properties: []Property{*prop},
		})
	}))
	defer mojang.Close()

	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {ID: id, Name: "jeb_", LastSeen: 0}, // stale
		},
	}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	player, err := svc.GetMojangProfile(id, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(player.Properties) != 1 || player.Properties[0].Name != TEXTURES {
		t.Fatalf("expected a single textures property, got %v", player.Properties)
	}
	got := player.ParseProperties()
	if got == nil || got.Textures.SKIN == nil || got.Textures.SKIN.URL != "http://textures.minecraft.net/texture/abc123hash" {
		t.Errorf("expected the freshly-fetched skin URL to survive the round trip, got %+v", got)
	}
}

func TestService_FetchProfileFromMojang_NoCape_DoesNotUpsertEmptyHash(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		textures := TexturesValue{
			ProfileID:   id,
			ProfileName: "jeb_",
			Textures: Textures{
				SKIN: &Texture{URL: "http://textures.minecraft.net/texture/skinhash123"},
			},
		}
		prop, _ := textures.ToProperty()
		json.NewEncoder(w).Encode(Player{
			ID:         id,
			Name:       "jeb_",
			Properties: []Property{*prop},
		})
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	if _, err := svc.GetMojangProfile(id, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Regression guard: a player with no cape must never reach
	// UpsertTextureHash with an empty hash — the DB rejects that outright
	// (textures_hash_not_empty), and fetchProfileFromMojang used to call
	// UpsertTextureHash(CAPE.Hash()) unconditionally.
	for _, h := range store.upsertedTextureHashes {
		if h == "" {
			t.Error("UpsertTextureHash should never be called with an empty hash")
		}
	}
	if len(store.upsertedTextureHashes) != 1 || store.upsertedTextureHashes[0] != "skinhash123" {
		t.Errorf("expected exactly one UpsertTextureHash call for the skin hash, got %v", store.upsertedTextureHashes)
	}
}

func TestService_FetchProfileFromMojang_SkinAndCape_UpsertsBothHashes(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		textures := TexturesValue{
			ProfileID:   id,
			ProfileName: "jeb_",
			Textures: Textures{
				SKIN: &Texture{URL: "http://textures.minecraft.net/texture/skinhash123"},
				CAPE: &Texture{URL: "http://textures.minecraft.net/texture/capehash456"},
			},
		}
		prop, _ := textures.ToProperty()
		json.NewEncoder(w).Encode(Player{
			ID:         id,
			Name:       "jeb_",
			Properties: []Property{*prop},
		})
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	if _, err := svc.GetMojangProfile(id, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.upsertedTextureHashes) != 2 {
		t.Fatalf("expected UpsertTextureHash to be called for both skin and cape, got %v", store.upsertedTextureHashes)
	}
	want := map[string]bool{"skinhash123": true, "capehash456": true}
	for _, h := range store.upsertedTextureHashes {
		if !want[h] {
			t.Errorf("unexpected hash upserted: %q", h)
		}
	}
}

func TestService_GetProfile_UnknownUUID_FetchesFromMojang(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Player{ID: id, Name: "jeb_"})
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	profile, err := svc.GetProfile(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Name != "jeb_" {
		t.Errorf("expected the freshly-fetched profile, got %+v", profile)
	}
}

func TestService_GetProfile_UnknownUUID_MojangNotFound(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	_, err := svc.GetProfile(id)
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestService_GetProfile_DBHit_TexturesReturnedAsJSONNotBase64Property(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {
				ID: id, Name: "jeb_", LastSeen: time.Now().UnixMilli(),
				Textures: &TexturesValue{
					ProfileID: id, ProfileName: "jeb_",
					Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + "abc123hash"}},
				},
			},
		},
	}

	svc := NewService(store, nil, "http://localhost/texture/")
	profile, err := svc.GetProfile(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Textures == nil {
		t.Fatal("expected Textures to be populated from the stored profile")
	}
	if profile.Textures.Textures.SKIN == nil || profile.Textures.Textures.SKIN.URL != "http://localhost/texture/abc123hash" {
		t.Errorf("expected the decoded route to use our own texture URL, got %+v", profile.Textures.Textures.SKIN)
	}
}

func TestService_GetMojangProfile_DBHit_KeepsMojangTextureURL(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {
				ID: id, Name: "jeb_", LastSeen: time.Now().UnixMilli(),
				Textures: &TexturesValue{
					ProfileID: id, ProfileName: "jeb_",
					Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + "abc123hash"}},
				},
			},
		},
	}

	svc := NewService(store, nil, "http://localhost/texture/")
	player, err := svc.GetMojangProfile(id, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := player.ParseProperties()
	if decoded == nil || decoded.Textures.SKIN == nil || decoded.Textures.SKIN.URL != mojangTextureURL+"abc123hash" {
		t.Errorf("expected the mirror route to keep Mojang's texture URL, got %+v", decoded)
	}
}

func TestService_GetProfile_MojangFetch_TexturesDecoded(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		textures := TexturesValue{
			ProfileID:   id,
			ProfileName: "jeb_",
			Textures: Textures{
				SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123hash"},
			},
		}
		prop, _ := textures.ToProperty()
		json.NewEncoder(w).Encode(Player{
			ID:         id,
			Name:       "jeb_",
			Properties: []Property{*prop},
		})
	}))
	defer mojang.Close()

	// GetProfile always resolves unsigned, so it only ever reaches Mojang
	// for a DB entry that's gone stale.
	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {ID: id, Name: "jeb_", LastSeen: 0},
		},
	}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	profile, err := svc.GetProfile(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Textures == nil || profile.Textures.Textures.SKIN == nil {
		t.Fatalf("expected decoded textures, got %+v", profile.Textures)
	}
	if profile.Textures.Textures.SKIN.URL != "http://localhost/texture/abc123hash" {
		t.Errorf("expected our own texture URL, got %s", profile.Textures.Textures.SKIN.URL)
	}
}

func TestService_GetMojangProfile_MojangFetch_NilProfileActionsNormalized(t *testing.T) {
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Intentionally omit "profileActions" from the response, as Mojang
		// does for accounts with no actions.
		json.NewEncoder(w).Encode(Player{
			ID:   "853c80ef3c3749fdaa49938b674adae6",
			Name: "jeb_",
		})
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	player, err := svc.GetMojangProfile("853c80ef3c3749fdaa49938b674adae6", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.ProfileActions == nil {
		t.Error("expected ProfileActions to be normalized to a non-nil empty slice, got nil")
	}
	if len(player.ProfileActions) != 0 {
		t.Errorf("expected no profile actions, got %v", player.ProfileActions)
	}
}

func TestService_GetMojangProfile_Signed_CacheHitSkipsMojang(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("did not expect a Mojang fetch on a signed cache hit")
	}))
	defer mojang.Close()

	store := &mockStore{
		signedProfilesCache: map[string]*Player{
			id: {ID: id, Name: "jeb_", Properties: []Property{{Name: TEXTURES, Value: "cached", Signature: "sig123"}}},
		},
	}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	player, err := svc.GetMojangProfile(id, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(player.Properties) != 1 || player.Properties[0].Signature != "sig123" {
		t.Errorf("expected the cached signed property verbatim, got %v", player.Properties)
	}
}

func TestService_GetMojangProfile_Signed_FetchCachesSeparatelyFromUnsigned(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Player{
			ID:   id,
			Name: "jeb_",
			Properties: []Property{
				{Name: TEXTURES, Value: encodedTextures(t, TexturesValue{ProfileID: id, ProfileName: "jeb_"}), Signature: "sig123"},
			},
		})
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupProfile = mojang.URL + "/"

	if _, err := svc.GetMojangProfile(id, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := store.signedProfilesCache[id]; !ok {
		t.Error("expected the signed response to be cached under the signed cache")
	}
	if _, ok := store.profilesCache[id]; ok {
		t.Error("expected a signed fetch not to populate the decoded Profile cache, since it can't represent a real signature")
	}
}

func TestService_GetGeyserXUID_OK(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Notch" {
			t.Errorf("expected gamertag in path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int64{"xuid": 2535457445285308})
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserXUIDLookup = geyser.URL + "/"

	player, err := svc.GetGeyserXUID("Notch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.Gamertag != "Notch" {
		t.Errorf("expected Notch, got %s", player.Gamertag)
	}
	if player.XUID != 2535457445285308 {
		t.Errorf("expected xuid 2535457445285308, got %d", player.XUID)
	}
	if player.UUID != xuidToUUID(2535457445285308) {
		t.Errorf("expected derived UUID %s, got %s", xuidToUUID(2535457445285308), player.UUID)
	}
}

// TestService_GetGeyserXUID_EscapesGamertagSpecialCharacters guards against a
// gamertag being concatenated raw into the upstream URL. Real Xbox gamertags
// can contain spaces and a "#dddd" discriminator suffix; unescaped, '#'
// truncates the request at the fragment and '?'/'/' inject extra query/path
// structure into the request actually sent to Geyser.
func TestService_GetGeyserXUID_EscapesGamertagSpecialCharacters(t *testing.T) {
	gamertags := []string{"Foo#1234", "Foo?bar=1", "Foo/Bar", "Foo%1234", "Foo Bar", "Notch"}
	for _, gamertag := range gamertags {
		t.Run(gamertag, func(t *testing.T) {
			var gotSegment string
			geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Recover exactly the escaped last path segment as sent on
				// the wire — r.URL.Path is pre-decoded by net/http and would
				// misreport an escaped '/' as an extra path segment.
				escaped := r.URL.EscapedPath()
				seg := escaped[strings.LastIndex(escaped, "/")+1:]
				unescaped, err := url.PathUnescape(seg)
				if err != nil {
					t.Fatalf("failed to unescape segment %q: %v", seg, err)
				}
				gotSegment = unescaped
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]int64{"xuid": 1})
			}))
			defer geyser.Close()

			store := &mockStore{}
			svc := NewService(store, geyser.Client(), "http://localhost/texture/")
			s := svc.(*service)
			s.geyserXUIDLookup = geyser.URL + "/"

			if _, err := svc.GetGeyserXUID(gamertag); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotSegment != gamertag {
				t.Errorf("expected upstream request for gamertag %q, got %q", gamertag, gotSegment)
			}
		})
	}
}

func TestService_GetGeyserXUID_InvalidGamertag(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserXUIDLookup = geyser.URL + "/"

	_, err := svc.GetGeyserXUID("this-gamertag-is-way-too-long-for-xbox")
	if !errors.Is(err, ErrInvalidGeyserRequest) {
		t.Errorf("expected ErrInvalidGeyserRequest, got %v", err)
	}
}

func TestService_GetGeyserXUID_NotFound(t *testing.T) {
	// Geyser's API has no 404 for this endpoint: an unknown gamertag comes
	// back as 200 with an empty object.
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserXUIDLookup = geyser.URL + "/"

	_, err := svc.GetGeyserXUID("nonexistent")
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestService_GetGeyserXUID_UpstreamError(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserXUIDLookup = geyser.URL + "/"

	_, err := svc.GetGeyserXUID("Notch")
	if err == nil {
		t.Fatal("expected an error for a non-200/404 upstream response")
	}
}

func TestService_GetGeyserXUID_DBCacheHit(t *testing.T) {
	store := &mockStore{
		geyserPlayers: map[string]*GeyserPlayer{
			"Notch": {Gamertag: "Notch", XUID: 2535457445285308, LastSeen: time.Now().UnixMilli()},
		},
	}
	// No mock Geyser server: a network call here would fail the test.
	svc := NewService(store, nil, "http://localhost/texture/")

	player, err := svc.GetGeyserXUID("Notch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.XUID != 2535457445285308 {
		t.Errorf("expected xuid 2535457445285308, got %d", player.XUID)
	}
	if player.UUID != xuidToUUID(2535457445285308) {
		t.Errorf("expected derived UUID %s, got %s", xuidToUUID(2535457445285308), player.UUID)
	}
}

func TestService_GetGeyserXUID_StaleDBEntry_RefetchesFromGeyser(t *testing.T) {
	var geyserHits int
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		geyserHits++
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int64{"xuid": 2535457445285308})
	}))
	defer geyser.Close()

	store := &mockStore{
		geyserPlayers: map[string]*GeyserPlayer{
			"Notch": {
				Gamertag: "Notch",
				XUID:     2535457445285308,
				LastSeen: time.Now().Add(-48 * time.Hour).UnixMilli(),
			},
		},
	}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserXUIDLookup = geyser.URL + "/"

	if _, err := svc.GetGeyserXUID("Notch"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if geyserHits != 1 {
		t.Errorf("expected a refetch from Geyser for a stale DB entry, got %d hits", geyserHits)
	}
}

func TestService_GetGeyserXUID_UpsertsOnFetch(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int64{"xuid": 2535457445285308})
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserXUIDLookup = geyser.URL + "/"

	if _, err := svc.GetGeyserXUID("Notch"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.geyserPlayers["Notch"]; !ok {
		t.Error("expected the fetched gamertag->XUID mapping to be persisted")
	}
}

func TestXUIDToUUID(t *testing.T) {
	got := xuidToUUID(2535457445285308)
	want := "00000000-0000-0000-0009-01fc305e8dbc"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestUUIDToXUID_RoundTrip(t *testing.T) {
	const xuid int64 = 2535457445285308
	got, err := uuidToXUID(xuidToUUID(xuid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != xuid {
		t.Errorf("expected %d, got %d", xuid, got)
	}
}

func TestUUIDToXUID_RejectsNonDerivedUUID(t *testing.T) {
	if _, err := uuidToXUID("853c80ef-3c37-49fd-aa49-938b674adae6"); err == nil {
		t.Error("expected error for a UUID with nonzero high bits")
	}
}

func TestUUIDToXUID_RejectsMalformedUUID(t *testing.T) {
	if _, err := uuidToXUID("not-a-uuid"); err == nil {
		t.Error("expected error for a malformed UUID")
	}
}

func TestService_GetGeyserSkin_OK(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2535457445285308" {
			t.Errorf("expected xuid in path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GeyserSkin{
			Hash:      "abc123",
			IsSteve:   true,
			TextureID: "def456",
			Value:     "base64value",
		})
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserSkinLookup = geyser.URL + "/"

	skin, err := svc.GetGeyserSkin(2535457445285308)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skin.Hash != "abc123" {
		t.Errorf("expected abc123, got %s", skin.Hash)
	}
	if !skin.IsSteve {
		t.Error("expected IsSteve to be true")
	}
}

func TestService_GetGeyserSkin_NotFound(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserSkinLookup = geyser.URL + "/"

	_, err := svc.GetGeyserSkin(2535457445285308)
	if !errors.Is(err, ErrSkinNotFound) {
		t.Errorf("expected ErrSkinNotFound, got %v", err)
	}
}

func TestService_GetGeyserSkin_UpstreamError(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserSkinLookup = geyser.URL + "/"

	_, err := svc.GetGeyserSkin(2535457445285308)
	if err == nil {
		t.Fatal("expected an error for a non-200 upstream response")
	}
}

func TestService_GetGeyserSkin_InvalidXUID(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserSkinLookup = geyser.URL + "/"

	_, err := svc.GetGeyserSkin(2535457445285308)
	if !errors.Is(err, ErrInvalidGeyserRequest) {
		t.Errorf("expected ErrInvalidGeyserRequest, got %v", err)
	}
}

func TestService_GetGeyserSkin_DBCacheHit(t *testing.T) {
	store := &mockStore{
		geyserSkins: map[int64]*GeyserSkin{
			2535457445285308: {Hash: "cached-hash", IsSteve: true, LastSeen: time.Now().UnixMilli()},
		},
	}
	// No mock Geyser server: a network call here would fail the test.
	svc := NewService(store, nil, "http://localhost/texture/")

	skin, err := svc.GetGeyserSkin(2535457445285308)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skin.Hash != "cached-hash" {
		t.Errorf("expected cached-hash, got %s", skin.Hash)
	}
}

func TestService_GetGeyserSkin_StaleDBEntry_RefetchesFromGeyser(t *testing.T) {
	var geyserHits int
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		geyserHits++
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GeyserSkin{Hash: "fresh-hash", TextureID: "id", Value: "val"})
	}))
	defer geyser.Close()

	store := &mockStore{
		geyserSkins: map[int64]*GeyserSkin{
			2535457445285308: {
				Hash:     "stale-hash",
				LastSeen: time.Now().Add(-48 * time.Hour).UnixMilli(),
			},
		},
	}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserSkinLookup = geyser.URL + "/"

	skin, err := svc.GetGeyserSkin(2535457445285308)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if geyserHits != 1 {
		t.Errorf("expected a refetch from Geyser for a stale DB entry, got %d hits", geyserHits)
	}
	if skin.Hash != "fresh-hash" {
		t.Errorf("expected fresh-hash, got %s", skin.Hash)
	}
}

func TestService_GetGeyserSkin_UpsertsOnFetch(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GeyserSkin{Hash: "abc123", TextureID: "id", Value: "val"})
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserSkinLookup = geyser.URL + "/"

	if _, err := svc.GetGeyserSkin(2535457445285308); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.geyserSkins[2535457445285308]; !ok {
		t.Error("expected the fetched skin to be persisted")
	}
}

func TestService_GetGeyserTextureContent_S3Hit(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("cached-geyser-texture-bytes"))
	}))
	defer cdn.Close()

	store := &mockStore{geyserS3Textures: map[string]bool{"abc123hash": true}}
	svc := NewService(store, cdn.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.nnGeyserTextureUrl = cdn.URL + "/"

	result, err := svc.GetGeyserTextureContent("abc123hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "cached-geyser-texture-bytes" {
		t.Errorf("expected cached-geyser-texture-bytes, got %s", body)
	}
	if len(store.geyserPutBodies) != 0 {
		t.Error("did not expect PutGeyserTextureInS3 to be called on a cache hit")
	}
}

func TestService_GetGeyserTextureContent_S3Hit_NotFound(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer cdn.Close()

	store := &mockStore{geyserS3Textures: map[string]bool{"abc123hash": true}}
	svc := NewService(store, cdn.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.nnGeyserTextureUrl = cdn.URL + "/"

	_, err := svc.GetGeyserTextureContent("abc123hash")
	if !errors.Is(err, ErrTextureNotFound) {
		t.Errorf("expected ErrTextureNotFound, got %v", err)
	}
}

func TestService_GetGeyserTextureContent_MissPath_FetchesOnceAndArchives(t *testing.T) {
	var upstreamHits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fresh-geyser-texture-bytes"))
	}))
	defer upstream.Close()

	value := TexturesValue{Textures: Textures{SKIN: &Texture{URL: upstream.URL + "/skin.png"}}}
	store := &mockStore{geyserSkinsByHash: map[string]*GeyserSkin{
		"newhash": {Hash: "newhash", Value: encodedTextures(t, value)},
	}}
	svc := NewService(store, upstream.Client(), "http://localhost/texture/")

	result, err := svc.GetGeyserTextureContent("newhash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "fresh-geyser-texture-bytes" {
		t.Errorf("expected fresh-geyser-texture-bytes, got %s", body)
	}
	if upstreamHits != 1 {
		t.Errorf("expected exactly 1 upstream fetch, got %d", upstreamHits)
	}
	if string(store.geyserPutBodies["newhash"]) != "fresh-geyser-texture-bytes" {
		t.Errorf("expected PutGeyserTextureInS3 to receive the fetched bytes, got %q", store.geyserPutBodies["newhash"])
	}
}

func TestService_GetGeyserTextureContent_MissPath_UnknownHash(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, nil, "http://localhost/texture/")

	_, err := svc.GetGeyserTextureContent("nonexistent")
	if !errors.Is(err, ErrTextureNotFound) {
		t.Errorf("expected ErrTextureNotFound, got %v", err)
	}
}

func TestService_GetGeyserTextureContent_MissPath_UndecodableValue(t *testing.T) {
	store := &mockStore{geyserSkinsByHash: map[string]*GeyserSkin{
		"badhash": {Hash: "badhash", Value: "not-valid-base64!!!"},
	}}
	svc := NewService(store, nil, "http://localhost/texture/")

	_, err := svc.GetGeyserTextureContent("badhash")
	if !errors.Is(err, ErrTextureNotFound) {
		t.Errorf("expected ErrTextureNotFound, got %v", err)
	}
}

func TestService_GetGeyserTextureContent_MissPath_UpstreamOutage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	value := TexturesValue{Textures: Textures{SKIN: &Texture{URL: upstream.URL + "/skin.png"}}}
	store := &mockStore{geyserSkinsByHash: map[string]*GeyserSkin{
		"newhash": {Hash: "newhash", Value: encodedTextures(t, value)},
	}}
	svc := NewService(store, upstream.Client(), "http://localhost/texture/")

	_, err := svc.GetGeyserTextureContent("newhash")
	if err == nil {
		t.Fatal("expected error on non-200 from upstream")
	}
	if errors.Is(err, ErrTextureNotFound) {
		t.Error("a 500 from upstream should not be reported as ErrTextureNotFound")
	}
}

func TestService_GetGeyserTextureContent_MissPath_ArchiveFailureStillServesClient(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fresh-geyser-texture-bytes"))
	}))
	defer upstream.Close()

	value := TexturesValue{Textures: Textures{SKIN: &Texture{URL: upstream.URL + "/skin.png"}}}
	store := &mockStore{
		putErr: errors.New("s3 unavailable"),
		geyserSkinsByHash: map[string]*GeyserSkin{
			"newhash": {Hash: "newhash", Value: encodedTextures(t, value)},
		},
	}
	svc := NewService(store, upstream.Client(), "http://localhost/texture/")

	result, err := svc.GetGeyserTextureContent("newhash")
	if err != nil {
		t.Fatalf("expected client to still be served when archival fails, got error: %v", err)
	}
	defer result.Body.Close()

	body, _ := io.ReadAll(result.Body)
	if string(body) != "fresh-geyser-texture-bytes" {
		t.Errorf("expected fresh-geyser-texture-bytes, got %s", body)
	}
}

func TestService_GetProfileByName_ResolvesNameThenProfile(t *testing.T) {
	id := "853c80ef3c3749fdaa49938b674adae6"
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Player{ID: id, Name: "jeb_"})
	}))
	defer mojang.Close()

	store := &mockStore{
		profilesByUUID: map[string]*Profile{
			id: {ID: id, Name: "jeb_", LastSeen: time.Now().UnixMilli()},
		},
	}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupByName = mojang.URL + "/"

	profile, err := svc.GetProfileByName("jeb_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.ID != id {
		t.Errorf("expected id %s, got %s", id, profile.ID)
	}
}

func TestService_GetProfileByName_NotFound(t *testing.T) {
	mojang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mojang.Close()

	store := &mockStore{}
	svc := NewService(store, mojang.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.lookupByName = mojang.URL + "/"

	_, err := svc.GetProfileByName("nonexistent")
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestService_GetGeyserProfile_ComposesPlayerAndSkin(t *testing.T) {
	const xuid int64 = 2535457445285308
	store := &mockStore{
		geyserPlayersByXUID: map[int64]*GeyserPlayer{
			xuid: {Gamertag: "Notch", XUID: xuid, LastSeen: time.Now().UnixMilli()},
		},
		geyserSkins: map[int64]*GeyserSkin{
			xuid: {Hash: "abc123", TextureID: "def456", Value: "val", LastSeen: time.Now().UnixMilli()},
		},
	}
	// No mock Geyser server: both lookups should be satisfied from the store.
	svc := NewService(store, nil, "http://localhost/texture/")

	profile, err := svc.GetGeyserProfile(xuid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Gamertag != "Notch" || profile.XUID != xuid {
		t.Errorf("expected composed identity, got %+v", profile)
	}
	if profile.Skin == nil || profile.Skin.Hash != "abc123" {
		t.Errorf("expected composed skin, got %+v", profile.Skin)
	}
}

func TestService_GetGeyserProfile_NoSkinYet(t *testing.T) {
	const xuid int64 = 2535457445285308
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(GeyserSkin{})
	}))
	defer geyser.Close()

	store := &mockStore{
		geyserPlayersByXUID: map[int64]*GeyserPlayer{
			xuid: {Gamertag: "Notch", XUID: xuid, LastSeen: time.Now().UnixMilli()},
		},
	}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserSkinLookup = geyser.URL + "/"

	profile, err := svc.GetGeyserProfile(xuid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Skin != nil {
		t.Errorf("expected nil skin, got %+v", profile.Skin)
	}
}

func TestService_GetGeyserProfile_UnknownXUID_FetchesGamertagFromGeyser(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/gamertag/"):
			json.NewEncoder(w).Encode(map[string]string{"gamertag": "Notch"})
		case strings.HasPrefix(r.URL.Path, "/skin/"):
			json.NewEncoder(w).Encode(GeyserSkin{})
		}
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserGamertagLookup = geyser.URL + "/gamertag/"
	s.geyserSkinLookup = geyser.URL + "/skin/"

	profile, err := svc.GetGeyserProfile(2535457445285308)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Gamertag != "Notch" {
		t.Errorf("expected gamertag Notch, got %s", profile.Gamertag)
	}
	if _, ok := store.geyserPlayersByXUID[2535457445285308]; !ok {
		t.Error("expected the resolved player to be persisted")
	}
}

func TestService_GetGeyserProfile_UnknownXUID_NotFound(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserGamertagLookup = geyser.URL + "/"

	_, err := svc.GetGeyserProfile(2535457445285308)
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestService_GetGeyserProfile_InvalidXUID_UpstreamRejected(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserGamertagLookup = geyser.URL + "/"

	_, err := svc.GetGeyserProfile(-1)
	if !errors.Is(err, ErrInvalidGeyserRequest) {
		t.Errorf("expected ErrInvalidGeyserRequest, got %v", err)
	}
}

func TestService_GetGeyserProfileByGamertag_ComposesPlayerAndSkin(t *testing.T) {
	const xuid int64 = 2535457445285308
	store := &mockStore{
		geyserPlayers: map[string]*GeyserPlayer{
			"Notch": {Gamertag: "Notch", XUID: xuid, LastSeen: time.Now().UnixMilli()},
		},
		geyserSkins: map[int64]*GeyserSkin{
			xuid: {Hash: "abc123", TextureID: "def456", Value: "val", LastSeen: time.Now().UnixMilli()},
		},
	}
	svc := NewService(store, nil, "http://localhost/texture/")

	profile, err := svc.GetGeyserProfileByGamertag("Notch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.UUID != xuidToUUID(xuid) {
		t.Errorf("expected derived UUID %s, got %s", xuidToUUID(xuid), profile.UUID)
	}
	if profile.Skin == nil || profile.Skin.Hash != "abc123" {
		t.Errorf("expected composed skin, got %+v", profile.Skin)
	}
}

func TestService_GetGeyserProfileByGamertag_PropagatesNotFound(t *testing.T) {
	geyser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]int64{})
	}))
	defer geyser.Close()

	store := &mockStore{}
	svc := NewService(store, geyser.Client(), "http://localhost/texture/")
	s := svc.(*service)
	s.geyserXUIDLookup = geyser.URL + "/"

	_, err := svc.GetGeyserProfileByGamertag("nonexistent")
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}
