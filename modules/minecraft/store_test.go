package minecraft

import (
	"context"
	"os"
	"testing"
	"time"

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
		rdb.Del(context.Background(), "player:853c80ef3c3749fdaa49938b674adae6", "player:jeb_")
		db.Close()
		rdb.Close()
	})

	return NewStore(db, rdb)
}

var testPlayer = &MCPlayer{
	ID:   "853c80ef3c3749fdaa49938b674adae6",
	Name: "jeb_",
}

// --- Tests ---

func TestStore_UpsertPlayer_Insert(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer); err != nil {
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

	if err := s.UpsertPlayer(testPlayer); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}

	got1, _ := s.GetPlayerByUUID(testPlayer.ID)
	firstSeen := got1.FirstSeen

	// Small sleep to ensure last_seen differs
	time.Sleep(10 * time.Millisecond)

	if err := s.UpsertPlayer(testPlayer); err != nil {
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

func TestStore_UpsertPlayer_NameHistory(t *testing.T) {
	s := setupStore(t)

	if err := s.UpsertPlayer(testPlayer); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Simulate name change
	renamed := &MCPlayer{ID: testPlayer.ID, Name: "jeb_renamed"}
	if err := s.UpsertPlayer(renamed); err != nil {
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
