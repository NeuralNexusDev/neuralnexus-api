package minecraft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func mcUnusedTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find an unused port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close probe listener: %v", err)
	}
	return port
}

func mcUnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	port := mcUnusedTCPPort(t)
	cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://user:pass@127.0.0.1:%d/db?sslmode=disable&connect_timeout=2", port))
	if err != nil {
		t.Fatalf("failed to parse pool config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func mcUnreachableRedis(t *testing.T) *redis.Client {
	t.Helper()
	port := mcUnusedTCPPort(t)
	c := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("127.0.0.1:%d", port),
		DialTimeout: 2 * time.Second,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func mcStoreWithUnreachableDB(t *testing.T) *store {
	return &store{db: mcUnreachablePool(t)}
}

func mcStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{rdb: mcUnreachableRedis(t)}
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

func mcLiveDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set; skipping live-Postgres test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect to TEST_POSTGRES_URL: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
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

func mcLiveStoreDB(t *testing.T) *store    { return &store{db: mcLiveDB(t)} }
func mcLiveStoreRedis(t *testing.T) *store { return &store{rdb: mcLiveRedis(t)} }

var mcXUIDCounter = time.Now().UnixNano()

func mcUniqueXUID() int64 { return atomic.AddInt64(&mcXUIDCounter, 1) }

func mcUniqueHash(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(uuid.New().String(), "-", ""))
}

func TestST01_NewStore(t *testing.T) {
	t.Run("ST-01_WrapsGivenValues", func(t *testing.T) {
		db := mcUnreachablePool(t)
		rdb := mcUnreachableRedis(t)
		s3c := mcFakeS3(t, func(http.ResponseWriter, *http.Request) {})

		got := NewStore(db, rdb, s3c)
		s, ok := got.(*store)
		if !ok {
			t.Fatalf("NewStore() returned %T, want *store", got)
		}
		if s.db != db || s.rdb != rdb || s.s3 != s3c {
			t.Error("NewStore() did not retain the given db/rdb/s3 values")
		}
	})
}

func TestST02to04_GetPlayerByUUID(t *testing.T) {
	t.Run("ST-02_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetPlayerByUUID(uuid.New().String())
		if err == nil || errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want a raw connection error", err)
		}
	})

	t.Run("ST-03_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve"}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByUUID(id)
		if err != nil {
			t.Fatalf("GetPlayerByUUID() error = %v", err)
		}
		if got.ID != id || got.Name != "Steve" {
			t.Errorf("got %+v, want id=%s name=Steve", got, id)
		}
	})

	t.Run("ST-04_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetPlayerByUUID(uuid.New().String())
		if err == nil {
			t.Error("expected a non-nil error for an unknown id")
		}
	})
}

func TestST05to07_GetPlayerByName(t *testing.T) {
	t.Run("ST-05_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetPlayerByName("nobody")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-06_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		name := "Name_" + mcUniqueHash("n")
		if err := s.UpsertPlayer(&Player{ID: id, Name: name}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByName(name)
		if err != nil {
			t.Fatalf("GetPlayerByName() error = %v", err)
		}
		if got.ID != id {
			t.Errorf("got %+v, want id=%s", got, id)
		}
	})

	t.Run("ST-07_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetPlayerByName("definitely-not-a-real-name-" + mcUniqueHash("n"))
		if err == nil {
			t.Error("expected a non-nil error for an unknown name")
		}
	})
}

func TestST08to11_GetProfileByUUID(t *testing.T) {
	t.Run("ST-08_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetProfileByUUID(uuid.New().String())
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-09_Live_WithTextures", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		hash := mcUniqueHash("skin")
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve", ProfileActions: []string{}}, true); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		if err := s.UpsertTextureHash(hash); err != nil {
			t.Fatalf("seed UpsertTextureHash() error = %v", err)
		}
		tv := &TexturesValue{ProfileID: id, Timestamp: mcNow(), Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + hash}}}
		if err := s.UpsertTextures(tv); err != nil {
			t.Fatalf("seed UpsertTextures() error = %v", err)
		}

		got, err := s.GetProfileByUUID(id)
		if err != nil {
			t.Fatalf("GetProfileByUUID() error = %v", err)
		}
		if got.Textures == nil || got.Textures.Textures.SKIN == nil {
			t.Errorf("got %+v, want decoded Textures with a SKIN entry", got)
		}
	})

	t.Run("ST-10_Live_NoTextures", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve", ProfileActions: []string{}}, true); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		got, err := s.GetProfileByUUID(id)
		if err != nil {
			t.Fatalf("GetProfileByUUID() error = %v", err)
		}
		if got.Textures != nil {
			t.Errorf("Textures = %+v, want nil", got.Textures)
		}
	})

	t.Run("ST-11_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetProfileByUUID(uuid.New().String())
		if err == nil {
			t.Error("expected a non-nil error for an unknown id")
		}
	})
}

