package beenamegenerator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// -------------- Local-only fixtures (no live service) --------------

// bngUnusedTCPPort returns a TCP port on 127.0.0.1 that is very likely free
// at the moment it's returned, so a subsequent connection attempt to it
// fails fast with "connection refused" instead of hanging.
func bngUnusedTCPPort(t *testing.T) int {
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

// bngUnreachablePool builds a real *pgxpool.Pool pointed at a closed local
// port, so any Exec/Query against it fails fast with a genuine connection
// error - without requiring a live Postgres instance.
func bngUnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	port := bngUnusedTCPPort(t)
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

func bngStoreWithUnreachableDB(t *testing.T) *store {
	return &store{db: bngUnreachablePool(t)}
}

// -------------- Live-dependency fixtures --------------

// bngLiveDB connects to TEST_POSTGRES_URL, skipping the test when it is
// unset so this suite still runs green without the docker-compose test-env
// (see `make test-env-up`/`make test-env-down`, backed by
// docker-compose.test.yml).
func bngLiveDB(t *testing.T) *pgxpool.Pool {
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

func bngLiveStore(t *testing.T) (*store, *pgxpool.Pool) {
	db := bngLiveDB(t)
	return &store{db: db}, db
}

// bngUniqueName hands out bee-name strings unique enough not to collide
// with fixtures from other tests/runs against a shared live database.
func bngUniqueName(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, strings.ReplaceAll(uuid.New().String(), "-", ""))
}

// bngClearTable deletes every row from the given table. bee_name and
// bee_name_suggestion are owned exclusively by this module (no other
// package touches them), so clearing them before an "empty table" assertion
// is a safe, targeted reset rather than a destructive action against
// unrelated data - it just removes the ordering-fragility of relying on
// this being the very first test to touch the table.
func bngClearTable(t *testing.T, db *pgxpool.Pool, table string) {
	t.Helper()
	if _, err := db.Exec(context.Background(), "DELETE FROM "+table); err != nil {
		t.Fatalf("failed to clear %s for test setup: %v", table, err)
	}
}

func bngCountByName(t *testing.T, db *pgxpool.Pool, table, name string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), "SELECT COUNT(*) FROM "+table+" WHERE name = $1", name).Scan(&n); err != nil {
		t.Fatalf("failed to count rows in %s for %q: %v", table, name, err)
	}
	return n
}

// -------------- NewStore --------------

func TestST01_NewStore(t *testing.T) {
	t.Run("ST-01_WrapsGivenPool", func(t *testing.T) {
		db := bngUnreachablePool(t)
		got := NewStore(db)
		s, ok := got.(*store)
		if !ok {
			t.Fatalf("NewStore() returned %T, want *store", got)
		}
		if s.db != db {
			t.Error("NewStore() did not retain the given db pool")
		}
	})
}

// -------------- GetBeeName --------------

func TestST02to04_GetBeeName(t *testing.T) {
	t.Run("ST-02_Unreachable", func(t *testing.T) {
		s := bngStoreWithUnreachableDB(t)
		_, err := s.GetBeeName()
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-04_Live_NoRows", func(t *testing.T) {
		s, db := bngLiveStore(t)
		bngClearTable(t, db, "bee_name")
		_, err := s.GetBeeName()
		if err == nil {
			t.Fatal("expected a non-nil error when bee_name has no rows")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("err = %v, want pgx.ErrNoRows", err)
		}
	})

	t.Run("ST-03_Live_Found", func(t *testing.T) {
		s, _ := bngLiveStore(t)
		name := bngUniqueName("get")
		if _, err := s.UploadBeeName(name); err != nil {
			t.Fatalf("seed UploadBeeName() error = %v", err)
		}
		t.Cleanup(func() { _, _ = s.DeleteBeeName(name) })

		got, err := s.GetBeeName()
		if err != nil {
			t.Fatalf("GetBeeName() error = %v", err)
		}
		if got == "" {
			t.Error("GetBeeName() returned an empty name, want a non-empty seeded name")
		}
	})
}

// -------------- UploadBeeName --------------

func TestST05to06_UploadBeeName(t *testing.T) {
	t.Run("ST-05_Unreachable", func(t *testing.T) {
		s := bngStoreWithUnreachableDB(t)
		_, err := s.UploadBeeName(bngUniqueName("up"))
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-06_Live_OK", func(t *testing.T) {
		s, db := bngLiveStore(t)
		name := bngUniqueName("up")
		got, err := s.UploadBeeName(name)
		if err != nil {
			t.Fatalf("UploadBeeName() error = %v", err)
		}
		if got != name {
			t.Errorf("UploadBeeName() = %q, want %q", got, name)
		}
		t.Cleanup(func() { _, _ = s.DeleteBeeName(name) })

		if n := bngCountByName(t, db, "bee_name", name); n != 1 {
			t.Errorf("bee_name has %d rows for %q, want 1", n, name)
		}
	})
}

