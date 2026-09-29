package petpictures

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// ppLiveDB skips the test unless DATABASE_URL is set. store.go's methods
// each dial the database directly from that env var (see NewStore's unused
// db field), so a live-DB test needs nothing more than the env var already
// being correct.
func ppLiveDB(t *testing.T) {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set; skipping live-Postgres test")
	}
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
	t.Run("ST-02_HappyPath", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		name := ppUniqueID("pet")
		pet, err := s.CreatePet(name)
		if err != nil {
			t.Fatalf("CreatePet() error = %v, want nil", err)
		}
		if pet.Name != name || pet.ID == 0 || pet.ProfilePicture != nil {
			t.Errorf("CreatePet() = %+v, want Name=%q, nonzero ID, nil ProfilePicture", pet, name)
		}
	})

	t.Run("ST-03_DuplicateNameErrors", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		name := ppUniqueID("pet")
		if _, err := s.CreatePet(name); err != nil {
			t.Fatalf("first CreatePet() error = %v, want nil", err)
		}
		if _, err := s.CreatePet(name); err == nil {
			t.Error("second CreatePet() error = nil, want a unique constraint violation")
		}
	})

	t.Run("ST-04_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.CreatePet(ppUniqueID("pet")); err == nil {
			t.Error("CreatePet() error = nil, want a connection error")
		}
	})

	t.Run("ST-05_ConcurrentSameNameOnlyOneSucceeds", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		// 5 rounds of 8 truly concurrent callers each, a fresh name per
		// round: a single round with several simultaneous racers already
		// exercises the unique constraint hard, and repeating it rules out
		// one lucky scheduling order slipping a broken guard past a single
		// trial.
		const rounds = 5
		const callers = 8
		for r := 0; r < rounds; r++ {
			name := ppUniqueID("pet-race")
			var wg sync.WaitGroup
			successes := make([]bool, callers)
			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, err := s.CreatePet(name)
					successes[i] = err == nil
				}(i)
			}
			wg.Wait()
			count := 0
			for _, ok := range successes {
				if ok {
					count++
				}
			}
			if count != 1 {
				t.Errorf("round %d: %d/%d CreatePet(%q) calls succeeded, want exactly 1", r, count, callers, name)
			}
		}
	})
}

func TestST25_CreatePetEmptyNameErrors(t *testing.T) {
	t.Run("ST-25_EmptyNameErrors", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		if _, err := s.CreatePet(""); !errors.Is(err, ErrPetNameEmpty) {
			t.Errorf("CreatePet(\"\") error = %v, want ErrPetNameEmpty", err)
		}
	})
}

func TestST06_GetPet(t *testing.T) {
	t.Run("ST-06_NoMatchingRowReturnsErrNoRows", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		// id is a serial column starting at 1, so -1 can never match a row.
		if _, err := s.GetPet(-1); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("GetPet() error = %v, want pgx.ErrNoRows", err)
		}
	})
}

func TestST07to09_GetPetByName(t *testing.T) {
	t.Run("ST-07_HappyPath", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		name := ppUniqueID("pet")
		created, err := s.CreatePet(name)
		if err != nil {
			t.Fatalf("CreatePet() error = %v, want nil", err)
		}
		got, err := s.GetPetByName(name)
		if err != nil {
			t.Fatalf("GetPetByName() error = %v, want nil", err)
		}
		if *got != *created {
			t.Errorf("GetPetByName() = %+v, want %+v", got, created)
		}
	})

	t.Run("ST-08_NoMatchingRowReturnsErrNoRows", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		if _, err := s.GetPetByName(ppUniqueID("missing-pet")); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("GetPetByName() error = %v, want pgx.ErrNoRows", err)
		}
	})

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
	// would hang the suite.

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
	// so no test is written that would hang the suite.

	t.Run("ST-15_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.CreatePetPicture(ppUniqueID("pic"), "jpg", 1, nil, nil); err == nil {
			t.Error("CreatePetPicture() error = nil, want a connection error")
		}
	})
}

func TestST17to18_GetRandPetPictureByName(t *testing.T) {
	t.Run("ST-17_PetNotFound", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		if _, err := s.GetRandPetPictureByName(ppUniqueID("missing-pet")); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("GetRandPetPictureByName() error = %v, want pgx.ErrNoRows (from upstream GetPetByName)", err)
		}
	})

	t.Run("ST-18_PetExistsNoPictures", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		pet, err := s.CreatePet(ppUniqueID("pet-no-pics"))
		if err != nil {
			t.Fatalf("CreatePet() error = %v, want nil", err)
		}
		if _, err := s.GetRandPetPictureByName(pet.Name); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("GetRandPetPictureByName() error = %v, want pgx.ErrNoRows", err)
		}
	})
}

func TestST19to20_GetPetPicture(t *testing.T) {
	t.Run("ST-19_NoMatchingRowReturnsErrNoRows", func(t *testing.T) {
		ppLiveDB(t)
		s := &store{}
		if _, err := s.GetPetPicture(ppUniqueID("missing-pic")); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("GetPetPicture() error = %v, want pgx.ErrNoRows", err)
		}
	})

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
	// would hang the suite.

	t.Run("ST-24_ConnectionFailure", func(t *testing.T) {
		ppSetUnreachableDatabaseURL(t)
		s := &store{}
		if _, err := s.DeletePetPicture("anything"); err == nil {
			t.Error("DeletePetPicture() error = nil, want a connection error")
		}
	})
}
