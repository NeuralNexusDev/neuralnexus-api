package minecraft

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// --- Setup ---

func setupStore(t *testing.T) Store {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if pgURL == "" || redisURL == "" {
		t.Skip("TEST_POSTGRES_URL and TEST_REDIS_URL must be set to run store tests")
	}

	db, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("failed to parse redis URL: %v", err)
	}
	rdb := redis.NewClient(opt)

	t.Cleanup(func() {
		db.Exec(context.Background(), "DELETE FROM player_names WHERE player_id = '853c80ef-3c37-49fd-aa49-938b674adae6'")
		db.Exec(context.Background(), "DELETE FROM players WHERE id = '853c80ef-3c37-49fd-aa49-938b674adae6'")
		rdb.Del(context.Background(), CachePlayer+"853c80ef3c3749fdaa49938b674adae6", CachePlayer+"jeb_")
		db.Close()
		rdb.Close()
	})

	return NewStore(db, rdb)
}

func setupRawDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run store tests")
	}
	db, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var testPlayer = &Player{
	ID:   "853c80ef3c3749fdaa49938b674adae6",
	Name: "jeb_",
}

// --- Tests ---

func TestStore_UpsertPlayer_Insert(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetPlayerByUUID(testPlayer.ID)
	if err != nil {
		t.Fatalf("failed to get player: %v", err)
	}
	if got.Name != testPlayer.Name {
		t.Errorf("expected %s, got %s", testPlayer.Name, got.Name)
	}
}

func TestStore_UpsertPlayer_UpdateLastSeen(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	got1, _ := s.GetPlayerByUUID(testPlayer.ID)
	firstSeen := got1.FirstSeen

	// Small sleep to ensure last_seen differs
	time.Sleep(10 * time.Millisecond)

	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	got2, _ := s.GetPlayerByUUID(testPlayer.ID)
	if got2.FirstSeen != firstSeen {
		t.Error("first_seen should not change on upsert")
	}
	if got2.LastSeen <= got1.LastSeen {
		t.Error("last_seen should be updated on upsert")
	}
}

func TestStore_UpsertPlayer_ProfileFields_NotUpdatedWithoutFlag(t *testing.T) {
	s := setupStore(t)

	full := &Player{
		ID:     testPlayer.ID,
		Name:   testPlayer.Name,
		Legacy: true,
		Demo:   true,
	}
	if err := s.UpsertPlayer(full, true); err != nil {
		t.Fatalf("upsert with profile failed: %v", err)
	}

	// Now upsert without profile flag — legacy and demo should remain unchanged
	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("upsert without profile failed: %v", err)
	}

	got, err := s.GetPlayerByUUID(testPlayer.ID)
	if err != nil {
		t.Fatalf("failed to get player: %v", err)
	}
	if !got.Legacy {
		t.Error("legacy should not be overwritten when updateProfile is false")
	}
	if !got.Demo {
		t.Error("demo should not be overwritten when updateProfile is false")
	}
}

func TestStore_UpsertPlayer_NameHistory(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Simulate name change
	renamed := &Player{ID: testPlayer.ID, Name: "jeb_renamed"}
	if err := s.UpsertPlayer(renamed, false); err != nil {
		t.Fatalf("upsert with new name failed: %v", err)
	}

	// Cleanup extra name entry
	t.Cleanup(func() {
		// handled by setupStore cleanup of the player row cascade
	})
}

func TestStore_GetPlayerByUUID_NotFound(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetPlayerByUUID("00000000000000000000000000000000")
	if err == nil {
		t.Error("expected error for unknown UUID")
	}
}

func TestStore_GetPlayerByName_NotFound(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetPlayerByName("nonexistent_player_xyz")
	if err == nil {
		t.Error("expected error for unknown name")
	}
}

func TestStore_SetPlayerInCache_And_Get(t *testing.T) {
	s := setupStore(t)

	if err := s.SetPlayerInCache(testPlayer); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// By UUID
	got, err := s.GetPlayerFromCache(testPlayer.ID)
	if err != nil {
		t.Fatalf("failed to get from cache by UUID: %v", err)
	}
	if got.Name != testPlayer.Name {
		t.Errorf("expected %s, got %s", testPlayer.Name, got.Name)
	}

	// By name
	got, err = s.GetPlayerFromCache(testPlayer.Name)
	if err != nil {
		t.Fatalf("failed to get from cache by name: %v", err)
	}
	if got.ID != testPlayer.ID {
		t.Errorf("expected %s, got %s", testPlayer.ID, got.ID)
	}
}

