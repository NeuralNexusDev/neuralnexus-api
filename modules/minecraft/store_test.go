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

func mcStoreWithUnreachableDB(t *testing.T) *store {
	return &store{db: testutil.UnreachablePool(t)}
}

func mcStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{cache: cache{rdb: testutil.UnreachableRedis(t)}}
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
	pool, err := pgxpool.New(context.Background(), testutil.PostgresURL(t, "archive"))
	if err != nil {
		t.Fatalf("failed to connect to the archive database: %v", err)
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
func mcLiveStoreRedis(t *testing.T) *store { return &store{cache: cache{rdb: mcLiveRedis(t)}} }

var mcXUIDCounter = time.Now().UnixNano()

func mcUniqueXUID() int64 { return atomic.AddInt64(&mcXUIDCounter, 1) }

func mcUniqueHash(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(uuid.New().String(), "-", ""))
}

func TestST02to04_GetPlayerByUUID(t *testing.T) {
	t.Run("ST-02_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetPlayerByUUID(mcCtx, uuid.New().String())
		if err == nil || errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want a raw connection error", err)
		}
	})

	t.Run("ST-03_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
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
		s := mcLiveStoreDB(t)
		_, err := s.GetPlayerByUUID(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST05to07_GetPlayerByName(t *testing.T) {
	t.Run("ST-05_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetPlayerByName(mcCtx, "nobody")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-06_Live_Found", func(t *testing.T) {
		s := mcLiveStoreDB(t)
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
		s := mcLiveStoreDB(t)
		_, err := s.GetPlayerByName(mcCtx, "definitely-not-a-real-name-"+mcUniqueHash("n"))
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST08to11_GetProfileByUUID(t *testing.T) {
	t.Run("ST-08_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetProfileByUUID(mcCtx, uuid.New().String())
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-11_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetProfileByUUID(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST12to14_GetTextures(t *testing.T) {
	t.Run("ST-12_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetTextures(mcCtx, JavaPlayerTexturesTable, uuid.New().String(), "Steve")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})
}

func TestST15to19_UpsertPlayer(t *testing.T) {
	t.Run("ST-15_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		err := s.UpsertPlayer(mcCtx, &MojangPlayer{ID: uuid.New().String(), Name: "Steve"}, false)
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-17_Live_UpdateProfileFalseDefaults", func(t *testing.T) {
		s := mcLiveStoreDB(t)
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
		s := mcLiveStoreDB(t)
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
		s := mcLiveStoreDB(t)
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
		s := mcStoreWithUnreachableDB(t)
		err := s.UpsertTextures(mcCtx, JavaPlayerTexturesTable, &TexturesValue{ProfileID: uuid.New().String(), Textures: Textures{SKIN: &Texture{URL: "http://x/hash"}}})
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})
}

func TestST26to28_UpsertTextureHash(t *testing.T) {
	t.Run("ST-26_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		if err := s.UpsertTextureHash(mcCtx, "abc"); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-27_Live_New", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		if err := s.UpsertTextureHash(mcCtx, mcUniqueHash("hash")); err != nil {
			t.Errorf("UpsertTextureHash() error = %v", err)
		}
	})

	t.Run("ST-28_Live_Duplicate", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		hash := mcUniqueHash("hash")
		if err := s.UpsertTextureHash(mcCtx, hash); err != nil {
			t.Fatalf("first UpsertTextureHash() error = %v", err)
		}
		if err := s.UpsertTextureHash(mcCtx, hash); err != nil {
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
		s := mcStoreWithUnreachableRedis(t)
		_, err := s.GetPlayerFromCache(mcCtx, "key")
		if err == nil || errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want a raw connection error", err)
		}
	})

	t.Run("ST-30_Live_Hit", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
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
		s := mcLiveStoreRedis(t)
		_, err := s.GetPlayerFromCache(mcCtx, "no-such-key-"+mcUniqueHash("k"))
		if !errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want %v", err, ErrCacheMiss)
		}
	})

	t.Run("ST-32_Live_MalformedJSON", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
		key := "malformed-" + mcUniqueHash("k")
		if err := s.cache.rdb.Set(context.Background(), CachePlayer+key, "not json", time.Minute).Err(); err != nil {
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
		s := mcStoreWithUnreachableRedis(t)
		if err := s.SetPlayerInCache(mcCtx, &MojangPlayer{ID: "id", Name: "name"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-34_Live_SetsBothKeys", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
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
		s := mcStoreWithUnreachableRedis(t)
		_, err := s.GetProfileFromCache(mcCtx, "id")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-36_Live_Hit", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
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
		s := mcLiveStoreRedis(t)
		_, err := s.GetProfileFromCache(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want %v", err, ErrCacheMiss)
		}
	})
}

func TestST38to39_SetProfileInCache(t *testing.T) {
	t.Run("ST-38_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableRedis(t)
		if err := s.SetProfileInCache(mcCtx, &MojangProfile{ID: "id"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-39_Live_ReadableBack", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
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
		s := mcStoreWithUnreachableRedis(t)
		_, err := s.GetSignedProfileFromCache(mcCtx, "id")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-41_Live_Hit", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
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
		s := mcLiveStoreRedis(t)
		_, err := s.GetSignedProfileFromCache(mcCtx, uuid.New().String())
		if !errors.Is(err, ErrCacheMiss) {
			t.Errorf("err = %v, want %v", err, ErrCacheMiss)
		}
	})
}

func TestST43to44_SetSignedProfileInCache(t *testing.T) {
	t.Run("ST-43_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableRedis(t)
		if err := s.SetSignedProfileInCache(mcCtx, &MojangPlayer{ID: "id"}); err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-44_Live_ReadableBack", func(t *testing.T) {
		s := mcLiveStoreRedis(t)
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
		s := &store{sThree: sThree{s3: s3c}}
		got, err := s.IsTextureInS3(mcCtx, "hash")
		if err != nil || !got {
			t.Errorf("got (%v, %v), want (true, nil)", got, err)
		}
	})

	t.Run("ST-46_NotFound", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
		s := &store{sThree: sThree{s3: s3c}}
		got, err := s.IsTextureInS3(mcCtx, "hash")
		if err != nil || got {
			t.Errorf("got (%v, %v), want (false, nil)", got, err)
		}
	})

	t.Run("ST-47_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{sThree: sThree{s3: s3c}}
		got, err := s.IsTextureInS3(mcCtx, "hash")
		if err == nil || got {
			t.Errorf("got (%v, %v), want (false, non-nil error)", got, err)
		}
	})
}

func TestST48to50_GetGeyserPlayerByGamertag(t *testing.T) {
	t.Run("ST-48_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-50_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "no-such-gamertag-"+mcUniqueHash("g"))
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST51to53_GetGeyserPlayerByXUID(t *testing.T) {
	t.Run("ST-51_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		_, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-53_Live_NotFound", func(t *testing.T) {
		s := mcLiveStoreDB(t)
		_, err := s.GetGeyserPlayerByXUID(mcCtx, mcUniqueXUID())
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("error = %v, want %v", err, ErrPlayerNotFound)
		}
	})
}

func TestST54to57_UpsertGeyserPlayer(t *testing.T) {
	t.Run("ST-54_Unreachable", func(t *testing.T) {
		s := mcStoreWithUnreachableDB(t)
		if err := s.UpsertGeyserPlayer(mcCtx, &GeyserPlayer{XUID: 1, Gamertag: "Notch"}); err == nil {
			t.Error("expected a raw connection error")
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
		s := &store{sThree: sThree{s3: s3c}}
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
		s := &store{sThree: sThree{s3: s3c}}
		if err := s.PutTextureInS3(mcCtx, "hash", mcSeekableReadCloser{strings.NewReader("hello")}); err != nil {
			t.Errorf("PutTextureInS3() error = %v", err)
		}
	})

	t.Run("ST-70_ServerError", func(t *testing.T) {
		s3c := mcFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		s := &store{sThree: sThree{s3: s3c}}
		err := s.PutTextureInS3(mcCtx, "hash", mcSeekableReadCloser{strings.NewReader("hello")})
		if !errors.Is(err, ErrUploadS3) {
			t.Errorf("err = %v, want it to wrap %v", err, ErrUploadS3)
		}
	})

}
