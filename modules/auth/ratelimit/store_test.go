package ratelimit

import (
	"context"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func stStoreWithUnreachableRedis(t *testing.T) *store {
	return &store{rdb: testutil.UnreachableRedis(t)}
}

func TestST01NewStore(t *testing.T) {
	t.Run("RLS-01_WrapsGivenClient", func(t *testing.T) {
		rdb := testutil.UnreachableRedis(t)

		got := NewStore(rdb)
		s, ok := got.(*store)
		if !ok {
			t.Fatalf("NewStore() returned type %T, want *store", got)
		}
		if s.rdb != rdb {
			t.Error("NewStore() did not retain the given *redis.Client")
		}
	})
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

func TestST41SetRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("RLS-41_ConnectionError", func(t *testing.T) {
		testutil.AssertRawConnectionError(t, s.SetRateLimit(context.Background(), "k", 1))
	})
}

func TestST42IncrementRateLimit(t *testing.T) {
	s := stStoreWithUnreachableRedis(t)
	t.Run("RLS-42_ConnectionError", func(t *testing.T) {
		testutil.AssertRawConnectionError(t, s.IncrementRateLimit(context.Background(), "k"))
	})
}
