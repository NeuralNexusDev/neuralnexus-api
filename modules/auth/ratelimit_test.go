package auth

import (
	"errors"
	"testing"
)

type rlFakeRateLimitStore struct {
	getVal    int
	getErr    error
	getCalls  int
	setErr    error
	setCalls  int
	incrErr   error
	incrCalls int
}

func (f *rlFakeRateLimitStore) GetRateLimit(_ string) (int, error) {
	f.getCalls++
	return f.getVal, f.getErr
}
func (f *rlFakeRateLimitStore) SetRateLimit(_ string, _ int) error {
	f.setCalls++
	return f.setErr
}
func (f *rlFakeRateLimitStore) IncrementRateLimit(_ string) error {
	f.incrCalls++
	return f.incrErr
}

type rlFakeStore struct {
	rl RateLimitStore
}

func (f *rlFakeStore) Account() AccountStore { panic("rlFakeStore: Account not implemented") }
func (f *rlFakeStore) AccountSettings() AccountSettingsStore {
	panic("rlFakeStore: AccountSettings not implemented")
}
func (f *rlFakeStore) Session() SessionStore { panic("rlFakeStore: Session not implemented") }
func (f *rlFakeStore) LinkAccount() LinkAccountStore {
	panic("rlFakeStore: LinkAccount not implemented")
}
func (f *rlFakeStore) RateLimit() RateLimitStore   { return f.rl }
func (f *rlFakeStore) OAuthToken() OAuthTokenStore { panic("rlFakeStore: OAuthToken not implemented") }

func TestRL01NewRateLimitService(t *testing.T) {
	t.Run("RL-01_WiresGivenSubStore", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{getVal: 7}
		svc := NewRateLimitService(&rlFakeStore{rl: rl})

		got, err := svc.GetRateLimit("k")
		if err != nil || got != 7 {
			t.Errorf("GetRateLimit() = (%d, %v), want (7, nil)", got, err)
		}
		if rl.getCalls != 1 {
			t.Errorf("expected the fake RateLimitStore to be called once, got %d", rl.getCalls)
		}
	})
}

func TestRL02to03GetRateLimit(t *testing.T) {
	t.Run("RL-02_Success", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{getVal: 5}
		svc := NewRateLimitService(&rlFakeStore{rl: rl})

		got, err := svc.GetRateLimit("k")
		if err != nil || got != 5 {
			t.Errorf("GetRateLimit() = (%d, %v), want (5, nil)", got, err)
		}
	})

	t.Run("RL-03_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		rl := &rlFakeRateLimitStore{getErr: wantErr}
		svc := NewRateLimitService(&rlFakeStore{rl: rl})

		_, err := svc.GetRateLimit("k")
		if !errors.Is(err, wantErr) {
			t.Errorf("GetRateLimit() err = %v, want %v", err, wantErr)
		}
	})
}

func TestRL04to05SetRateLimit(t *testing.T) {
	t.Run("RL-04_Success", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{}
		svc := NewRateLimitService(&rlFakeStore{rl: rl})

		if err := svc.SetRateLimit("k", 3); err != nil {
			t.Errorf("SetRateLimit() = %v, want nil", err)
		}
		if rl.setCalls != 1 {
			t.Errorf("expected SetRateLimit to be called once, got %d", rl.setCalls)
		}
	})

	t.Run("RL-05_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		rl := &rlFakeRateLimitStore{setErr: wantErr}
		svc := NewRateLimitService(&rlFakeStore{rl: rl})

		err := svc.SetRateLimit("k", 3)
		if !errors.Is(err, wantErr) {
			t.Errorf("SetRateLimit() err = %v, want %v", err, wantErr)
		}
	})
}

func TestRL06to07IncrRateLimit(t *testing.T) {
	t.Run("RL-06_Success", func(t *testing.T) {
		rl := &rlFakeRateLimitStore{}
		svc := NewRateLimitService(&rlFakeStore{rl: rl})

		if err := svc.IncrRateLimit("k"); err != nil {
			t.Errorf("IncrRateLimit() = %v, want nil", err)
		}
		if rl.incrCalls != 1 {
			t.Errorf("expected IncrementRateLimit to be called once, got %d", rl.incrCalls)
		}
	})

	t.Run("RL-07_StoreError", func(t *testing.T) {
		wantErr := errors.New("boom")
		rl := &rlFakeRateLimitStore{incrErr: wantErr}
		svc := NewRateLimitService(&rlFakeStore{rl: rl})

		err := svc.IncrRateLimit("k")
		if !errors.Is(err, wantErr) {
			t.Errorf("IncrRateLimit() err = %v, want %v", err, wantErr)
		}
	})
}