func TestST12to14_GetTextures(t *testing.T) {
	t.Run("ST-12_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.getTextures(uuid.New().String(), "Steve")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-13_Live_Present", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		hash := mcUniqueHash("skin")
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve"}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		if err := s.UpsertTextureHash(hash); err != nil {
			t.Fatalf("seed UpsertTextureHash() error = %v", err)
		}
		tv := &TexturesValue{ProfileID: id, Timestamp: mcNow(), Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + hash}}}
		if err := s.UpsertTextures(tv); err != nil {
			t.Fatalf("seed UpsertTextures() error = %v", err)
		}
		got, err := s.getTextures(id, "Steve")
		if err != nil || got == nil {
			t.Fatalf("getTextures() = (%+v, %v), want a decoded value", got, err)
		}
	})

	t.Run("ST-14_Live_Absent", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve"}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		got, err := s.getTextures(id, "Steve")
		if err != nil || got != nil {
			t.Errorf("getTextures() = (%+v, %v), want (nil, nil)", got, err)
		}
	})
}

func TestST15to19_UpsertPlayer(t *testing.T) {
	t.Run("ST-15_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		err := s.UpsertPlayer(&Player{ID: uuid.New().String(), Name: "Steve"}, false)
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-16_Live_UpdateProfileTrue", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve", Legacy: true, ProfileActions: []string{"FORCED_NAME_CHANGE"}}, true); err != nil {
			t.Fatalf("UpsertPlayer() error = %v", err)
		}
		got, err := s.GetProfileByUUID(id)
		if err != nil {
			t.Fatalf("GetProfileByUUID() error = %v", err)
		}
		if !got.Legacy || len(got.ProfileActions) != 1 {
			t.Errorf("got %+v, want Legacy=true and one ProfileAction", got)
		}
	})

	t.Run("ST-17_Live_UpdateProfileFalseDefaults", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve", Legacy: true}, false); err != nil {
			t.Fatalf("UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByUUID(id)
		if err != nil {
			t.Fatalf("GetPlayerByUUID() error = %v", err)
		}
		if got.Legacy || got.Demo {
			t.Errorf("got %+v, want Legacy=false Demo=false regardless of input", got)
		}
	})

	t.Run("ST-18_Live_UpsertTwiceUpdatesInPlace", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		if err := s.UpsertPlayer(&Player{ID: id, Name: "First"}, false); err != nil {
			t.Fatalf("first UpsertPlayer() error = %v", err)
		}
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Second"}, false); err != nil {
			t.Fatalf("second UpsertPlayer() error = %v", err)
		}
		got, err := s.GetPlayerByUUID(id)
		if err != nil {
			t.Fatalf("GetPlayerByUUID() error = %v", err)
		}
		if got.Name != "Second" {
			t.Errorf("Name = %q, want Second", got.Name)
		}
	})

	t.Run("ST-19_Live_ConcurrentUpsertsSameID", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		const n = 50
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = s.UpsertPlayer(&Player{ID: id, Name: fmt.Sprintf("Name%d", i)}, false)
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d: UpsertPlayer() error = %v, want nil (ON CONFLICT should absorb the race)", i, err)
			}
		}
		if _, err := s.GetPlayerByUUID(id); err != nil {
			t.Errorf("GetPlayerByUUID() after concurrent upserts error = %v, want exactly one surviving row", err)
		}
	})
}

