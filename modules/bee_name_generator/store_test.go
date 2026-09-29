package beenamegenerator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// bngUnusedTCPPort returns a port that's very likely free when returned, so
// a connection attempt against it fails fast ("connection refused") rather
// than hanging.
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

// bngTruncatingConn relays the server's response to the first query written
// after armed is set with its last 25 bytes cut off (the trailing
// CommandComplete/ReadyForQuery and part of the last DataRow), then fails the
// next read, so the client hits an error mid-result.
type bngTruncatingConn struct {
	net.Conn
	armed   *atomic.Bool
	queried bool
	pending []byte
	failed  bool
}

func (c *bngTruncatingConn) Write(b []byte) (int, error) {
	if c.armed.Load() {
		c.queried = true
	}
	return c.Conn.Write(b)
}

func (c *bngTruncatingConn) Read(b []byte) (int, error) {
	if len(c.pending) > 0 {
		n := copy(b, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}
	if c.failed {
		return 0, io.ErrUnexpectedEOF
	}
	if !c.queried {
		return c.Conn.Read(b)
	}
	readyForQuery := []byte{'Z', 0, 0, 0, 5, 'I'}
	var resp []byte
	buf := make([]byte, 4096)
	for !bytes.HasSuffix(resp, readyForQuery) {
		n, err := c.Conn.Read(buf)
		resp = append(resp, buf[:n]...)
		if err != nil {
			return 0, err
		}
	}
	const tail = 25
	c.pending, c.failed, c.queried = resp[:len(resp)-tail], true, false
	return c.Read(b)
}

func bngPoolCuttingFirstResult(t *testing.T) (*pgxpool.Pool, *atomic.Bool) {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set; skipping live-Postgres test")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("failed to parse TEST_POSTGRES_URL: %v", err)
	}
	var armed atomic.Bool
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &bngTruncatingConn{Conn: conn, armed: &armed}, nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("failed to warm the pool: %v", err)
	}
	return pool, &armed
}

func bngUniqueName(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, strings.ReplaceAll(uuid.New().String(), "-", ""))
}

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

func TestST05to06and21and24_UploadBeeName(t *testing.T) {
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

	t.Run("ST-21_Live_ConcurrentUpload", func(t *testing.T) {
		s, db := bngLiveStore(t)
		const trials = 20

		for trial := 0; trial < trials; trial++ {
			name := bngUniqueName(fmt.Sprintf("rup%d", trial))

			var wg sync.WaitGroup
			errs := make([]error, 2)
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					_, errs[idx] = s.UploadBeeName(name)
				}(i)
			}
			wg.Wait()

			if (errs[0] == nil) == (errs[1] == nil) {
				t.Errorf("trial %d: want exactly one concurrent UploadBeeName call to succeed, got errs = %v, %v", trial, errs[0], errs[1])
			}

			if n := bngCountByName(t, db, "bee_name", name); n != 1 {
				t.Errorf("trial %d: bee_name has %d rows for %q after both uploads, want exactly 1", trial, n, name)
			}

			_, _ = s.DeleteBeeName(name)
		}
	})

	t.Run("ST-24_Live_WhitespaceOnlyNameRejected", func(t *testing.T) {
		s, db := bngLiveStore(t)
		blank := "  \t "
		t.Cleanup(func() {
			_, _ = db.Exec(context.Background(), "DELETE FROM bee_name WHERE name = $1", blank)
		})

		got, err := s.UploadBeeName(blank)
		if err == nil {
			t.Error("UploadBeeName() error = nil, want the CHECK constraint violation")
		}
		if got != "" {
			t.Errorf("UploadBeeName() = %q, want an empty string on error", got)
		}
		if n := bngCountByName(t, db, "bee_name", blank); n != 0 {
			t.Errorf("bee_name holds %d rows named %q, want 0", n, blank)
		}
	})
}

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

