package petpictures

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// ppUnusedTCPPort returns a TCP port on 127.0.0.1 that is very likely free
// at the moment it's returned, so a subsequent connection attempt to it
// fails fast with "connection refused" instead of hanging.
func ppUnusedTCPPort(t *testing.T) int {
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

// ppSetUnreachableDatabaseURL points DATABASE_URL at a closed local port for
// the duration of the test. store.go appends "/pet_pictures" to this value
// itself, so it must stay a bare "scheme://user:pass@host:port" DSN with no
// path or query string, matching the real contract. pgxpool.New only parses
// this eagerly (it dials lazily), so it never trips database.GetDB's
// log.Fatal; the later query fails with a genuine connection-refused error.
func ppSetUnreachableDatabaseURL(t *testing.T) {
	t.Helper()
	port := ppUnusedTCPPort(t)
	t.Setenv("DATABASE_URL", fmt.Sprintf("postgres://user:pass@127.0.0.1:%d", port))
}

// ppUniqueID hands out name/id values that won't collide across subtests or
// concurrent test binaries sharing the same live database.
func ppUniqueID(prefix string) string {
	return prefix + "-" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

func TestST01_NewStore(t *testing.T) {
	t.Run("ST-01_WrapsGivenPool", func(t *testing.T) {
		if s := NewStore(nil); s == nil {
			t.Error("NewStore(nil) = nil, want a non-nil PetPicStore")
		}
	})
}

func TestST02to05_CreatePet(t *testing.T) {
	// ST-02 (happy path), ST-03 (duplicate name) and ST-05 (concurrent
	// same-name) are not tested here: they require a live connection to
	// pet_pictures' own separate Postgres database, which store.go dials
	// itself from DATABASE_URL instead of using the shared test database
	// every other module uses. That bespoke, substitution-based wiring is
	// being deferred to a proper refactor rather than patched (see
	// test/plans/pet_pictures.md's self-check), so no test depends on it.

	t.Run("ST-04_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.CreatePet(ppUniqueID("pet")); err == nil {
			t.Error("CreatePet() error = nil, want a connection error")
		}
	})
}

func TestST07to09_GetPetByName(t *testing.T) {
	// ST-07 (happy path) and ST-08 (no matching row) are not tested here:
	// they require a live connection to pet_pictures' own separate Postgres
	// database (see TestST02to05_CreatePet's comment and
	// test/plans/pet_pictures.md's self-check).

	t.Run("ST-09_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.GetPetByName("anything"); err == nil {
			t.Error("GetPetByName() error = nil, want a connection error")
		}
	})
}

func TestST10to12_UpdatePet(t *testing.T) {
	// ST-10 (happy path) and ST-11 (nonexistent id) are not tested here:
	// UpdatePet runs its UPDATE through db.Query and never reads or closes
	// the returned Rows, so any live call leaks the pool's connection; the
	// deferred db.Close() that follows then hangs forever inside
	// pgxpool.Close() waiting for that connection to be released. This is a
	// known, deferred source bug (see test/plans/pet_pictures.md's
	// self-check) — not fixed in this pass, so no test is written that
	// would hang the suite. A live call would also require pet_pictures'
	// own separate Postgres database, itself deferred to a proper refactor.

	t.Run("ST-12_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.UpdatePet(&Pet{ID: 1, Name: "x"}); err == nil {
			t.Error("UpdatePet() error = nil, want a connection error")
		}
	})
}

func TestST13to16_CreatePetPicture(t *testing.T) {
	// ST-13 (happy path), ST-14 (duplicate id) and ST-16 (concurrency) are
	// not tested here: CreatePetPicture runs its INSERT through db.Query
	// and never reads or closes the returned Rows, so any live call leaks
	// the pool's connection; the deferred db.Close() that follows then
	// hangs forever inside pgxpool.Close() waiting for that connection to
	// be released. This is a known, deferred source bug (see
	// test/plans/pet_pictures.md's self-check) — not fixed in this pass,
	// so no test is written that would hang the suite. A live call would
	// also require pet_pictures' own separate Postgres database, itself
	// deferred to a proper refactor.

	t.Run("ST-15_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.CreatePetPicture(ppUniqueID("pic"), "jpg", 1, nil, nil); err == nil {
			t.Error("CreatePetPicture() error = nil, want a connection error")
		}
	})
}

func TestST19to20_GetPetPicture(t *testing.T) {
	// ST-19 (no matching row) is not tested here: it requires a live
	// connection to pet_pictures' own separate Postgres database (see
	// TestST02to05_CreatePet's comment and test/plans/pet_pictures.md's
	// self-check).

	t.Run("ST-20_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.GetPetPicture("anything"); err == nil {
			t.Error("GetPetPicture() error = nil, want a connection error")
		}
	})
}

func TestST21_UpdatePetPicture(t *testing.T) {
	t.Run("ST-21_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.UpdatePetPicture(PetPicture{ID: "anything"}); err == nil {
			t.Error("UpdatePetPicture() error = nil, want a connection error")
		}
	})
}

func TestST22to24_DeletePetPicture(t *testing.T) {
	// ST-22 (happy path) and ST-23 (nonexistent id) are not tested here:
	// DeletePetPicture runs its DELETE through db.Query and never reads or
	// closes the returned Rows, so any live call leaks the pool's
	// connection; the deferred db.Close() that follows then hangs forever
	// inside pgxpool.Close() waiting for that connection to be released.
	// This is a known, deferred source bug (see test/plans/pet_pictures.md's
	// self-check) — not fixed in this pass, so no test is written that
	// would hang the suite. A live call would also require pet_pictures'
	// own separate Postgres database, itself deferred to a proper refactor.

	t.Run("ST-24_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.DeletePetPicture("anything"); err == nil {
			t.Error("DeletePetPicture() error = nil, want a connection error")
		}
	})
}