func TestST20to23_UpsertTextures(t *testing.T) {
	t.Run("ST-20_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		err := s.UpsertTextures(&TexturesValue{ProfileID: uuid.New().String(), Textures: Textures{SKIN: &Texture{URL: "http://x/hash"}}})
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-21_Live_SlimModel", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		hash := mcUniqueHash("skin")
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve"}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		if err := s.UpsertTextureHash(hash); err != nil {
			t.Fatalf("seed UpsertTextureHash() error = %v", err)
		}
		slim := SLIM
		tv := &TexturesValue{ProfileID: id, Timestamp: mcNow(), Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + hash, Metadata: &Metadata{Model: slim}}}}
		if err := s.UpsertTextures(tv); err != nil {
			t.Fatalf("UpsertTextures() error = %v", err)
		}
		got, err := s.getTextures(id, "Steve")
		if err != nil || got == nil || got.Textures.SKIN.Metadata == nil || got.Textures.SKIN.Metadata.Model != SLIM {
			t.Errorf("got (%+v, %v), want SLIM model stored", got, err)
		}
	})

	t.Run("ST-22_Live_UpsertTwiceUpdatesLastSeen", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		hash := mcUniqueHash("skin")
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve"}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		if err := s.UpsertTextureHash(hash); err != nil {
			t.Fatalf("seed UpsertTextureHash() error = %v", err)
		}
		tv := &TexturesValue{ProfileID: id, Timestamp: mcNow(), Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + hash}}}
		if err := s.UpsertTextures(tv); err != nil {
			t.Fatalf("first UpsertTextures() error = %v", err)
		}
		tv.Timestamp = mcNow() + 1000
		if err := s.UpsertTextures(tv); err != nil {
			t.Fatalf("second UpsertTextures() error = %v", err)
		}
		got, err := s.getTextures(id, "Steve")
		if err != nil || got == nil {
			t.Fatalf("getTextures() = (%+v, %v)", got, err)
		}
		if got.Timestamp != tv.Timestamp {
			t.Errorf("Timestamp = %d, want the updated %d (no duplicate row)", got.Timestamp, tv.Timestamp)
		}
	})

	t.Run("ST-23_Live_ConcurrentUpsertsSameConflictKey", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		id := uuid.New().String()
		hash := mcUniqueHash("skin")
		if err := s.UpsertPlayer(&Player{ID: id, Name: "Steve"}, false); err != nil {
			t.Fatalf("seed UpsertPlayer() error = %v", err)
		}
		if err := s.UpsertTextureHash(hash); err != nil {
			t.Fatalf("seed UpsertTextureHash() error = %v", err)
		}
		const n = 50
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				tv := &TexturesValue{ProfileID: id, Timestamp: mcNow(), Textures: Textures{SKIN: &Texture{URL: mojangTextureURL + hash}}}
				errs[i] = s.UpsertTextures(tv)
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d: UpsertTextures() error = %v, want nil", i, err)
			}
		}
	})
}

func TestST24to25_TextureHash(t *testing.T) {
	t.Run("ST-24_Parsable", func(t *testing.T) {
		got := textureHash(&Texture{URL: "http://x/abc"})
		if got == nil || *got != "abc" {
			t.Errorf("textureHash() = %v, want pointer to abc", got)
		}
	})

	t.Run("ST-25_Unparsable", func(t *testing.T) {
		if got := textureHash(nil); got != nil {
			t.Errorf("textureHash(nil) = %v, want nil", got)
		}
	})
}

