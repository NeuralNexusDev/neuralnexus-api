package minecraft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func mcMojangStore(db *pgxpool.Pool) *mojangStore {
	return &mojangStore{db: db, texture: &textureStore{db: db}}
}

func mcGeyserStore(db *pgxpool.Pool) *geyserStore {
	return &geyserStore{db: db, texture: &textureStore{db: db}}
}

func mcTextureStore(db *pgxpool.Pool) *textureStore {
	return &textureStore{db: db}
}

func mcFakeS3(t *testing.T, handler http.HandlerFunc) *s3.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(srv.URL)
		o.UsePathStyle = true
		o.RetryMaxAttempts = 1
	})
}

func mcLiveRedis(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set; skipping live-Redis test")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("failed to parse TEST_REDIS_URL: %v", err)
	}
	c := redis.NewClient(opts)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

var mcXUIDCounter = time.Now().UnixNano()

func mcUniqueXUID() int64 { return atomic.AddInt64(&mcXUIDCounter, 1) }

func mcUniqueHash(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(uuid.New().String(), "-", ""))
}

func mcSeedPlayerWithHashes(t *testing.T, db *pgxpool.Pool, hashes ...string) string {
	t.Helper()
	id := uuid.New().String()
	if err := mcMojangStore(db).UpsertPlayer(mcCtx, &MojangPlayer{ID: id, Name: "Tex_" + mcUniqueHash("n")}, false); err != nil {
		t.Fatalf("seed UpsertPlayer() error = %v", err)
	}
	for _, h := range hashes {
		if err := mcTextureStore(db).UpsertTextureHash(mcCtx, h); err != nil {
			t.Fatalf("seed UpsertTextureHash() error = %v", err)
		}
	}
	return id
}

func mcTextureRows(t *testing.T, db *pgxpool.Pool, id string) (count int, skin, cape *string) {
	t.Helper()
	if err := db.QueryRow(mcCtx, "SELECT count(*) FROM player_textures WHERE player_id = $1", id).Scan(&count); err != nil {
		t.Fatalf("count player_textures error = %v", err)
	}
	if count == 0 {
		return count, nil, nil
	}
	if err := db.QueryRow(mcCtx, "SELECT skin, cape FROM player_textures WHERE player_id = $1 LIMIT 1", id).Scan(&skin, &cape); err != nil {
		t.Fatalf("select player_textures error = %v", err)
	}
	return count, skin, cape
}