// -------------- DeleteBeeName --------------

func TestST07to09_DeleteBeeName(t *testing.T) {
	t.Run("ST-07_Unreachable", func(t *testing.T) {
		s := bngStoreWithUnreachableDB(t)
		_, err := s.DeleteBeeName(bngUniqueName("del"))
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-08_Live_OK", func(t *testing.T) {
		s, db := bngLiveStore(t)
		name := bngUniqueName("del")
		if _, err := s.UploadBeeName(name); err != nil {
			t.Fatalf("seed UploadBeeName() error = %v", err)
		}

		got, err := s.DeleteBeeName(name)
		if err != nil {
			t.Fatalf("DeleteBeeName() error = %v", err)
		}
		if got != name {
			t.Errorf("DeleteBeeName() = %q, want %q", got, name)
		}
		if n := bngCountByName(t, db, "bee_name", name); n != 0 {
			t.Errorf("bee_name has %d rows for %q after delete, want 0", n, name)
		}
	})

	t.Run("ST-09_Live_NonExistent", func(t *testing.T) {
		s, _ := bngLiveStore(t)
		name := bngUniqueName("del-missing")
		got, err := s.DeleteBeeName(name)
		if err != nil {
			t.Fatalf("DeleteBeeName() of a non-existent name error = %v, want nil", err)
		}
		if got != name {
			t.Errorf("DeleteBeeName() = %q, want %q", got, name)
		}
	})
}

// -------------- SubmitBeeName --------------

func TestST10to11_SubmitBeeName(t *testing.T) {
	t.Run("ST-10_Unreachable", func(t *testing.T) {
		s := bngStoreWithUnreachableDB(t)
		_, err := s.SubmitBeeName(bngUniqueName("sub"))
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-11_Live_OK", func(t *testing.T) {
		s, db := bngLiveStore(t)
		name := bngUniqueName("sub")
		got, err := s.SubmitBeeName(name)
		if err != nil {
			t.Fatalf("SubmitBeeName() error = %v", err)
		}
		if got != name {
			t.Errorf("SubmitBeeName() = %q, want %q", got, name)
		}
		t.Cleanup(func() { _, _ = s.RejectBeeNameSuggestion(name) })

		if n := bngCountByName(t, db, "bee_name_suggestion", name); n != 1 {
			t.Errorf("bee_name_suggestion has %d rows for %q, want 1", n, name)
		}
	})
}

// -------------- GetBeeNameSuggestions --------------

func TestST12to14_GetBeeNameSuggestions(t *testing.T) {
	t.Run("ST-12_Unreachable", func(t *testing.T) {
		s := bngStoreWithUnreachableDB(t)
		got, err := s.GetBeeNameSuggestions(5)
		if err == nil {
			t.Error("expected a raw connection error")
		}
		if got == nil || len(got) != 0 {
			t.Errorf("got %v, want an empty (non-nil) slice on error", got)
		}
	})

	t.Run("ST-14_Live_NoRows", func(t *testing.T) {
		s, db := bngLiveStore(t)
		bngClearTable(t, db, "bee_name_suggestion")

		got, err := s.GetBeeNameSuggestions(5)
		if err != nil {
			t.Fatalf("GetBeeNameSuggestions() error = %v, want nil", err)
		}
		if len(got) != 0 {
			t.Errorf("got %v, want an empty slice when no suggestions exist", got)
		}
	})

	t.Run("ST-13_Live_OK", func(t *testing.T) {
		s, db := bngLiveStore(t)
		bngClearTable(t, db, "bee_name_suggestion")

		const n = 3
		names := make([]string, n)
		for i := range names {
			names[i] = bngUniqueName("sugg")
			if _, err := s.SubmitBeeName(names[i]); err != nil {
				t.Fatalf("seed SubmitBeeName() error = %v", err)
			}
		}
		t.Cleanup(func() {
			for _, name := range names {
				_, _ = s.RejectBeeNameSuggestion(name)
			}
		})

		got, err := s.GetBeeNameSuggestions(int64(n))
		if err != nil {
			t.Fatalf("GetBeeNameSuggestions() error = %v", err)
		}
		if len(got) != n {
			t.Errorf("got %d suggestions, want %d", len(got), n)
		}
	})
}

// -------------- AcceptBeeNameSuggestion --------------