func TestST26to28_UpsertTextureHash(t *testing.T) {
	t.Run("ST-26_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		if err := s.UpsertTextureHash("abc"); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-27_Live_New", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		if err := s.UpsertTextureHash(mcUniqueHash("hash")); err != nil {
			t.Errorf("UpsertTextureHash() error = %v", err)
		}
	})

	t.Run("ST-28_Live_Duplicate", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		hash := mcUniqueHash("hash")
		if err := s.UpsertTextureHash(hash); err != nil {
			t.Fatalf("first UpsertTextureHash() error = %v", err)
		}
		if err := s.UpsertTextureHash(hash); err != nil {
			t.Errorf("second UpsertTextureHash() error = %v, want nil (ON CONFLICT DO NOTHING)", err)
		}
	})

	t.Run("ST-76_Live_ConcurrentUpsertsSameHash", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		hash := mcUniqueHash("hash")
		const n = 50
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = s.UpsertTextureHash(hash)
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
		s := mcStoreWithUnreachableRedis(t)
		_, err := s.GetPlayerFromCache("key")
		if err == nil || errors.Is(err, redis.Nil) {
			t.Errorf("err = %v, want a raw connection error", err)
		}
	})

	t.Run("ST-30_Live_Hit", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		id := uuid.New().String()
		p := &Player{ID: id, Name: "Steve_" + mcUniqueHash("n")}
		if err := s.SetPlayerInCache(p); err != nil {
			t.Fatalf("seed SetPlayerInCache() error = %v", err)
		}
		got, err := s.GetPlayerFromCache(id)
		if err != nil || got.ID != id {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("ST-31_Live_Miss", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		_, err := s.GetPlayerFromCache("no-such-key-" + mcUniqueHash("k"))
		if !errors.Is(err, redis.Nil) {
			t.Errorf("err = %v, want redis.Nil", err)
		}
	})

	t.Run("ST-32_Live_MalformedJSON", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		key := "malformed-" + mcUniqueHash("k")
		if err := s.rdb.Set(context.Background(), CachePlayer+key, "not json", time.Minute).Err(); err != nil {
			t.Fatalf("seed Set() error = %v", err)
		}
		_, err := s.GetPlayerFromCache(key)
		if err == nil {
			t.Error("expected an unmarshal error")
		}
	})
}

func TestST33to34_SetPlayerInCache(t *testing.T) {
	t.Run("ST-33_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableRedis(t)
		if err := s.SetPlayerInCache(&Player{ID: "id", Name: "name"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-34_Live_SetsBothKeys", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		id := uuid.New().String()
		name := "Steve_" + mcUniqueHash("n")
		p := &Player{ID: id, Name: name}
		if err := s.SetPlayerInCache(p); err != nil {
			t.Fatalf("SetPlayerInCache() error = %v", err)
		}
		byID, err := s.GetPlayerFromCache(id)
		if err != nil || byID.ID != id {
			t.Errorf("GetPlayerFromCache(id) = (%+v, %v)", byID, err)
		}
		byName, err := s.GetPlayerFromCache(name)
		if err != nil || byName.ID != id {
			t.Errorf("GetPlayerFromCache(name) = (%+v, %v)", byName, err)
		}
	})
}

func TestST35to37_GetProfileFromCache(t *testing.T) {
	t.Run("ST-35_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableRedis(t)
		_, err := s.GetProfileFromCache("id")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-36_Live_Hit", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		id := uuid.New().String()
		if err := s.SetProfileInCache(&Profile{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("seed SetProfileInCache() error = %v", err)
		}
		got, err := s.GetProfileFromCache(id)
		if err != nil || got.ID != id {
			t.Errorf("got (%+v, %v), want cached profile", got, err)
		}
	})

	t.Run("ST-37_Live_Miss", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		_, err := s.GetProfileFromCache(uuid.New().String())
		if !errors.Is(err, redis.Nil) {
			t.Errorf("err = %v, want redis.Nil", err)
		}
	})
}