func TestST02to04_GetPlayerByUUID(t *testing.T) {
	t.Run("ST-02_Unreachable", func(t *testing.T) {
		s := mcMojangStore(testutil.UnreachablePool(t))
		_, err := s.GetPlayerByUUID(mcCtx, uuid.New().String())
		if err == nil || errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want a raw connection error", err)
		}
	})

	t.Run("ST-03_Live_Found", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		id := uuid.New().String()
		if err := s.UpsertPlayer(mcCtx, &MojangPlayer{ID: id, Name: "Steve"}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByUUID(mcCtx, id)
		if err != nil {
			t.Fatalf("GetPlayerByUUID() error = %v", err)
		}
		if got.ID != id || got.Name != "Steve" {
			t.Errorf("got %+v, want id=%s name=Steve", got, id)
		}
	})

	t.Run("ST-04_Live_NotFound", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		_, err := s.GetPlayerByUUID(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST05to07_GetPlayerByName(t *testing.T) {
	t.Run("ST-05_Unreachable", func(t *testing.T) {
		s := mcMojangStore(testutil.UnreachablePool(t))
		_, err := s.GetPlayerByName(mcCtx, "nobody")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-06_Live_Found", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		id := uuid.New().String()
		name := "Name_" + mcUniqueHash("n")
		if err := s.UpsertPlayer(mcCtx, &MojangPlayer{ID: id, Name: name}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByName(mcCtx, name)
		if err != nil {
			t.Fatalf("GetPlayerByName() error = %v", err)
		}
		if got.ID != id {
			t.Errorf("got %+v, want id=%s", got, id)
		}
	})

	t.Run("ST-07_Live_NotFound", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		_, err := s.GetPlayerByName(mcCtx, "definitely-not-a-real-name-"+mcUniqueHash("n"))
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST08to11_GetProfileByUUID(t *testing.T) {
	t.Run("ST-08_Unreachable", func(t *testing.T) {
		s := mcMojangStore(testutil.UnreachablePool(t))
		_, err := s.GetProfileByUUID(mcCtx, uuid.New().String())
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-09_Live_WithTextures", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		skin, cape := mcUniqueHash("skin"), mcUniqueHash("cape")
		id := mcSeedPlayerWithHashes(t, db, skin, cape)
		value := &TexturesValue{ProfileID: id, Timestamp: time.Now().UnixMilli(), Textures: Textures{SKIN: &Texture{URL: "http://t/" + skin}, CAPE: &Texture{URL: "http://t/" + cape}}}
		if err := mcTextureStore(db).UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("seed UpsertTextures() error = %v", err)
		}
		got, err := mcMojangStore(db).GetProfileByUUID(mcCtx, id)
		if err != nil {
			t.Fatalf("GetProfileByUUID() error = %v", err)
		}
		if got.Textures == nil || got.Textures.Textures.SKIN == nil || !strings.HasSuffix(got.Textures.Textures.SKIN.URL, skin) || got.Textures.Textures.CAPE == nil || !strings.HasSuffix(got.Textures.Textures.CAPE.URL, cape) {
			t.Errorf("Textures = %+v, want skin %q and cape %q", got.Textures, skin, cape)
		}
	})

	t.Run("ST-10_Live_NoTextures", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		id := mcSeedPlayerWithHashes(t, db)
		got, err := mcMojangStore(db).GetProfileByUUID(mcCtx, id)
		if err != nil {
			t.Fatalf("GetProfileByUUID() error = %v", err)
		}
		if got.Textures != nil {
			t.Errorf("Textures = %+v, want nil", got.Textures)
		}
	})

	t.Run("ST-11_Live_NotFound", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		_, err := s.GetProfileByUUID(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST12to14_GetTextures(t *testing.T) {
	t.Run("ST-12_Unreachable", func(t *testing.T) {
		s := mcTextureStore(testutil.UnreachablePool(t))
		_, err := s.GetTextures(mcCtx, JavaPlayerTexturesTable, uuid.New().String(), "Steve")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-13_Live_Found", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		skin, cape := mcUniqueHash("skin"), mcUniqueHash("cape")
		id := mcSeedPlayerWithHashes(t, db, skin, cape)
		value := &TexturesValue{ProfileID: id, Timestamp: time.Now().UnixMilli(), Textures: Textures{SKIN: &Texture{URL: "http://t/" + skin}, CAPE: &Texture{URL: "http://t/" + cape}}}
		if err := mcTextureStore(db).UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("seed UpsertTextures() error = %v", err)
		}
		got, err := mcTextureStore(db).GetTextures(mcCtx, JavaPlayerTexturesTable, id, "Steve")
		if err != nil {
			t.Fatalf("GetTextures() error = %v", err)
		}
		if got == nil || got.Textures.SKIN == nil || !strings.HasSuffix(got.Textures.SKIN.URL, skin) || got.Textures.CAPE == nil || !strings.HasSuffix(got.Textures.CAPE.URL, cape) {
			t.Errorf("got %+v, want skin %q and cape %q", got, skin, cape)
		}
	})

	t.Run("ST-14_Live_NoRow", func(t *testing.T) {
		got, err := mcTextureStore(testutil.LivePool(t, "archive")).GetTextures(mcCtx, JavaPlayerTexturesTable, uuid.New().String(), "Steve")
		if got != nil || err != nil {
			t.Errorf("got (%+v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("ST-79_Live_SkinOnlyHasNilCape", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		skin := mcUniqueHash("skin")
		id := mcSeedPlayerWithHashes(t, db, skin)
		value := &TexturesValue{ProfileID: id, Timestamp: time.Now().UnixMilli(), Textures: Textures{SKIN: &Texture{URL: "http://t/" + skin}}}
		if err := mcTextureStore(db).UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("seed UpsertTextures() error = %v", err)
		}
		got, err := mcTextureStore(db).GetTextures(mcCtx, JavaPlayerTexturesTable, id, "Steve")
		if err != nil {
			t.Fatalf("GetTextures() error = %v", err)
		}
		if got == nil || got.Textures.SKIN == nil || !strings.HasSuffix(got.Textures.SKIN.URL, skin) || got.Textures.CAPE != nil {
			t.Errorf("got %+v, want skin %q and nil CAPE", got, skin)
		}
	})
}

func TestST15to19_UpsertPlayer(t *testing.T) {
	t.Run("ST-15_Unreachable", func(t *testing.T) {
		s := mcMojangStore(testutil.UnreachablePool(t))
		err := s.UpsertPlayer(mcCtx, &MojangPlayer{ID: uuid.New().String(), Name: "Steve"}, false)
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-16_Live_UpdateProfileTrue", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		id := uuid.New().String()
		player := &MojangPlayer{ID: id, Name: "Alex", Legacy: true, Demo: true, ProfileActions: []string{"FORCED_NAME_CHANGE"}}
		if err := mcMojangStore(db).UpsertPlayer(mcCtx, player, true); err != nil {
			t.Fatalf("UpsertPlayer() error = %v", err)
		}
		var legacy, demo bool
		var actions []string
		if err := db.QueryRow(mcCtx, "SELECT legacy, demo, profile_actions FROM players WHERE id = $1", id).Scan(&legacy, &demo, &actions); err != nil {
			t.Fatalf("select players error = %v", err)
		}
		if !legacy || !demo || len(actions) != 1 || actions[0] != "FORCED_NAME_CHANGE" {
			t.Errorf("row = legacy %v demo %v actions %v, want true true [FORCED_NAME_CHANGE]", legacy, demo, actions)
		}
		var names int
		if err := db.QueryRow(mcCtx, "SELECT count(*) FROM player_names WHERE player_id = $1 AND name = $2", id, "Alex").Scan(&names); err != nil {
			t.Fatalf("count player_names error = %v", err)
		}
		if names != 1 {
			t.Errorf("name history rows = %d, want 1", names)
		}
	})

	t.Run("ST-17_Live_UpdateProfileFalseDefaults", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		id := uuid.New().String()
		if err := s.UpsertPlayer(mcCtx, &MojangPlayer{ID: id, Name: "Steve", Legacy: true}, false); err != nil {
			t.Fatalf("UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByUUID(mcCtx, id)
		if err != nil {
			t.Fatalf("GetPlayerByUUID() error = %v", err)
		}
		if got.Legacy || got.Demo {
			t.Errorf("got %+v, want Legacy=false Demo=false regardless of input", got)
		}
	})

	t.Run("ST-18_Live_UpsertTwiceUpdatesInPlace", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		id := uuid.New().String()
		if err := s.UpsertPlayer(mcCtx, &MojangPlayer{ID: id, Name: "First"}, false); err != nil {
			t.Fatalf("first UpsertPlayer() error = %v", err)
		}
		if err := s.UpsertPlayer(mcCtx, &MojangPlayer{ID: id, Name: "Second"}, false); err != nil {
			t.Fatalf("second UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByUUID(mcCtx, id)
		if err != nil {
			t.Fatalf("GetPlayerByUUID() error = %v", err)
		}
		if got.Name != "Second" {
			t.Errorf("Name = %q, want Second", got.Name)
		}
	})

	t.Run("ST-19_Live_ConcurrentUpsertsSameID", func(t *testing.T) {
		s := mcMojangStore(testutil.LivePool(t, "archive"))
		id := uuid.New().String()
		const n = 50
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = s.UpsertPlayer(mcCtx, &MojangPlayer{ID: id, Name: fmt.Sprintf("Name%d", i)}, false)
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d: UpsertPlayer() error = %v, want nil (ON CONFLICT should absorb the race)", i, err)
			}
		}
		if _, err := s.GetPlayerByUUID(mcCtx, id); err != nil {
			t.Errorf("GetPlayerByUUID() after concurrent upserts error = %v, want exactly one surviving row", err)
		}
	})
}

func TestST20to23_UpsertTextures(t *testing.T) {
	t.Run("ST-20_Unreachable", func(t *testing.T) {
		s := mcTextureStore(testutil.UnreachablePool(t))
		err := s.UpsertTextures(mcCtx, JavaPlayerTexturesTable, &TexturesValue{ProfileID: uuid.New().String(), Textures: Textures{SKIN: &Texture{URL: "http://x/hash"}}})
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-21_Live_SkinCapeSlimModel", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		skin, cape := mcUniqueHash("skin"), mcUniqueHash("cape")
		id := mcSeedPlayerWithHashes(t, db, skin, cape)
		value := &TexturesValue{ProfileID: id, Timestamp: time.Now().UnixMilli(), Textures: Textures{
			SKIN: &Texture{URL: "http://t/" + skin, Metadata: &Metadata{Model: SLIM}},
			CAPE: &Texture{URL: "http://t/" + cape},
		}}
		if err := mcTextureStore(db).UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("UpsertTextures() error = %v", err)
		}
		var gotSkin, gotCape, model string
		if err := db.QueryRow(mcCtx, "SELECT skin, cape, model FROM player_textures WHERE player_id = $1", id).Scan(&gotSkin, &gotCape, &model); err != nil {
			t.Fatalf("select player_textures error = %v", err)
		}
		if gotSkin != skin || gotCape != cape || model != string(SLIM) {
			t.Errorf("row = (%q, %q, %q), want (%q, %q, %q)", gotSkin, gotCape, model, skin, cape, SLIM)
		}
	})

	t.Run("ST-22_Live_SameKeyTwice", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		skin := mcUniqueHash("skin")
		id := mcSeedPlayerWithHashes(t, db, skin)
		s := mcTextureStore(db)
		value := &TexturesValue{ProfileID: id, Timestamp: 1000, Textures: Textures{SKIN: &Texture{URL: "http://t/" + skin}}}
		if err := s.UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("first UpsertTextures() error = %v", err)
		}
		value.Timestamp = 2000
		if err := s.UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("second UpsertTextures() error = %v", err)
		}
		var count int
		var lastSeen int64
		if err := db.QueryRow(mcCtx, "SELECT count(*), max(last_seen) FROM player_textures WHERE player_id = $1", id).Scan(&count, &lastSeen); err != nil {
			t.Fatalf("select player_textures error = %v", err)
		}
		if count != 1 || lastSeen != 2000 {
			t.Errorf("rows = %d, last_seen = %d, want 1 and 2000", count, lastSeen)
		}
	})

	t.Run("ST-23_Live_ConcurrentUpsertsSameKey", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		s := mcTextureStore(db)
		const n = 50
		for trial := 0; trial < 3; trial++ {
			skin := mcUniqueHash("skin")
			id := mcSeedPlayerWithHashes(t, db, skin)
			var wg sync.WaitGroup
			errs := make([]error, n)
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					value := &TexturesValue{ProfileID: id, Timestamp: int64(1000 + i), Textures: Textures{SKIN: &Texture{URL: "http://t/" + skin}}}
					errs[i] = s.UpsertTextures(mcCtx, JavaPlayerTexturesTable, value)
				}(i)
			}
			wg.Wait()
			for i, err := range errs {
				if err != nil {
					t.Errorf("trial %d goroutine %d: UpsertTextures() error = %v, want nil", trial, i, err)
				}
			}
			if count, _, _ := mcTextureRows(t, db, id); count != 1 {
				t.Errorf("trial %d: rows = %d, want 1", trial, count)
			}
		}
	})

	t.Run("ST-77_Live_SkinOnlyStoresNullCape", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		skin := mcUniqueHash("skin")
		id := mcSeedPlayerWithHashes(t, db, skin)
		value := &TexturesValue{ProfileID: id, Timestamp: time.Now().UnixMilli(), Textures: Textures{SKIN: &Texture{URL: "http://t/" + skin}}}
		if err := mcTextureStore(db).UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("UpsertTextures() error = %v", err)
		}
		count, gotSkin, gotCape := mcTextureRows(t, db, id)
		if count != 1 || gotSkin == nil || *gotSkin != skin || gotCape != nil {
			t.Errorf("rows = %d, skin = %v, cape = %v, want 1 row with skin %q and NULL cape", count, gotSkin, gotCape, skin)
		}
	})

	t.Run("ST-78_Live_CapeOnlyStoresNullSkin", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		cape := mcUniqueHash("cape")
		id := mcSeedPlayerWithHashes(t, db, cape)
		value := &TexturesValue{ProfileID: id, Timestamp: time.Now().UnixMilli(), Textures: Textures{CAPE: &Texture{URL: "http://t/" + cape}}}
		if err := mcTextureStore(db).UpsertTextures(mcCtx, JavaPlayerTexturesTable, value); err != nil {
			t.Fatalf("UpsertTextures() error = %v", err)
		}
		count, gotSkin, gotCape := mcTextureRows(t, db, id)
		if count != 1 || gotSkin != nil || gotCape == nil || *gotCape != cape {
			t.Errorf("rows = %d, skin = %v, cape = %v, want 1 row with NULL skin and cape %q", count, gotSkin, gotCape, cape)
		}
	})
}