func TestST10to11and22and25_SubmitBeeName(t *testing.T) {
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

	t.Run("ST-22_Live_ConcurrentSubmit", func(t *testing.T) {
		s, db := bngLiveStore(t)
		const trials = 20

		for trial := 0; trial < trials; trial++ {
			name := bngUniqueName(fmt.Sprintf("rsub%d", trial))

			var wg sync.WaitGroup
			errs := make([]error, 2)
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					_, errs[idx] = s.SubmitBeeName(name)
				}(i)
			}
			wg.Wait()

			if (errs[0] == nil) == (errs[1] == nil) {
				t.Errorf("trial %d: want exactly one concurrent SubmitBeeName call to succeed, got errs = %v, %v", trial, errs[0], errs[1])
			}

			if n := bngCountByName(t, db, "bee_name_suggestion", name); n != 1 {
				t.Errorf("trial %d: bee_name_suggestion has %d rows for %q after both submits, want exactly 1", trial, n, name)
			}

			_, _ = s.RejectBeeNameSuggestion(name)
		}
	})

	t.Run("ST-25_Live_WhitespaceOnlyNameRejected", func(t *testing.T) {
		s, db := bngLiveStore(t)
		blank := "  \t "
		t.Cleanup(func() {
			_, _ = db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", blank)
		})

		got, err := s.SubmitBeeName(blank)
		if err == nil {
			t.Error("SubmitBeeName() error = nil, want the CHECK constraint violation")
		}
		if got != "" {
			t.Errorf("SubmitBeeName() = %q, want an empty string on error", got)
		}
		if n := bngCountByName(t, db, "bee_name_suggestion", blank); n != 0 {
			t.Errorf("bee_name_suggestion holds %d rows named %q, want 0", n, blank)
		}
	})
}

func TestST12to14and23_GetBeeNameSuggestions(t *testing.T) {
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

	t.Run("ST-23_Live_IterationError", func(t *testing.T) {
		seed, db := bngLiveStore(t)
		bngClearTable(t, db, "bee_name_suggestion")
		names := make([]string, 5)
		for i := range names {
			names[i] = bngUniqueName("sugg")
			if _, err := seed.SubmitBeeName(names[i]); err != nil {
				t.Fatalf("seed SubmitBeeName() error = %v", err)
			}
		}
		t.Cleanup(func() {
			for _, name := range names {
				_, _ = seed.RejectBeeNameSuggestion(name)
			}
		})

		pool, armed := bngPoolCuttingFirstResult(t)
		armed.Store(true)
		got, err := (&store{db: pool}).GetBeeNameSuggestions(int64(len(names)))
		if err == nil {
			t.Error("GetBeeNameSuggestions() error = nil, want the iteration error")
		}
		if got != nil {
			t.Errorf("got %v, want nil alongside an iteration error", got)
		}
	})
}

func TestST15to17and26_AcceptBeeNameSuggestion(t *testing.T) {
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

			if (errs[0] == nil) == (errs[1] == nil) {
				t.Errorf("trial %d: want exactly one concurrent AcceptBeeNameSuggestion call to succeed, got errs = %v, %v", trial, errs[0], errs[1])
			}

			if n := bngCountByName(t, db, "bee_name_suggestion", name); n != 0 {
				t.Errorf("trial %d: bee_name_suggestion has %d rows for %q after both accepts, want 0", trial, n, name)
			}
			if n := bngCountByName(t, db, "bee_name", name); n != 1 {
				t.Errorf("trial %d: bee_name has %d rows for %q after both accepts, want exactly 1", trial, n, name)
			}

			_, _ = s.DeleteBeeName(name)
		}
	})

	t.Run("ST-26_Live_WhitespaceOnlyNameRejected", func(t *testing.T) {
		s, db := bngLiveStore(t)
		blank := "  \t "
		t.Cleanup(func() {
			_, _ = db.Exec(context.Background(), "DELETE FROM bee_name WHERE name = $1", blank)
		})

		got, err := s.AcceptBeeNameSuggestion(blank)
		if err == nil {
			t.Error("AcceptBeeNameSuggestion() error = nil, want the CHECK constraint violation")
		}
		if got != "" {
			t.Errorf("AcceptBeeNameSuggestion() = %q, want an empty string on error", got)
		}
		if n := bngCountByName(t, db, "bee_name", blank); n != 0 {
			t.Errorf("bee_name holds %d rows named %q, want 0", n, blank)
		}
	})
}

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