func TestST38to39_SetProfileInCache(t *testing.T) {
	t.Run("ST-38_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableRedis(t)
		if err := s.SetProfileInCache(&Profile{ID: "id"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-39_Live_ReadableBack", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		id := uuid.New().String()
		if err := s.SetProfileInCache(&Profile{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("SetProfileInCache() error = %v", err)
		}
		got, err := s.GetProfileFromCache(id)
		if err != nil || got.Name != "Steve" {
			t.Errorf("got (%+v, %v)", got, err)
		}
	})
}

func TestST40to42_GetSignedProfileFromCache(t *testing.T) {
	t.Run("ST-40_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableRedis(t)
		_, err := s.GetSignedProfileFromCache("id")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-41_Live_Hit", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		id := uuid.New().String()
		if err := s.SetSignedProfileInCache(&Player{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("seed SetSignedProfileInCache() error = %v", err)
		}
		got, err := s.GetSignedProfileFromCache(id)
		if err != nil || got.ID != id {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("ST-42_Live_Miss", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		_, err := s.GetSignedProfileFromCache(uuid.New().String())
		if !errors.Is(err, redis.Nil) {
			t.Errorf("err = %v, want redis.Nil", err)
		}
	})
}

func TestST43to44_SetSignedProfileInCache(t *testing.T) {
	t.Run("ST-43_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableRedis(t)
		if err := s.SetSignedProfileInCache(&Player{ID: "id"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-44_Live_ReadableBack", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		id := uuid.New().String()
		if err := s.SetSignedProfileInCache(&Player{ID: id, Name: "Steve"}); err != nil {
			t.Fatalf("SetSignedProfileInCache() error = %v", err)
		}
		got, err := s.GetSignedProfileFromCache(id)
		if err != nil || got.Name != "Steve" {
			t.Errorf("got (%+v, %v)", got, err)
		}
	})
}

func TestST45to47_IsTextureInS3(t *testing.T) {
	t.Run("ST-45_Exists", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		s := &store{s3: s3c}
		got, err := s.IsTextureInS3("hash")
		if err != nil || !got {
			t.Errorf("got (%v, %v), want (true, nil)", got, err)
		}
	})

	t.Run("ST-46_NotFound", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
		s := &store{s3: s3c}
		got, err := s.IsTextureInS3("hash")
		if err != nil || got {
			t.Errorf("got (%v, %v), want (false, nil)", got, err)
		}
	})

	t.Run("ST-47_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{s3: s3c}
		got, err := s.IsTextureInS3("hash")
		if err == nil || got {
			t.Errorf("got (%v, %v), want (false, non-nil error)", got, err)
		}
	})
}

func TestST48to50_GetGeyserPlayerByGamertag(t *testing.T) {
	t.Run("ST-48_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetGeyserPlayerByGamertag("Notch")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-49_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		gamertag := "Gamertag_" + mcUniqueHash("g")
		if err := s.UpsertGeyserPlayer(&GeyserPlayer{XUID: xuid, Gamertag: gamertag}); err != nil {
			t.Fatalf("seed UpsertGeyserPlayer() error = %v", err)
		}
		got, err := s.GetGeyserPlayerByGamertag(gamertag)
		if err != nil {
			t.Fatalf("GetGeyserPlayerByGamertag() error = %v", err)
		}
		if got.XUID != xuid || got.UUID != xuidToUUID(xuid) {
			t.Errorf("got %+v, want xuid=%d with derived UUID", got, xuid)
		}
	})

	t.Run("ST-50_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetGeyserPlayerByGamertag("no-such-gamertag-" + mcUniqueHash("g"))
		if err == nil {
			t.Error("expected a non-nil error")
		}
	})
}

func TestST51to53_GetGeyserPlayerByXUID(t *testing.T) {
	t.Run("ST-51_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetGeyserPlayerByXUID(1)
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-52_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		if err := s.UpsertGeyserPlayer(&GeyserPlayer{XUID: xuid, Gamertag: "Notch"}); err != nil {
			t.Fatalf("seed UpsertGeyserPlayer() error = %v", err)
		}
		got, err := s.GetGeyserPlayerByXUID(xuid)
		if err != nil || got.UUID != xuidToUUID(xuid) {
			t.Errorf("got (%+v, %v), want derived UUID", got, err)
		}
	})

	t.Run("ST-53_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetGeyserPlayerByXUID(mcUniqueXUID())
		if err == nil {
			t.Error("expected a non-nil error")
		}
	})
}

func TestST54to57_UpsertGeyserPlayer(t *testing.T) {
	t.Run("ST-54_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		if err := s.UpsertGeyserPlayer(&GeyserPlayer{XUID: 1, Gamertag: "Notch"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-55_Live_New", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		if err := s.UpsertGeyserPlayer(&GeyserPlayer{XUID: mcUniqueXUID(), Gamertag: "Notch"}); err != nil {
			t.Errorf("UpsertGeyserPlayer() error = %v", err)
		}
	})

	t.Run("ST-56_Live_UpsertTwiceUpdatesInPlace", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		if err := s.UpsertGeyserPlayer(&GeyserPlayer{XUID: xuid, Gamertag: "First"}); err != nil {
			t.Fatalf("first UpsertGeyserPlayer() error = %v", err)
		}
		if err := s.UpsertGeyserPlayer(&GeyserPlayer{XUID: xuid, Gamertag: "Second"}); err != nil {
			t.Fatalf("second UpsertGeyserPlayer() error = %v", err)
		}
		got, err := s.GetGeyserPlayerByXUID(xuid)
		if err != nil || got.Gamertag != "Second" {
			t.Errorf("got (%+v, %v), want Gamertag=Second", got, err)
		}
	})

	t.Run("ST-57_Live_ConcurrentUpsertsSameXUID", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		const n = 50
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = s.UpsertGeyserPlayer(&GeyserPlayer{XUID: xuid, Gamertag: fmt.Sprintf("G%d", i)})
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d: UpsertGeyserPlayer() error = %v, want nil", i, err)
			}
		}
	})
}

