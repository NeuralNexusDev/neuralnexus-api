package minecraft

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
		db.Exec(context.Background(), "DELETE FROM player_textures WHERE player_id = '853c80ef-3c37-49fd-aa49-938b674adae6'")
		db.Exec(context.Background(), "DELETE FROM players WHERE id = '853c80ef-3c37-49fd-aa49-938b674adae6'")
		db.Exec(context.Background(), "DELETE FROM geyser_player_textures WHERE xuid = 2535457445285308")
		db.Exec(context.Background(), "DELETE FROM geyser_players WHERE xuid = 2535457445285308")
		rdb.Del(context.Background(),
			CachePlayer+"853c80ef-3c37-49fd-aa49-938b674adae6", CachePlayer+"jeb_",
			CacheProfile+"853c80ef-3c37-49fd-aa49-938b674adae6",
			CacheProfileSigned+"853c80ef-3c37-49fd-aa49-938b674adae6",
			// The Redis-only cache tests (SetProfileInCache, SetSignedProfileInCache)
			// use this dashless literal directly instead of testPlayer.
			CacheProfile+"853c80ef3c3749fdaa49938b674adae6",
			CacheProfileSigned+"853c80ef3c3749fdaa49938b674adae6",
		)
		db.Close()
		rdb.Close()
	})

	return NewStore(db, rdb, nil)
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

// setupMockS3 creates an isolated AWS S3 Client targeting a mock HTTP server.
func setupMockS3(t *testing.T, handler http.HandlerFunc) *s3.Client {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{URL: server.URL}, nil
	})

	cfg := aws.Config{
		Region:                      "us-east-1",
		Credentials:                 credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
		EndpointResolverWithOptions: resolver,
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
}