func TestST15to17_AcceptBeeNameSuggestion(t *testing.T) {
	t.Run("ST-15_Unreachable", func(t *testing.T) {
		s := bngStoreWithUnreachableDB(t)
		_, err := s.AcceptBeeNameSuggestion(bngUniqueName("acc"))
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-16_Live_OK", func(t *testing.T) {
		s, db := bngLiveStore(t)
		name := bngUniqueName("acc")
		if _, err := s.SubmitBeeName(name); err != nil {
			t.Fatalf("seed SubmitBeeName() error = %v", err)
		}

		got, err := s.AcceptBeeNameSuggestion(name)
		if err != nil {
			t.Fatalf("AcceptBeeNameSuggestion() error = %v", err)
		}
		if got != name {
			t.Errorf("AcceptBeeNameSuggestion() = %q, want %q", got, name)
		}
		t.Cleanup(func() { _, _ = s.DeleteBeeName(name) })

		if n := bngCountByName(t, db, "bee_name", name); n < 1 {
			t.Errorf("bee_name has %d rows for %q after accept, want >= 1", n, name)
		}
		if n := bngCountByName(t, db, "bee_name_suggestion", name); n != 0 {
			t.Errorf("bee_name_suggestion has %d rows for %q after accept, want 0", n, name)
		}
	})

	// ST-17: AcceptBeeNameSuggestion does its insert-into-bee_name and
	// delete-from-bee_name_suggestion as two separate, non-transactional
	// statements. This invariant proves that racing that two-step sequence
	// from concurrent callers never leaves the suggestion "stuck" (still
	// present in bee_name_suggestion after both calls finish) and never
	// deadlocks/panics - looped across 20 trials with 2 concurrent callers
	// each, since a single trial can't reliably distinguish a working
	// sequence from one that occasionally loses an update under a race.
	t.Run("ST-17_Live_ConcurrentAccept", func(t *testing.T) {
		s, db := bngLiveStore(t)
		const trials = 20

		for trial := 0; trial < trials; trial++ {
			name := bngUniqueName(fmt.Sprintf("racc%d", trial))
			if _, err := s.SubmitBeeName(name); err != nil {
				t.Fatalf("trial %d: seed SubmitBeeName() error = %v", trial, err)
			}

			var wg sync.WaitGroup
			errs := make([]error, 2)
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					_, errs[idx] = s.AcceptBeeNameSuggestion(name)
				}(i)
			}
			wg.Wait()

			if errs[0] != nil && errs[1] != nil {
				t.Errorf("trial %d: both concurrent AcceptBeeNameSuggestion calls failed: %v, %v", trial, errs[0], errs[1])
			}

			if n := bngCountByName(t, db, "bee_name_suggestion", name); n != 0 {
				t.Errorf("trial %d: bee_name_suggestion has %d rows for %q after both accepts, want 0", trial, n, name)
			}
			if n := bngCountByName(t, db, "bee_name", name); n < 1 {
				t.Errorf("trial %d: bee_name has %d rows for %q after both accepts, want >= 1", trial, n, name)
			}

			_, _ = s.DeleteBeeName(name)
		}
	})
}

// -------------- RejectBeeNameSuggestion --------------

func TestST18to20_RejectBeeNameSuggestion(t *testing.T) {
	t.Run("ST-18_Unreachable", func(t *testing.T) {
		s := bngStoreWithUnreachableDB(t)
		_, err := s.RejectBeeNameSuggestion(bngUniqueName("rej"))
		if err == nil {
			t.Error("expected a raw connection error")
		}
	})

	t.Run("ST-19_Live_OK", func(t *testing.T) {
		s, db := bngLiveStore(t)
		name := bngUniqueName("rej")
		if _, err := s.SubmitBeeName(name); err != nil {
			t.Fatalf("seed SubmitBeeName() error = %v", err)
		}

		got, err := s.RejectBeeNameSuggestion(name)
		if err != nil {
			t.Fatalf("RejectBeeNameSuggestion() error = %v", err)
		}
		if got != name {
			t.Errorf("RejectBeeNameSuggestion() = %q, want %q", got, name)
		}
		if n := bngCountByName(t, db, "bee_name_suggestion", name); n != 0 {
			t.Errorf("bee_name_suggestion has %d rows for %q after reject, want 0", n, name)
		}
	})

	t.Run("ST-20_Live_NonExistent", func(t *testing.T) {
		s, _ := bngLiveStore(t)
		name := bngUniqueName("rej-missing")
		got, err := s.RejectBeeNameSuggestion(name)
		if err != nil {
			t.Fatalf("RejectBeeNameSuggestion() of a non-existent name error = %v, want nil", err)
		}
		if got != name {
			t.Errorf("RejectBeeNameSuggestion() = %q, want %q", got, name)
		}
	})
}