func TestST26to28_UpsertTextureHash(t *testing.T) {
	t.Run("ST-26_Unreachable", func(t *testing.T) {
		s := mcTextureStore(testutil.UnreachablePool(t))
		if err := s.UpsertTextureHash(mcCtx, "abc"); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-27_Live_New", func(t *testing.T) {
		s := mcTextureStore(testutil.LivePool(t, "archive"))
		if err := s.UpsertTextureHash(mcCtx, mcUniqueHash("hash")); err != nil {
			t.Errorf("UpsertTextureHash() error = %v", err)
		}
	})

	t.Run("ST-28_Live_Duplicate", func(t *testing.T) {
		s := mcTextureStore(testutil.LivePool(t, "archive"))
		hash := mcUniqueHash("hash")
		if err := s.UpsertTextureHash(mcCtx, hash); err != nil {
			t.Fatalf("first UpsertTextureHash() error = %v", err)
		}
		if err := s.UpsertTextureHash(mcCtx, hash); err != nil {
			t.Errorf("second UpsertTextureHash() error = %v, want nil (ON CONFLICT DO NOTHING)", err)
		}
	})

	t.Run("ST-76_Live_ConcurrentUpsertsSameHash", func(t *testing.T) {
		s := mcTextureStore(testutil.LivePool(t, "archive"))
		hash := mcUniqueHash("hash")
		const n = 50
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = s.UpsertTextureHash(mcCtx, hash)
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d: UpsertTextureHash() error = %v, want nil (ON CONFLICT should absorb the race)", i, err)
			}
		}
	})
}

