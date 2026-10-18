package ratelimit

import (
	"context"
	"errors"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

type rlFakeRateLimitStore struct {
	getCtx    context.Context
	getVal    int
	getErr    error
	getCalls  int
	setErr    error
	setCalls  int
	incrErr   error
	incrCalls int
}

func (f *rlFakeRateLimitStore) GetRateLimit(ctx context.Context, _ string) (int, error) {
	f.getCtx = ctx
	f.getCalls++
	return f.getVal, f.getErr
}
func (f *rlFakeRateLimitStore) SetRateLimit(_ context.Context, _ string, _ int) error {
	f.setCalls++
	return f.setErr
}
func (f *rlFakeRateLimitStore) IncrementRateLimit(_ context.Context, _ string) error {
	f.incrCalls++
	return f.incrErr
}

func TestRL01NewService(t *testing.T) {
	t.Run("RL-01_WiresGivenSubStore", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{getVal: 7}
		svc := NewService(rl)

		got, err := svc.GetRateLimit(context.Background(), "k")
		if err != nil || got != 7 {
			t.Errorf("GetRateLimit() = (%d, %v), want (7, nil)", got, err)
		}
		if rl.getCalls != 1 {
			t.Errorf("expected the fake Store to be called once, got %d", rl.getCalls)
		}
	})
}

func TestRL02to03GetRateLimit(t *testing.T) {
	t.Run("RL-02_Success", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{getVal: 5}
		svc := NewService(rl)

		got, err := svc.GetRateLimit(context.Background(), "k")
		if err != nil || got != 5 {
			t.Errorf("GetRateLimit() = (%d, %v), want (5, nil)", got, err)
		}
	})

	t.Run("RL-03_StoreError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		rl := &rlFakeRateLimitStore{getErr: wantErr}
		svc := NewService(rl)

		_, err := svc.GetRateLimit(context.Background(), "k")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetRateLimit() err = %v, want %v", err, wantErr)
		}
	})
}

func TestRL04to05SetRateLimit(t *testing.T) {
	t.Run("RL-04_Success", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{}
		svc := NewService(rl)

		if err := svc.SetRateLimit(context.Background(), "k", 3); err != nil {
			t.Errorf("SetRateLimit() = %v, want nil", err)
		}
		if rl.setCalls != 1 {
			t.Errorf("expected SetRateLimit to be called once, got %d", rl.setCalls)
		}
	})

	t.Run("RL-05_StoreError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		rl := &rlFakeRateLimitStore{setErr: wantErr}
		svc := NewService(rl)

		err := svc.SetRateLimit(context.Background(), "k", 3)
		if !errors.Is(err, wantErr) {
			t.Errorf("SetRateLimit() err = %v, want %v", err, wantErr)
		}
	})
}

func TestRL06to07IncrRateLimit(t *testing.T) {
	t.Run("RL-06_Success", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{}
		svc := NewService(rl)

		if err := svc.IncrRateLimit(context.Background(), "k"); err != nil {
			t.Errorf("IncrRateLimit() = %v, want nil", err)
		}
		if rl.incrCalls != 1 {
			t.Errorf("expected IncrementRateLimit to be called once, got %d", rl.incrCalls)
		}
	})

	t.Run("RL-07_StoreError", func(t *testing.T) {
		wantErr := testutil.ErrBoom
		rl := &rlFakeRateLimitStore{incrErr: wantErr}
		svc := NewService(rl)

		err := svc.IncrRateLimit(context.Background(), "k")
		if !errors.Is(err, wantErr) {
			t.Errorf("IncrRateLimit() err = %v, want %v", err, wantErr)
		}
	})
}

func TestRL08RequestContextReachesStore(t *testing.T) {
	t.Run("RL-08_RequestContextReachesStore", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{}
		ctx := context.WithValue(context.Background(), reqctx.UserIDKey, "u1")

		if _, err := NewService(rl).GetRateLimit(ctx, "k"); err != nil {
			t.Fatalf("GetRateLimit() error = %v", err)
		}
		if got := rl.getCtx.Value(reqctx.UserIDKey); got != "u1" {
			t.Errorf("store context user ID = %v, want %q", got, "u1")
		}
	})
}