func TestST58to60_GetGeyserSkin(t *testing.T) {
	t.Run("ST-58_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetGeyserSkin(1)
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-59_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		hash := mcUniqueHash("skin")
		skin := &GeyserSkin{Hash: hash, IsSteve: true, TextureID: "tex", Value: "encoded-value"}
		if err := s.UpsertGeyserSkin(xuid, skin); err != nil {
			t.Fatalf("seed UpsertGeyserSkin() error = %v", err)
		}
		got, err := s.GetGeyserSkin(xuid)
		if err != nil || got.Hash != hash {
			t.Errorf("got (%+v, %v), want hash=%s", got, err, hash)
		}
	})

	t.Run("ST-60_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetGeyserSkin(mcUniqueXUID())
		if err == nil {
			t.Error("expected a non-nil error")
		}
	})
}

func TestST61to63_GetGeyserSkinByHash(t *testing.T) {
	t.Run("ST-61_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetGeyserSkinByHash("hash")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-62_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		hash := mcUniqueHash("skin")
		skin := &GeyserSkin{Hash: hash, IsSteve: true, TextureID: "tex", Value: "encoded-value"}
		if err := s.UpsertGeyserSkin(xuid, skin); err != nil {
			t.Fatalf("seed UpsertGeyserSkin() error = %v", err)
		}
		got, err := s.GetGeyserSkinByHash(hash)
		if err != nil || got == nil || got.Hash != hash {
			t.Errorf("got (%+v, %v), want hash=%s", got, err, hash)
		}
	})

	t.Run("ST-63_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		got, err := s.GetGeyserSkinByHash(mcUniqueHash("nope"))
		if err != nil || got != nil {
			t.Errorf("got (%+v, %v), want (nil, nil)", got, err)
		}
	})
}