func TestStore_GetPlayerFromCache_Miss(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetPlayerFromCache("nonexistent_key")
	if err == nil {
		t.Error("expected cache miss error")
	}
}

func TestSetProfileInCache_Unsigned(t *testing.T) {
	s := setupStore(t)

	player := &Player{
		ID:   "853c80ef3c3749fdaa49938b674adae6",
		Name: "jeb_",
		Properties: []Property{
			{Name: TEXTURES, Value: encodedTextures(t, TexturesValue{
				ProfileID:   "853c80ef3c3749fdaa49938b674adae6",
				ProfileName: "jeb_",
				Textures:    Textures{SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123"}},
			})},
		},
	}

	if err := s.UpsertPlayer(player, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}
	if err := s.SetPlayerInCache(player); err != nil {
		t.Fatalf("failed to set player in cache: %v", err)
	}
	if err := s.SetProfileInCache(player, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetProfileFromCache(player.ID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Properties) != 1 {
		t.Fatalf("expected 1 property, got %d", len(got.Properties))
	}
	if got.Properties[0].Signature != "" {
		t.Error("expected no signature on unsigned response")
	}
	if got.Properties[0].Value != player.Properties[0].Value {
		t.Error("expected cached value to match what was set, verbatim")
	}
}

func TestSetProfileInCache_Signed(t *testing.T) {
	s := setupStore(t)

	player := &Player{
		ID:   "853c80ef3c3749fdaa49938b674adae6",
		Name: "jeb_",
		Properties: []Property{
			{
				Name: TEXTURES,
				Value: encodedTextures(t, TexturesValue{
					ProfileID:         "853c80ef3c3749fdaa49938b674adae6",
					ProfileName:       "jeb_",
					SignatureRequired: true,
					Textures:          Textures{SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123"}},
				}),
				Signature: "sig123",
			},
		},
	}

	if err := s.UpsertPlayer(player, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}
	if err := s.SetPlayerInCache(player); err != nil {
		t.Fatalf("failed to set player in cache: %v", err)
	}
	if err := s.SetProfileInCache(player, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Signed cache holds the property, signature included, verbatim
	gotSigned, err := s.GetProfileFromCache(player.ID, true)
	if err != nil {
		t.Fatalf("unexpected error getting signed: %v", err)
	}
	if gotSigned.Properties[0].Signature != "sig123" {
		t.Errorf("expected sig123, got %s", gotSigned.Properties[0].Signature)
	}
	if gotSigned.Properties[0].Value != player.Properties[0].Value {
		t.Error("expected cached value to match what was set, verbatim")
	}

	// Signed and unsigned caches are independent — no unsigned entry was ever written
	if _, err := s.GetProfileFromCache(player.ID, false); err == nil {
		t.Error("expected cache miss on unsigned key")
	}
}

func TestGetProfileFromCache_Miss(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetProfileFromCache("00000000000000000000000000000000", false)
	if err == nil {
		t.Error("expected cache miss error")
	}
}

func TestGetProfileFromCache_MissingProperties(t *testing.T) {
	s := setupStore(t)

	player := &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}
	if err := s.UpsertPlayer(player, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}
	if err := s.SetPlayerInCache(player); err != nil {
		t.Fatalf("failed to set player in cache: %v", err)
	}

	// Player is in cache but properties are not
	_, err := s.GetProfileFromCache(player.ID, false)
	if err == nil {
		t.Error("expected error when properties not cached")
	}
}

func TestStore_GetProfileByUUID_HydratesTextures(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}

	skin := &Texture{
		URL:      "http://textures.minecraft.net/texture/skin123",
		Metadata: &Metadata{Model: SLIM},
	}
	cape := &Texture{URL: "http://textures.minecraft.net/texture/cape456"}
	if err := s.UpsertTextureHash(skin); err != nil {
		t.Fatalf("failed to upsert skin hash: %v", err)
	}
	if err := s.UpsertTextureHash(cape); err != nil {
		t.Fatalf("failed to upsert cape hash: %v", err)
	}

	value := &TexturesValue{
		Timestamp: 1234567890000,
		ProfileID: testPlayer.ID,
		Textures:  Textures{SKIN: skin, CAPE: cape},
	}
	if err := s.UpsertTextures(value); err != nil {
		t.Fatalf("failed to upsert textures: %v", err)
	}

	got, err := s.GetProfileByUUID(testPlayer.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Properties) != 1 {
		t.Fatalf("expected 1 property, got %d", len(got.Properties))
	}

	decoded, err := base64.StdEncoding.DecodeString(got.Properties[0].Value)
	if err != nil {
		t.Fatalf("failed to decode property: %v", err)
	}
	var textures TexturesValue
	if err := json.Unmarshal(decoded, &textures); err != nil {
		t.Fatalf("failed to unmarshal textures: %v", err)
	}
	if textures.Textures.SKIN == nil || textures.Textures.SKIN.Hash() != "skin123" {
		t.Errorf("expected skin hash skin123, got %+v", textures.Textures.SKIN)
	}
	if textures.Textures.SKIN.Metadata == nil || textures.Textures.SKIN.Metadata.Model != SLIM {
		t.Error("expected slim model to be preserved")
	}
	if textures.Textures.CAPE == nil || textures.Textures.CAPE.Hash() != "cape456" {
		t.Errorf("expected cape hash cape456, got %+v", textures.Textures.CAPE)
	}
}

func TestStore_GetProfileByUUID_NoTextures(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}

	got, err := s.GetProfileByUUID(testPlayer.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Properties) != 0 {
		t.Errorf("expected no properties when none stored, got %d", len(got.Properties))
	}
}

func TestStore_UpsertTextureHash(t *testing.T) {
	s := setupStore(t)

	tex := &Texture{URL: "http://textures.minecraft.net/texture/abc123"}
	if err := s.UpsertTextureHash(tex); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second upsert should not error
	if err := s.UpsertTextureHash(tex); err != nil {
		t.Fatalf("unexpected error on duplicate: %v", err)
	}

	t.Cleanup(func() {
		db := setupRawDB(t)
		db.Exec(context.Background(), "DELETE FROM textures WHERE hash = 'abc123'")
	})
}

func TestStore_UpsertTextureHash_Nil(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertTextureHash(nil); err != nil {
		t.Errorf("expected nil error for nil texture, got %v", err)
	}
}

func TestStore_UpsertTextures(t *testing.T) {
	s := setupStore(t)

	// Player must exist first
	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}

	skin := &Texture{URL: "http://textures.minecraft.net/texture/skin123"}
	cape := &Texture{URL: "http://textures.minecraft.net/texture/cape456"}

	if err := s.UpsertTextureHash(skin); err != nil {
		t.Fatalf("failed to upsert skin hash: %v", err)
	}
	if err := s.UpsertTextureHash(cape); err != nil {
		t.Fatalf("failed to upsert cape hash: %v", err)
	}

	value := &TexturesValue{
		Timestamp: 1234567890000,
		ProfileID: testPlayer.ID,
		Textures: Textures{
			SKIN: skin,
			CAPE: cape,
		},
	}

	if err := s.UpsertTextures(value); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second upsert should update last_seen without error
	if err := s.UpsertTextures(value); err != nil {
		t.Fatalf("unexpected error on duplicate: %v", err)
	}
}

func TestStore_UpsertTextures_SlimModel(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}

	skin := &Texture{
		URL:      "http://textures.minecraft.net/texture/slim123",
		Metadata: &Metadata{Model: SLIM},
	}

	if err := s.UpsertTextureHash(skin); err != nil {
		t.Fatalf("failed to upsert skin hash: %v", err)
	}

	value := &TexturesValue{
		Timestamp: 1234567890000,
		ProfileID: testPlayer.ID,
		Textures:  Textures{SKIN: skin},
	}

	if err := s.UpsertTextures(value); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
