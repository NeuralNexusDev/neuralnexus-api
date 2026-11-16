package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func stStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{rdb: testutil.UnreachableRedis(t)}
}

func TestST40GetRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("RLS-40_ConnectionErrorNotTreatedAsMiss", func(t *testing.T) {
		got, err := s.GetRateLimit(context.Background(), "k")
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
		testutil.AssertRawConnectionError(t, err)
	})
}

const rlWindow = time.Minute

func rlLiveStore(t *testing.T) (Store, *redis.Client, string) {
	t.Helper()
	rdb := testutil.LiveRedis(t)
	key := "rls-" + uuid.NewString()
	t.Cleanup(func() { rdb.Del(context.Background(), rateLimitKeyPrefix+key) })
	return NewStore(config.RateLimit{Window: rlWindow}, rdb), rdb, key
}

func rlRequireStored(t *testing.T, rdb *redis.Client, key string, want int) {
	t.Helper()
	got, err := rdb.Get(context.Background(), rateLimitKeyPrefix+key).Int()
	if err != nil {
		t.Fatalf("failed to read the stored count: %v", err)
	}
	if got != want {
		t.Errorf("stored count = %d, want %d", got, want)
	}
}

func rlRequireWindowTTL(t *testing.T, rdb *redis.Client, key string, max time.Duration) {
	t.Helper()
	ttl, err := rdb.TTL(context.Background(), rateLimitKeyPrefix+key).Result()
	if err != nil {
		t.Fatalf("failed to read the expiry: %v", err)
	}
	if ttl <= 0 || ttl > max {
		t.Errorf("expiry = %v, want more than 0 and at most %v", ttl, max)
	}
}

func TestRLS43to44GetRateLimit(t *testing.T) {
	s, rdb, key := rlLiveStore(t)
	ctx := context.Background()

	t.Run("RLS-43_MissProvisionsKeyWithWindow", func(t *testing.T) {
		got, err := s.GetRateLimit(ctx, key)
		if got != 1 || err != nil {
			t.Fatalf("GetRateLimit() = (%d, %v), want (1, nil)", got, err)
		}
		rlRequireStored(t, rdb, key, 1)
		rlRequireWindowTTL(t, rdb, key, rlWindow)
	})

	t.Run("RLS-44_HitReturnsStoredCount", func(t *testing.T) {
		if err := s.SetRateLimit(ctx, key, 7); err != nil {
			t.Fatalf("failed to seed the count: %v", err)
		}
		got, err := s.GetRateLimit(ctx, key)
		if got != 7 || err != nil {
			t.Errorf("GetRateLimit() = (%d, %v), want (7, nil)", got, err)
		}
	})
}

func TestRLS45SetRateLimit(t *testing.T) {
	s, rdb, key := rlLiveStore(t)

	t.Run("RLS-45_StoresCountWithWindow", func(t *testing.T) {
		if err := s.SetRateLimit(context.Background(), key, 5); err != nil {
			t.Fatalf("SetRateLimit() = %v, want nil", err)
		}
		rlRequireStored(t, rdb, key, 5)
		rlRequireWindowTTL(t, rdb, key, rlWindow)
	})
}

func TestRLS46to47IncrementRateLimit(t *testing.T) {
	ctx := context.Background()

	t.Run("RLS-46_NewKeyStartsAtOneWithWindow", func(t *testing.T) {
		s, rdb, key := rlLiveStore(t)
		if err := s.IncrementRateLimit(ctx, key); err != nil {
			t.Fatalf("IncrementRateLimit() = %v, want nil", err)
		}
		rlRequireStored(t, rdb, key, 1)
		rlRequireWindowTTL(t, rdb, key, rlWindow)
	})

	t.Run("RLS-47_ExistingKeyKeepsItsWindow", func(t *testing.T) {
		s, rdb, key := rlLiveStore(t)
		if err := s.SetRateLimit(ctx, key, 3); err != nil {
			t.Fatalf("failed to seed the count: %v", err)
		}
		if err := rdb.Expire(ctx, rateLimitKeyPrefix+key, 30*time.Second).Err(); err != nil {
			t.Fatalf("failed to shorten the expiry: %v", err)
		}
		if err := s.IncrementRateLimit(ctx, key); err != nil {
			t.Fatalf("IncrementRateLimit() = %v, want nil", err)
		}
		rlRequireStored(t, rdb, key, 4)
		rlRequireWindowTTL(t, rdb, key, 30*time.Second)
	})
}