func TestST64to67_UpsertGeyserSkin(t *testing.T) {
	t.Run("ST-64_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		err := s.UpsertGeyserSkin(1, &GeyserSkin{Hash: "hash", TextureID: "tex", Value: "v"})
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-65_Live_EmptySignatureStoredAsNull", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		hash := mcUniqueHash("skin")
		if err := s.UpsertGeyserSkin(xuid, &GeyserSkin{Hash: hash, TextureID: "tex", Value: "v", Signature: ""}); err != nil {
			t.Fatalf("UpsertGeyserSkin() error = %v", err)
		}
		got, err := s.GetGeyserSkin(xuid)
		if err != nil || got.Signature != "" {
			t.Errorf("got (%+v, %v), want empty Signature read back via COALESCE", got, err)
		}
	})

	t.Run("ST-66_Live_UpsertTwiceUpdatesInPlace", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		hash := mcUniqueHash("skin")
		if err := s.UpsertGeyserSkin(xuid, &GeyserSkin{Hash: hash, TextureID: "tex1", Value: "v1"}); err != nil {
			t.Fatalf("first UpsertGeyserSkin() error = %v", err)
		}
		if err := s.UpsertGeyserSkin(xuid, &GeyserSkin{Hash: hash, TextureID: "tex2", Value: "v2"}); err != nil {
			t.Fatalf("second UpsertGeyserSkin() error = %v", err)
		}
		got, err := s.GetGeyserSkin(xuid)
		if err != nil || got.TextureID != "tex2" {
			t.Errorf("got (%+v, %v), want TextureID=tex2", got, err)
		}
	})

	t.Run("ST-67_Live_ConcurrentUpsertsSameKey", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		xuid := mcUniqueXUID()
		hash := mcUniqueHash("skin")
		const n = 50
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = s.UpsertGeyserSkin(xuid, &GeyserSkin{Hash: hash, TextureID: fmt.Sprintf("tex%d", i), Value: "v"})
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d: UpsertGeyserSkin() error = %v, want nil", i, err)
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

func mcRequireUploadS3Text(t *testing.T, err error) {
	t.Helper()
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 2 {
		t.Fatalf("error = %v, want an error wrapping ErrUploadS3 and its cause", err)
	}
	want := ErrUploadS3.Error() + ": " + joined.Unwrap()[1].Error()
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestST68to70and77_PutTextureInS3(t *testing.T) {
	t.Run("ST-68_WithLen", func(t *testing.T) {
		var gotLen int64 = -1
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) {
			gotLen = r.ContentLength
			w.WriteHeader(http.StatusOK)
		})
		s := &store{s3: s3c}
		body := mcLenReadCloser{strings.NewReader("hello world")}
		if err := s.PutTextureInS3("hash", body); err != nil {
			t.Fatalf("PutTextureInS3() error = %v", err)
		}
		if gotLen != int64(len("hello world")) {
			t.Errorf("Content-Length = %d, want %d", gotLen, len("hello world"))
		}
	})

	t.Run("ST-69_WithoutLen", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		s := &store{s3: s3c}
		if err := s.PutTextureInS3("hash", mcSeekableReadCloser{strings.NewReader("hello")}); err != nil {
			t.Errorf("PutTextureInS3() error = %v", err)
		}
	})

	t.Run("ST-70_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{s3: s3c}
		err := s.PutTextureInS3("hash", mcSeekableReadCloser{strings.NewReader("hello")})
		if !errors.Is(err, ErrUploadS3) {
			t.Errorf("err = %v, want it to wrap %v", err, ErrUploadS3)
		}
	})

	t.Run("ST-77_ServerErrorMessage", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{s3: s3c}
		err := s.PutTextureInS3("hash", mcSeekableReadCloser{strings.NewReader("hello")})
		mcRequireUploadS3Text(t, err)
	})
}

func TestST71to73_IsGeyserTextureInS3(t *testing.T) {
	t.Run("ST-71_Exists", func(t *testing.T) {
		var gotPath string
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		})
		s := &store{s3: s3c}
		got, err := s.IsGeyserTextureInS3("hash")
		if err != nil || !got {
			t.Errorf("got (%v, %v), want (true, nil)", got, err)
		}
		if !strings.Contains(gotPath, GeyserS3KeyPrefix) {
			t.Errorf("request path = %q, want it to use GeyserS3KeyPrefix", gotPath)
		}
	})

	t.Run("ST-72_NotFound", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
		s := &store{s3: s3c}
		got, err := s.IsGeyserTextureInS3("hash")
		if err != nil || got {
			t.Errorf("got (%v, %v), want (false, nil)", got, err)
		}
	})

	t.Run("ST-73_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{s3: s3c}
		got, err := s.IsGeyserTextureInS3("hash")
		if err == nil || got {
			t.Errorf("got (%v, %v), want (false, non-nil error)", got, err)
		}
	})
}

func TestST74to75and78_PutGeyserTextureInS3(t *testing.T) {
	t.Run("ST-74_OK", func(t *testing.T) {
		var gotPath string
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		})
		s := &store{s3: s3c}
		if err := s.PutGeyserTextureInS3("hash", mcSeekableReadCloser{strings.NewReader("hello")}); err != nil {
			t.Fatalf("PutGeyserTextureInS3() error = %v", err)
		}
		if !strings.Contains(gotPath, GeyserS3KeyPrefix) {
			t.Errorf("request path = %q, want it to use GeyserS3KeyPrefix", gotPath)
		}
	})

	t.Run("ST-75_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{s3: s3c}
		err := s.PutGeyserTextureInS3("hash", mcSeekableReadCloser{strings.NewReader("hello")})
		if !errors.Is(err, ErrUploadS3) {
			t.Errorf("err = %v, want it to wrap %v", err, ErrUploadS3)
		}
	})

	t.Run("ST-78_ServerErrorMessage", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{s3: s3c}
		err := s.PutGeyserTextureInS3("hash", mcSeekableReadCloser{strings.NewReader("hello")})
		mcRequireUploadS3Text(t, err)
	})
}