func TestST29to32_GetPlayerFromCache(t *testing.T) {
	t.Run("ST-29_Unreachable", func(t *testing.T) {
		s := &cacheStore{rdb: testutil.UnreachableRedis(t)}
		_, err := s.GetPlayerFromCache(mcCtx, "key")
		if err == nil || errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want a raw connection error", err)
		}
	})

	t.Run("ST-30_Live_Hit", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		id := uuid.New().String()
		p := &MojangPlayer{ID: id, Name: "Steve_" + mcUniqueHash("n")}
		if err := s.SetPlayerInCache(mcCtx, p); err != nil {
			t.Fatalf("seed SetPlayerInCache() error = %v", err)
		}
		got, err := s.GetPlayerFromCache(mcCtx, id)
		if err != nil || got.ID != id {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("ST-31_Live_Miss", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		_, err := s.GetPlayerFromCache(mcCtx, "no-such-key-"+mcUniqueHash("k"))
		if !errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want %v", err, ErrCacheMiss)
		}
	})

	t.Run("ST-32_Live_MalformedJSON", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		key := "malformed-" + mcUniqueHash("k")
		if err := s.rdb.Set(context.Background(), CachePlayer+key, "not json", time.Minute).Err(); err != nil {
			t.Fatalf("seed Set() error = %v", err)
		}
		_, err := s.GetPlayerFromCache(mcCtx, key)
		if err == nil {
			t.Error("expected an unmarshal error")
		}
	})
}