// players.id is a UUID column, which Postgres always returns in canonical
// dashed form regardless of how it was written — so the fixture uses that
// form too, rather than only matching by accident on the round trip.
var testPlayer = &Player{
	ID:   "853c80ef-3c37-49fd-aa49-938b674adae6",
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

	// Verify we can find the player by their new name
	got, err := s.GetPlayerByName("jeb_renamed")
	if err != nil {
		t.Fatalf("failed to get player by new name: %v", err)
	}
	if got.ID != testPlayer.ID {
		t.Errorf("expected UUID %s, got %s", testPlayer.ID, got.ID)
	}

	// Verify looking up by UUID returns the updated name
	gotUUID, err := s.GetPlayerByUUID(testPlayer.ID)
	if err != nil {
		t.Fatalf("failed to get player by UUID: %v", err)
	}
	if gotUUID.Name != "jeb_renamed" {
		t.Errorf("expected name jeb_renamed, got %s", gotUUID.Name)
	}

	// Additional cleanup for the extra name entry in player_names
	t.Cleanup(func() {
		db := setupRawDB(t)
		db.Exec(context.Background(), "DELETE FROM player_names WHERE name = 'jeb_renamed'")
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

func TestStore_SetProfileInCache(t *testing.T) {
	s := setupStore(t)

	profile := &Profile{
		ID:   "853c80ef3c3749fdaa49938b674adae6",
		Name: "jeb_",
		Textures: &TexturesValue{
			ProfileID:   "853c80ef3c3749fdaa49938b674adae6",
			ProfileName: "jeb_",
			Textures:    Textures{SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123"}},
		},
	}

	if err := s.SetProfileInCache(profile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetProfileFromCache(profile.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Textures == nil || got.Textures.Textures.SKIN == nil {
		t.Fatal("expected cached textures to round-trip")
	}
	if got.Textures.Textures.SKIN.URL != profile.Textures.Textures.SKIN.URL {
		t.Error("expected cached value to match what was set, verbatim")
	}
}

func TestStore_SetSignedProfileInCache(t *testing.T) {
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

	if err := s.SetSignedProfileInCache(player); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetSignedProfileFromCache(player.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Properties) != 1 || got.Properties[0].Signature != "sig123" {
		t.Errorf("expected the signature to round-trip verbatim, got %v", got.Properties)
	}

	// The signed cache is separate from the decoded Profile cache — no
	// unsigned entry was ever written for this ID.
	if _, err := s.GetProfileFromCache(player.ID); err == nil {
		t.Error("expected no entry in the decoded Profile cache")
	}
}

func TestGetProfileFromCache_Miss(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetProfileFromCache("00000000000000000000000000000000")
	if err == nil {
		t.Error("expected cache miss error")
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

	if err := s.UpsertTextureHash(skin.Hash()); err != nil {
		t.Fatalf("failed to upsert skin hash: %v", err)
	}
	if err := s.UpsertTextureHash(cape.Hash()); err != nil {
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
	if got.Textures == nil {
		t.Fatal("expected textures to be hydrated")
	}
	if got.Textures.Textures.SKIN == nil || got.Textures.Textures.SKIN.Hash() != "skin123" {
		t.Errorf("expected skin hash skin123, got %+v", got.Textures.Textures.SKIN)
	}
	if got.Textures.Textures.SKIN.Metadata == nil || got.Textures.Textures.SKIN.Metadata.Model != SLIM {
		t.Error("expected slim model to be preserved")
	}
	if got.Textures.Textures.CAPE == nil || got.Textures.Textures.CAPE.Hash() != "cape456" {
		t.Errorf("expected cape hash cape456, got %+v", got.Textures.Textures.CAPE)
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
	if got.Textures != nil {
		t.Errorf("expected no textures when none stored, got %+v", got.Textures)
	}
}

func TestStore_UpsertTextureHash(t *testing.T) {
	s := setupStore(t)

	hash := "abc12345"
	if err := s.UpsertTextureHash(hash); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second upsert should not error
	if err := s.UpsertTextureHash(hash); err != nil {
		t.Fatalf("unexpected error on duplicate: %v", err)
	}

	t.Cleanup(func() {
		db := setupRawDB(t)
		db.Exec(context.Background(), "DELETE FROM textures WHERE hash = 'abc12345'")
	})
}

func TestStore_UpsertTextures(t *testing.T) {
	s := setupStore(t)

	// Player must exist first
	if err := s.UpsertPlayer(testPlayer, false); err != nil {
		t.Fatalf("failed to upsert player: %v", err)
	}

	skin := &Texture{URL: "http://textures.minecraft.net/texture/skin123"}
	cape := &Texture{URL: "http://textures.minecraft.net/texture/cape456"}

	if err := s.UpsertTextureHash(skin.Hash()); err != nil {
		t.Fatalf("failed to upsert skin hash: %v", err)
	}
	if err := s.UpsertTextureHash(cape.Hash()); err != nil {
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

	if err := s.UpsertTextureHash(skin.Hash()); err != nil {
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

	// Verify at the layer that actually enforces it: the absent cape must
	// be NULL in the row, never "" — the CHECK constraint on this column
	// would reject "" outright, but a regression that quietly wrote NULL
	// as a different sentinel wouldn't trip that constraint.
	db := setupRawDB(t)
	var cape *string
	err := db.QueryRow(context.Background(),
		"SELECT cape FROM player_textures WHERE player_id = $1", testPlayer.ID).Scan(&cape)
	if err != nil {
		t.Fatalf("failed to query player_textures: %v", err)
	}
	if cape != nil {
		t.Errorf("expected cape to be NULL, got %q", *cape)
	}
}

func TestStore_UpsertTextureHash_EmptyHash_RejectedByCheckConstraint(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertTextureHash(""); err == nil {
		t.Error("expected the textures_hash_not_empty CHECK constraint to reject an empty hash")
	}
}

func TestStore_IsTextureInS3_Exists(t *testing.T) {
	client := setupMockS3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	s := &store{s3: client}
	exists, err := s.IsTextureInS3("mockhash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected texture to exist")
	}
}

func TestStore_IsTextureInS3_NotFound(t *testing.T) {
	client := setupMockS3(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		// Standard AWS SDK behavior expects this XML format to map to "NoSuchKey"
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code></Error>`))
	})

	s := &store{s3: client}
	exists, err := s.IsTextureInS3("mockhash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected texture to not exist")
	}
}

func TestStore_PutTextureInS3(t *testing.T) {
	client := setupMockS3(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type image/png, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	})

	s := &store{s3: client}
	err := s.PutTextureInS3("mockhash", nil) // Body is irrelevant for mocked http server
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// testGeyserPlayer's UUID is intentionally left unset: GetGeyserPlayerByGamertag
// derives it from XUID rather than reading it from the database.
var testGeyserPlayer = &GeyserPlayer{Gamertag: "Notch", XUID: 2535457445285308}

func TestStore_UpsertGeyserPlayer_Insert(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetGeyserPlayerByGamertag(testGeyserPlayer.Gamertag)
	if err != nil {
		t.Fatalf("failed to get geyser player: %v", err)
	}
	if got.XUID != testGeyserPlayer.XUID {
		t.Errorf("expected xuid %d, got %d", testGeyserPlayer.XUID, got.XUID)
	}
	if got.UUID != xuidToUUID(testGeyserPlayer.XUID) {
		t.Errorf("expected derived UUID %s, got %s", xuidToUUID(testGeyserPlayer.XUID), got.UUID)
	}
}

func TestStore_UpsertGeyserPlayer_UpdateLastSeen(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	got1, _ := s.GetGeyserPlayerByGamertag(testGeyserPlayer.Gamertag)
	firstSeen := got1.FirstSeen

	time.Sleep(10 * time.Millisecond)

	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	got2, _ := s.GetGeyserPlayerByGamertag(testGeyserPlayer.Gamertag)
	if got2.FirstSeen != firstSeen {
		t.Error("first_seen should not change on upsert")
	}
	if got2.LastSeen <= got1.LastSeen {
		t.Error("last_seen should be updated on upsert")
	}
}

func TestStore_UpsertGeyserPlayer_GamertagChange(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	renamed := &GeyserPlayer{Gamertag: "NotchRenamed", XUID: testGeyserPlayer.XUID}
	if err := s.UpsertGeyserPlayer(renamed); err != nil {
		t.Fatalf("upsert with new gamertag failed: %v", err)
	}

	got, err := s.GetGeyserPlayerByGamertag("NotchRenamed")
	if err != nil {
		t.Fatalf("failed to get geyser player by new gamertag: %v", err)
	}
	if got.XUID != testGeyserPlayer.XUID {
		t.Errorf("expected xuid %d, got %d", testGeyserPlayer.XUID, got.XUID)
	}

	if _, err := s.GetGeyserPlayerByGamertag(testGeyserPlayer.Gamertag); err == nil {
		t.Error("expected the old gamertag to no longer resolve, since xuid is the stable key")
	}
}

func TestStore_GetGeyserPlayerByGamertag_NotFound(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetGeyserPlayerByGamertag("nonexistent_gamertag_xyz")
	if err == nil {
		t.Error("expected error for unknown gamertag")
	}
}

func TestStore_GetGeyserPlayerByXUID(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetGeyserPlayerByXUID(testGeyserPlayer.XUID)
	if err != nil {
		t.Fatalf("failed to get geyser player: %v", err)
	}
	if got.Gamertag != testGeyserPlayer.Gamertag {
		t.Errorf("expected gamertag %s, got %s", testGeyserPlayer.Gamertag, got.Gamertag)
	}
	if got.UUID != xuidToUUID(testGeyserPlayer.XUID) {
		t.Errorf("expected derived UUID %s, got %s", xuidToUUID(testGeyserPlayer.XUID), got.UUID)
	}
}

func TestStore_GetGeyserPlayerByXUID_NotFound(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetGeyserPlayerByXUID(1234567890)
	if err == nil {
		t.Error("expected error for unknown xuid")
	}
}

// TestStore_GetGeyserPlayerByGamertag_HandlesGamertagReuse guards against a
// released gamertag being picked up by a different Xbox account: nothing
// stops two different xuids from carrying the same gamertag at once (the
// old xuid's row simply hasn't gone stale yet), so the lookup must resolve
// to one row deterministically instead of erroring on multiple matches.
func TestStore_GetGeyserPlayerByGamertag_HandlesGamertagReuse(t *testing.T) {
	s := setupStore(t)
	const otherXUID int64 = 9999999999999999

	older := &GeyserPlayer{Gamertag: "DupTag", XUID: testGeyserPlayer.XUID}
	if err := s.UpsertGeyserPlayer(older); err != nil {
		t.Fatalf("failed to insert older player: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	newer := &GeyserPlayer{Gamertag: "DupTag", XUID: otherXUID}
	if err := s.UpsertGeyserPlayer(newer); err != nil {
		t.Fatalf("failed to insert newer player: %v", err)
	}

	got, err := s.GetGeyserPlayerByGamertag("DupTag")
	if err != nil {
		t.Fatalf("unexpected error on duplicate gamertag: %v", err)
	}
	if got.XUID != otherXUID {
		t.Errorf("expected the most recently seen xuid %d, got %d", otherXUID, got.XUID)
	}

	t.Cleanup(func() {
		db := setupRawDB(t)
		db.Exec(context.Background(), "DELETE FROM geyser_players WHERE xuid = $1", otherXUID)
	})
}

func TestStore_UpsertGeyserSkin_Insert(t *testing.T) {
	s := setupStore(t)
	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("failed to seed geyser player: %v", err)
	}

	skin := &GeyserSkin{Hash: "abc123", IsSteve: true, Signature: "sig123", TextureID: "def456", Value: "base64value"}
	if err := s.UpsertGeyserSkin(testGeyserPlayer.XUID, skin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetGeyserSkin(testGeyserPlayer.XUID)
	if err != nil {
		t.Fatalf("failed to get geyser skin: %v", err)
	}
	if got.Hash != "abc123" || got.TextureID != "def456" || got.Value != "base64value" {
		t.Errorf("unexpected skin: %+v", got)
	}
	if got.Signature != "sig123" {
		t.Errorf("expected signature sig123, got %s", got.Signature)
	}
	if !got.IsSteve {
		t.Error("expected IsSteve to be true")
	}
}

func TestStore_UpsertGeyserSkin_WithoutKnownGeyserPlayer(t *testing.T) {
	// A xuid can reach GetGeyserSkin without ever being resolved through
	// GetGeyserXUID/UpsertGeyserPlayer first (e.g. a caller that already has
	// the xuid from elsewhere), so geyser_player_textures must not require a
	// geyser_players row to already exist.
	s := setupStore(t)
	const unknownXUID int64 = 1111111111111111

	skin := &GeyserSkin{Hash: "abc123", TextureID: "def456", Value: "base64value"}
	if err := s.UpsertGeyserSkin(unknownXUID, skin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetGeyserSkin(unknownXUID)
	if err != nil {
		t.Fatalf("failed to get geyser skin: %v", err)
	}
	if got.Hash != "abc123" {
		t.Errorf("expected abc123, got %s", got.Hash)
	}

	t.Cleanup(func() {
		db := setupRawDB(t)
		db.Exec(context.Background(), "DELETE FROM geyser_player_textures WHERE xuid = $1", unknownXUID)
	})
}

func TestStore_UpsertGeyserSkin_NoSignature(t *testing.T) {
	s := setupStore(t)
	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("failed to seed geyser player: %v", err)
	}

	skin := &GeyserSkin{Hash: "abc123", TextureID: "def456", Value: "base64value"}
	if err := s.UpsertGeyserSkin(testGeyserPlayer.XUID, skin); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetGeyserSkin(testGeyserPlayer.XUID)
	if err != nil {
		t.Fatalf("failed to get geyser skin: %v", err)
	}
	if got.Signature != "" {
		t.Errorf("expected empty signature, got %s", got.Signature)
	}
}

func TestStore_UpsertGeyserSkin_UpdateLastSeen(t *testing.T) {
	s := setupStore(t)
	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("failed to seed geyser player: %v", err)
	}

	skin := &GeyserSkin{Hash: "abc123", TextureID: "def456", Value: "base64value"}
	if err := s.UpsertGeyserSkin(testGeyserPlayer.XUID, skin); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	got1, _ := s.GetGeyserSkin(testGeyserPlayer.XUID)
	firstSeen := got1.FirstSeen

	time.Sleep(10 * time.Millisecond)

	if err := s.UpsertGeyserSkin(testGeyserPlayer.XUID, skin); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	got2, _ := s.GetGeyserSkin(testGeyserPlayer.XUID)
	if got2.FirstSeen != firstSeen {
		t.Error("first_seen should not change on upsert")
	}
	if got2.LastSeen <= got1.LastSeen {
		t.Error("last_seen should be updated on upsert")
	}
}

func TestStore_GetGeyserSkin_ReturnsMostRecent(t *testing.T) {
	s := setupStore(t)
	if err := s.UpsertGeyserPlayer(testGeyserPlayer); err != nil {
		t.Fatalf("failed to seed geyser player: %v", err)
	}

	older := &GeyserSkin{Hash: "older-hash", TextureID: "id1", Value: "val1"}
	if err := s.UpsertGeyserSkin(testGeyserPlayer.XUID, older); err != nil {
		t.Fatalf("failed to insert older skin: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	newer := &GeyserSkin{Hash: "newer-hash", TextureID: "id2", Value: "val2"}
	if err := s.UpsertGeyserSkin(testGeyserPlayer.XUID, newer); err != nil {
		t.Fatalf("failed to insert newer skin: %v", err)
	}

	got, err := s.GetGeyserSkin(testGeyserPlayer.XUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Hash != "newer-hash" {
		t.Errorf("expected newer-hash, got %s", got.Hash)
	}
}

func TestStore_GetGeyserSkin_NotFound(t *testing.T) {
	s := setupStore(t)

	_, err := s.GetGeyserSkin(999999999999999)
	if err == nil {
		t.Error("expected error for unknown xuid")
	}
}