func TestST33to34_SetPlayerInCache(t *testing.T) {
	t.Run("ST-33_Unreachable", func(t *testing.T) {
		s := &cacheStore{rdb: testutil.UnreachableRedis(t)}
		if err := s.SetPlayerInCache(mcCtx, &MojangPlayer{ID: "id", Name: "name"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-34_Live_SetsBothKeys", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		id := uuid.New().String()
		name := "Steve_" + mcUniqueHash("n")
		p := &MojangPlayer{ID: id, Name: name}
		if err := s.SetPlayerInCache(mcCtx, p); err != nil {
			t.Fatalf("SetPlayerInCache() error = %v", err)
		}
		byID, err := s.GetPlayerFromCache(mcCtx, id)
		if err != nil || byID.ID != id {
			t.Errorf("GetPlayerFromCache(id) = (%+v, %v)", byID, err)
		}
		byName, err := s.GetPlayerFromCache(mcCtx, name)
		if err != nil || byName.ID != id {
			t.Errorf("GetPlayerFromCache(name) = (%+v, %v)", byName, err)
		}
	})
}

func TestST35to37_GetProfileFromCache(t *testing.T) {
	t.Run("ST-35_Unreachable", func(t *testing.T) {
		s := &cacheStore{rdb: testutil.UnreachableRedis(t)}
		_, err := s.GetProfileFromCache(mcCtx, "id")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-36_Live_Hit", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		id := uuid.New().String()
		if err := s.SetProfileInCache(mcCtx, &MojangProfile{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("seed SetProfileInCache() error = %v", err)
		}
		got, err := s.GetProfileFromCache(mcCtx, id)
		if err != nil || got.ID != id {
			t.Errorf("got (%+v, %v), want cached profile", got, err)
		}
	})

	t.Run("ST-37_Live_Miss", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		_, err := s.GetProfileFromCache(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want %v", err, ErrCacheMiss)
		}
	})
}

func TestST38to39_SetProfileInCache(t *testing.T) {
	t.Run("ST-38_Unreachable", func(t *testing.T) {
		s := &cacheStore{rdb: testutil.UnreachableRedis(t)}
		if err := s.SetProfileInCache(mcCtx, &MojangProfile{ID: "id"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-39_Live_ReadableBack", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		id := uuid.New().String()
		if err := s.SetProfileInCache(mcCtx, &MojangProfile{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("SetProfileInCache() error = %v", err)
		}
		got, err := s.GetProfileFromCache(mcCtx, id)
		if err != nil || got.Name != "Steve" {
			t.Errorf("got (%+v, %v)", got, err)
		}
	})
}

func TestST40to42_GetSignedProfileFromCache(t *testing.T) {
	t.Run("ST-40_Unreachable", func(t *testing.T) {
		s := &cacheStore{rdb: testutil.UnreachableRedis(t)}
		_, err := s.GetSignedProfileFromCache(mcCtx, "id")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-41_Live_Hit", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		id := uuid.New().String()
		if err := s.SetSignedProfileInCache(mcCtx, &MojangPlayer{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("seed SetSignedProfileInCache() error = %v", err)
		}
		got, err := s.GetSignedProfileFromCache(mcCtx, id)
		if err != nil || got.ID != id {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("ST-42_Live_Miss", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		_, err := s.GetSignedProfileFromCache(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want %v", err, ErrCacheMiss)
		}
	})
}

func TestST43to44_SetSignedProfileInCache(t *testing.T) {
	t.Run("ST-43_Unreachable", func(t *testing.T) {
		s := &cacheStore{rdb: testutil.UnreachableRedis(t)}
		if err := s.SetSignedProfileInCache(mcCtx, &MojangPlayer{ID: "id"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-44_Live_ReadableBack", func(t *testing.T) {
		s := &cacheStore{rdb: mcLiveRedis(t)}
		id := uuid.New().String()
		if err := s.SetSignedProfileInCache(mcCtx, &MojangPlayer{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("SetSignedProfileInCache() error = %v", err)
		}
		got, err := s.GetSignedProfileFromCache(mcCtx, id)
		if err != nil || got.Name != "Steve" {
			t.Errorf("got (%+v, %v)", got, err)
		}
	})
}

func TestST45to47_IsTextureInS3(t *testing.T) {
	t.Run("ST-45_Exists", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		s := &s3store{s3: s3c}
		got, err := s.IsTextureInS3(mcCtx, "hash")
		if err != nil || !got {
			t.Errorf("got (%v, %v), want (true, nil)", got, err)
		}
	})

	t.Run("ST-46_NotFound", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
		s := &s3store{s3: s3c}
		got, err := s.IsTextureInS3(mcCtx, "hash")
		if err != nil || got {
			t.Errorf("got (%v, %v), want (false, nil)", got, err)
		}
	})

	t.Run("ST-47_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &s3store{s3: s3c}
		got, err := s.IsTextureInS3(mcCtx, "hash")
		if err == nil || got {
			t.Errorf("got (%v, %v), want (false, non-nil error)", got, err)
		}
	})
}

func TestST48to50_GetGeyserPlayerByGamertag(t *testing.T) {
	t.Run("ST-48_Unreachable", func(t *testing.T) {
		s := mcGeyserStore(testutil.UnreachablePool(t))
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-49_Live_Found", func(t *testing.T) {
		s := mcGeyserStore(testutil.LivePool(t, "archive"))
		xuid := mcUniqueXUID()
		gamertag := "gt-" + mcUniqueHash("g")
		if err := s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{ID: xuidToUUID(xuid), XUID: xuid, Name: gamertag}); err != nil {
			t.Fatalf("seed UpsertGeyserPlayer() error = %v", err)
		}
		got, err := s.GetGeyserPlayerByGamertag(mcCtx, gamertag)
		if err != nil {
			t.Fatalf("GetGeyserPlayerByGamertag() error = %v", err)
		}
		if got.ID != xuidToUUID(xuid) || got.XUID != xuid || got.Name != gamertag {
			t.Errorf("got %+v, want id=%s xuid=%d gamertag=%s", got, xuidToUUID(xuid), xuid, gamertag)
		}
	})

	t.Run("ST-50_Live_NotFound", func(t *testing.T) {
		s := mcGeyserStore(testutil.LivePool(t, "archive"))
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "no-such-gamertag-"+mcUniqueHash("g"))
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST51to53_GetGeyserPlayerByXUID(t *testing.T) {
	t.Run("ST-51_Unreachable", func(t *testing.T) {
		s := mcGeyserStore(testutil.UnreachablePool(t))
		_, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-52_Live_Found", func(t *testing.T) {
		s := mcGeyserStore(testutil.LivePool(t, "archive"))
		xuid := mcUniqueXUID()
		gamertag := "gt-" + mcUniqueHash("g")
		if err := s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{ID: xuidToUUID(xuid), XUID: xuid, Name: gamertag}); err != nil {
			t.Fatalf("seed UpsertGeyserPlayer() error = %v", err)
		}
		got, err := s.GetGeyserPlayerByXUID(mcCtx, xuid)
		if err != nil {
			t.Fatalf("GetGeyserPlayerByXUID() error = %v", err)
		}
		if got.ID != xuidToUUID(xuid) || got.XUID != xuid || got.Name != gamertag {
			t.Errorf("got %+v, want id=%s xuid=%d gamertag=%s", got, xuidToUUID(xuid), xuid, gamertag)
		}
	})

	t.Run("ST-53_Live_NotFound", func(t *testing.T) {
		s := mcGeyserStore(testutil.LivePool(t, "archive"))
		_, err := s.GetGeyserPlayerByXUID(mcCtx, mcUniqueXUID())
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST54to57_UpsertGeyserPlayer(t *testing.T) {
	t.Run("ST-54_Unreachable", func(t *testing.T) {
		s := mcGeyserStore(testutil.UnreachablePool(t))
		if err := s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{XUID: 1, Name: "Notch"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-55_Live_NewXUID", func(t *testing.T) {
		s := mcGeyserStore(testutil.LivePool(t, "archive"))
		xuid := mcUniqueXUID()
		if err := s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{ID: xuidToUUID(xuid), XUID: xuid, Name: "Fresh"}); err != nil {
			t.Fatalf("UpsertGeyserPlayer() error = %v", err)
		}
		got, err := s.GetGeyserPlayerByXUID(mcCtx, xuid)
		if err != nil {
			t.Fatalf("GetGeyserPlayerByXUID() error = %v", err)
		}
		if got.Name != "Fresh" {
			t.Errorf("Name = %q, want Fresh", got.Name)
		}
	})

	t.Run("ST-56_Live_SameXUIDTwice", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		s := mcGeyserStore(db)
		xuid := mcUniqueXUID()
		id := xuidToUUID(xuid)
		if err := s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{ID: id, XUID: xuid, Name: "First"}); err != nil {
			t.Fatalf("first UpsertGeyserPlayer() error = %v", err)
		}
		if err := s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{ID: id, XUID: xuid, Name: "Second"}); err != nil {
			t.Fatalf("second UpsertGeyserPlayer() error = %v", err)
		}
		var count int
		if err := db.QueryRow(mcCtx, "SELECT count(*) FROM geyser_players WHERE id = $1", id).Scan(&count); err != nil {
			t.Fatalf("count geyser_players error = %v", err)
		}
		got, err := s.GetGeyserPlayerByXUID(mcCtx, xuid)
		if err != nil {
			t.Fatalf("GetGeyserPlayerByXUID() error = %v", err)
		}
		if count != 1 || got.Name != "Second" {
			t.Errorf("rows = %d, Name = %q, want 1 and Second", count, got.Name)
		}
	})

	t.Run("ST-57_Live_ConcurrentUpsertsSameXUID", func(t *testing.T) {
		db := testutil.LivePool(t, "archive")
		s := mcGeyserStore(db)
		const n = 50
		for trial := 0; trial < 3; trial++ {
			xuid := mcUniqueXUID()
			id := xuidToUUID(xuid)
			var wg sync.WaitGroup
			errs := make([]error, n)
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					errs[i] = s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{ID: id, XUID: xuid, Name: fmt.Sprintf("Name%d", i)})
				}(i)
			}
			wg.Wait()
			for i, err := range errs {
				if err != nil {
					t.Errorf("trial %d goroutine %d: UpsertGeyserPlayer() error = %v, want nil", trial, i, err)
				}
			}
			var count int
			if err := db.QueryRow(mcCtx, "SELECT count(*) FROM geyser_players WHERE id = $1", id).Scan(&count); err != nil {
				t.Fatalf("count geyser_players error = %v", err)
			}
			if count != 1 {
				t.Errorf("trial %d: rows = %d, want 1", trial, count)
			}
		}
	})
}

type mcLenReadCloser struct{ *strings.Reader }

func (mcLenReadCloser) Close() error { return nil }

// mcSeekableReadCloser wraps io.ReadSeeker (not the concrete *strings.Reader)
// so Len() isn't promoted, simulating a body with no Content-Length. The S3
// SDK still needs Seek() to hash the payload; io.NopCloser would strip it
// and fail PutObject before it ever reaches the fake server.
type mcSeekableReadCloser struct{ io.ReadSeeker }

func (mcSeekableReadCloser) Close() error { return nil }

func TestST68to70_PutTextureInS3(t *testing.T) {
	t.Run("ST-68_WithLen", func(t *testing.T) {
		var gotLen int64 = -1
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) {
			gotLen = r.ContentLength
			w.WriteHeader(http.StatusOK)
		})
		s := &s3store{s3: s3c}
		body := mcLenReadCloser{strings.NewReader("hello world")}
		if err := s.PutTextureInS3(mcCtx, "hash", body); err != nil {
			t.Fatalf("PutTextureInS3() error = %v", err)
		}
		if gotLen != int64(len("hello world")) {
			t.Errorf("Content-Length = %d, want %d", gotLen, len("hello world"))
		}
	})

	t.Run("ST-69_WithoutLen", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		s := &s3store{s3: s3c}
		if err := s.PutTextureInS3(mcCtx, "hash", mcSeekableReadCloser{strings.NewReader("hello")}); err != nil {
			t.Errorf("PutTextureInS3() error = %v", err)
		}
	})

	t.Run("ST-70_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &s3store{s3: s3c}
		err := s.PutTextureInS3(mcCtx, "hash", mcSeekableReadCloser{strings.NewReader("hello")})
		if !errors.Is(err, ErrUploadS3) {
			t.Errorf("err = %v, want it to wrap %v", err, ErrUploadS3)
		}
	})

}
