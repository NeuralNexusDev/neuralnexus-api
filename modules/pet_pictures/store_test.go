package petpictures

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ppCtx = context.Background()

func ppUnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	port := testutil.UnusedTCPPort(t)
	cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://user:pass@127.0.0.1:%d/pet_pictures?sslmode=disable&connect_timeout=2", port))
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

func ppLiveDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testutil.PostgresURL(t, "pet_pictures"))
	if err != nil {
		t.Fatalf("failed to connect to pet_pictures: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func ppCreatePet(t *testing.T, pool *pgxpool.Pool, s *store, name string) *Pet {
	t.Helper()
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pets WHERE name = $1", name) })
	pet, err := s.CreatePet(ppCtx, name)
	if err != nil {
		t.Fatalf("CreatePet(%q) error = %v", name, err)
	}
	return pet
}

func ppCreatePicture(t *testing.T, pool *pgxpool.Pool, s *store, id string, primary int, others []int, aliases []string) {
	t.Helper()
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pictures WHERE id = $1", id) })
	if _, err := s.CreatePetPicture(ppCtx, id, "jpg", primary, others, aliases); err != nil {
		t.Fatalf("CreatePetPicture(%q) error = %v", id, err)
	}
}

var ppSessionTimeZones = []string{"America/Los_Angeles", "Asia/Kolkata"}

func ppAssertCreatedMatchesDatabase(t *testing.T, pool *pgxpool.Pool, id, got string) {
	t.Helper()
	var want time.Time
	if err := pool.QueryRow(context.Background(), "SELECT created_at FROM pictures WHERE id = $1", id).Scan(&want); err != nil {
		t.Fatalf("failed to read created_at for %q: %v", id, err)
	}
	created, err := time.Parse(time.RFC3339Nano, got)
	if err != nil || !strings.HasSuffix(got, "Z") || !created.Equal(want) {
		t.Errorf("Created = %q, want the instant %s as RFC3339 UTC ending in Z (parse error: %v)", got, want.UTC().Format(time.RFC3339Nano), err)
	}
}

func ppUniqueID(prefix string) string {
	return prefix + "-" + strings.ReplaceAll(uuid.New().String(), "-", "")
}

func TestST02to05and26_CreatePet(t *testing.T) {
	t.Run("PPS-04_ConnectionFailure", func(t *testing.T) {
		s := &store{db: ppUnreachablePool(t)}
		if _, err := s.CreatePet(ppCtx, ppUniqueID("pet")); err == nil {
			t.Error("CreatePet() error = nil, want a connection error")
		}
	})

	t.Run("PPS-02_HappyPath", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		name := ppUniqueID("pet")
		pet := ppCreatePet(t, pool, s, name)
		if pet.Name != name || pet.ID == 0 || pet.ProfilePicture != nil {
			t.Errorf("CreatePet() = %+v, want Name %q, nonzero ID, nil ProfilePicture", pet, name)
		}
	})

	t.Run("PPS-03_DuplicateName", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		name := ppUniqueID("pet")
		ppCreatePet(t, pool, s, name)
		pet, err := s.CreatePet(ppCtx, name)
		if pet != nil || err == nil {
			t.Errorf("CreatePet(duplicate) = (%v, %v), want (nil, error)", pet, err)
		}
	})

	t.Run("PPS-05_ConcurrentSameName", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		for round := 0; round < 5; round++ {
			name := ppUniqueID("pet")
			t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pets WHERE name = $1", name) })
			const callers = 8
			results := make([]*Pet, callers)
			errs := make([]error, callers)
			var wg sync.WaitGroup
			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					results[i], errs[i] = s.CreatePet(ppCtx, name)
				}(i)
			}
			wg.Wait()
			succeeded := 0
			for i := range results {
				if errs[i] == nil && results[i] != nil && results[i].ID != 0 {
					succeeded++
				}
			}
			if succeeded != 1 {
				t.Fatalf("round %d: %d CreatePet calls succeeded, want exactly 1", round, succeeded)
			}
		}
	})

	t.Run("PPS-26_EmptyName", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pets WHERE name = ''") })
		s := &store{db: pool}
		pet, err := s.CreatePet(ppCtx, "")
		if pet != nil || !errors.Is(err, ErrPetNameEmpty) {
			t.Errorf("CreatePet(\"\") = (%v, %v), want (nil, ErrPetNameEmpty)", pet, err)
		}
	})
}

func TestST06to09and27_GetPetAndGetPetByName(t *testing.T) {
	t.Run("PPS-09_ConnectionFailure", func(t *testing.T) {
		s := &store{db: ppUnreachablePool(t)}
		if _, err := s.GetPetByName(ppCtx, "anything"); err == nil {
			t.Error("GetPetByName() error = nil, want a connection error")
		}
	})

	t.Run("PPS-06_GetPetNoRow", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		pet, err := s.GetPet(ppCtx, -1)
		if pet != nil || !errors.Is(err, ErrPetNotFound) {
			t.Errorf("GetPet(-1) = (%v, %v), want (nil, ErrPetNotFound)", pet, err)
		}
	})

	t.Run("PPS-27_GetPetHappyPath", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		created := ppCreatePet(t, pool, s, ppUniqueID("pet"))
		got, err := s.GetPet(ppCtx, created.ID)
		if err != nil || !reflect.DeepEqual(got, created) {
			t.Errorf("GetPet() = (%+v, %v), want (%+v, nil)", got, err, created)
		}
	})

	t.Run("PPS-07_HappyPath", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		created := ppCreatePet(t, pool, s, ppUniqueID("pet"))
		got, err := s.GetPetByName(ppCtx, created.Name)
		if err != nil || !reflect.DeepEqual(got, created) {
			t.Errorf("GetPetByName() = (%+v, %v), want (%+v, nil)", got, err, created)
		}
	})

	t.Run("PPS-08_NoRow", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		pet, err := s.GetPetByName(ppCtx, ppUniqueID("missing"))
		if pet != nil || !errors.Is(err, ErrPetNotFound) {
			t.Errorf("GetPetByName(missing) = (%v, %v), want (nil, ErrPetNotFound)", pet, err)
		}
	})
}

func TestST10to12_UpdatePet(t *testing.T) {
	t.Run("PPS-12_ConnectionFailure", func(t *testing.T) {
		s := &store{db: ppUnreachablePool(t)}
		if _, err := s.UpdatePet(ppCtx, &Pet{ID: 1, Name: "x"}); err == nil {
			t.Error("UpdatePet() error = nil, want a connection error")
		}
	})

	t.Run("PPS-10_HappyPath", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		created := ppCreatePet(t, pool, s, ppUniqueID("pet"))
		newName := ppUniqueID("renamed")
		t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pets WHERE name = $1", newName) })
		pic := "pic.jpg"
		in := &Pet{ID: created.ID, Name: newName, ProfilePicture: &pic}
		got, err := s.UpdatePet(ppCtx, in)
		if err != nil || got != in {
			t.Fatalf("UpdatePet() = (%v, %v), want the same pet and nil error", got, err)
		}
		after, err := s.GetPetByName(ppCtx, newName)
		if err != nil || after.ID != created.ID || after.ProfilePicture == nil || *after.ProfilePicture != pic {
			t.Errorf("GetPetByName(new name) = (%+v, %v), want the updated row", after, err)
		}
	})

	t.Run("PPS-11_NoMatchingRow", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		got, err := s.UpdatePet(ppCtx, &Pet{ID: -1, Name: ppUniqueID("ghost")})
		if got != nil || !errors.Is(err, ErrPetNotFound) {
			t.Errorf("UpdatePet(no row) = (%v, %v), want (nil, ErrPetNotFound)", got, err)
		}
	})
}

func TestST13to16_CreatePetPicture(t *testing.T) {
	t.Run("PPS-15_ConnectionFailure", func(t *testing.T) {
		s := &store{db: ppUnreachablePool(t)}
		if _, err := s.CreatePetPicture(ppCtx, ppUniqueID("pic"), "jpg", 1, nil, nil); err == nil {
			t.Error("CreatePetPicture() error = nil, want a connection error")
		}
	})

	t.Run("PPS-13_HappyPath", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		id := ppUniqueID("pic")
		t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pictures WHERE id = $1", id) })
		got, err := s.CreatePetPicture(ppCtx, id, "png", 3, []int{4, 5}, []string{"a", "b"})
		want := &PetPicture{ID: id, FileExt: "png", PrimarySubject: 3, OthersSubjects: []int{4, 5}, Aliases: []string{"a", "b"}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("CreatePetPicture() = (%+v, %v), want (%+v, nil)", got, err, want)
		}
	})

	t.Run("PPS-31_NoOtherSubjects", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		id := ppUniqueID("pic")
		ppCreatePicture(t, pool, s, id, 6, nil, nil)
		var subjects []int
		if err := pool.QueryRow(ppCtx, "SELECT subjects FROM pictures WHERE id = $1", id).Scan(&subjects); err != nil || !reflect.DeepEqual(subjects, []int{6}) {
			t.Fatalf("subjects = (%v, %v), want ([6], nil)", subjects, err)
		}
		got, err := s.GetPetPicture(ppCtx, id)
		if err != nil || got.PrimarySubject != 6 || got.OthersSubjects != nil {
			t.Errorf("GetPetPicture() = (%+v, %v), want PrimarySubject 6 and nil OthersSubjects", got, err)
		}
	})

	t.Run("PPS-32_SubjectsOrder", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		id := ppUniqueID("pic")
		ppCreatePicture(t, pool, s, id, 3, []int{4, 5}, nil)
		var subjects []int
		if err := pool.QueryRow(ppCtx, "SELECT subjects FROM pictures WHERE id = $1", id).Scan(&subjects); err != nil || !reflect.DeepEqual(subjects, []int{3, 4, 5}) {
			t.Fatalf("subjects = (%v, %v), want ([3 4 5], nil)", subjects, err)
		}
		got, err := s.GetPetPicture(ppCtx, id)
		if err != nil || got.PrimarySubject != 3 || !reflect.DeepEqual(got.OthersSubjects, []int{4, 5}) {
			t.Errorf("GetPetPicture() = (%+v, %v), want PrimarySubject 3 and OthersSubjects [4 5]", got, err)
		}
	})

	t.Run("PPS-33_EmptySubjectsRejected", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		id := ppUniqueID("pic")
		t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pictures WHERE id = $1", id) })
		_, err := pool.Exec(ppCtx, "INSERT INTO pictures (id, file_ext, subjects) VALUES ($1, 'jpg', '{}')", id)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "subjects_check" {
			t.Errorf("INSERT with empty subjects error = %v, want check violation on subjects_check", err)
		}
	})

	t.Run("PPS-14_DuplicateID", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		id := ppUniqueID("pic")
		ppCreatePicture(t, pool, s, id, 1, nil, nil)
		got, err := s.CreatePetPicture(ppCtx, id, "jpg", 1, nil, nil)
		if got != nil || err == nil {
			t.Errorf("CreatePetPicture(duplicate) = (%v, %v), want (nil, error)", got, err)
		}
	})

	t.Run("PPS-16_ConcurrentSameID", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		for round := 0; round < 5; round++ {
			id := ppUniqueID("pic")
			t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM pictures WHERE id = $1", id) })
			const callers = 8
			errs := make([]error, callers)
			var wg sync.WaitGroup
			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, errs[i] = s.CreatePetPicture(ppCtx, id, "jpg", 1, nil, nil)
				}(i)
			}
			wg.Wait()
			succeeded := 0
			for _, err := range errs {
				if err == nil {
					succeeded++
				}
			}
			if succeeded != 1 {
				t.Fatalf("round %d: %d CreatePetPicture calls succeeded, want exactly 1", round, succeeded)
			}
		}
	})
}

func TestST17to20and28to29_GetPetPictureAndGetRandPetPictureByName(t *testing.T) {
	t.Run("PPS-20_ConnectionFailure", func(t *testing.T) {
		s := &store{db: ppUnreachablePool(t)}
		if _, err := s.GetPetPicture(ppCtx, "anything"); err == nil {
			t.Error("GetPetPicture() error = nil, want a connection error")
		}
	})

	t.Run("PPS-17_RandPictureNoPet", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		got, err := s.GetRandPetPictureByName(ppCtx, ppUniqueID("missing"))
		if got != nil || !errors.Is(err, ErrPetNotFound) {
			t.Errorf("GetRandPetPictureByName(no pet) = (%v, %v), want (nil, ErrPetNotFound)", got, err)
		}
	})

	t.Run("PPS-18_RandPictureNoPictures", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		pet := ppCreatePet(t, pool, s, ppUniqueID("pet"))
		got, err := s.GetRandPetPictureByName(ppCtx, pet.Name)
		if got != nil || !errors.Is(err, ErrPetPictureNotFound) {
			t.Errorf("GetRandPetPictureByName(no pictures) = (%v, %v), want (nil, ErrPetPictureNotFound)", got, err)
		}
	})

	t.Run("PPS-19_NoRow", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		got, err := s.GetPetPicture(ppCtx, ppUniqueID("missing"))
		if got != nil || !errors.Is(err, ErrPetPictureNotFound) {
			t.Errorf("GetPetPicture(missing) = (%v, %v), want (nil, ErrPetPictureNotFound)", got, err)
		}
	})

	for _, tz := range ppSessionTimeZones {
		t.Run("PPS-28_GetPetPictureHappyPath_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			t.Setenv("PGTZ", tz)
			pool := ppLiveDatabase(t)
			s := &store{db: pool}
			id := ppUniqueID("pic")
			ppCreatePicture(t, pool, s, id, 7, []int{8}, []string{"x"})
			got, err := s.GetPetPicture(ppCtx, id)
			if err != nil || got.ID != id || got.FileExt != "jpg" || got.PrimarySubject != 7 ||
				!reflect.DeepEqual(got.OthersSubjects, []int{8}) || !reflect.DeepEqual(got.Aliases, []string{"x"}) {
				t.Fatalf("GetPetPicture() = (%+v, %v), want the created row", got, err)
			}
			ppAssertCreatedMatchesDatabase(t, pool, id, got.Created)
		})
	}

	t.Run("PPS-29_RandPictureHappyPath", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		pet := ppCreatePet(t, pool, s, ppUniqueID("pet"))
		id := ppUniqueID("pic")
		ppCreatePicture(t, pool, s, id, pet.ID, nil, nil)
		got, err := s.GetRandPetPictureByName(ppCtx, pet.Name)
		if err != nil || got.ID != id {
			t.Errorf("GetRandPetPictureByName() = (%+v, %v), want picture %q", got, err, id)
		}
	})
}

func TestST21to22and30_UpdatePetPicture(t *testing.T) {
	t.Run("PPS-21_ConnectionFailure", func(t *testing.T) {
		s := &store{db: ppUnreachablePool(t)}
		if _, err := s.UpdatePetPicture(ppCtx, PetPicture{ID: "anything"}); err == nil {
			t.Error("UpdatePetPicture() error = nil, want a connection error")
		}
	})

	t.Run("PPS-30_NoMatchingRow", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		got, err := s.UpdatePetPicture(ppCtx, PetPicture{ID: ppUniqueID("missing"), FileExt: "png", Created: "caller-supplied"})
		if got != nil || !errors.Is(err, ErrPetPictureNotFound) {
			t.Errorf("UpdatePetPicture(no row) = (%v, %v), want (nil, ErrPetPictureNotFound)", got, err)
		}
	})

	for _, tz := range ppSessionTimeZones {
		t.Run("PPS-22_HappyPath_"+strings.ReplaceAll(tz, "/", "_"), func(t *testing.T) {
			t.Setenv("PGTZ", tz)
			pool := ppLiveDatabase(t)
			s := &store{db: pool}
			id := ppUniqueID("pic")
			ppCreatePicture(t, pool, s, id, 1, nil, nil)
			in := PetPicture{ID: id, FileExt: "webp", PrimarySubject: 2, OthersSubjects: []int{3}, Aliases: []string{"z"}}
			got, err := s.UpdatePetPicture(ppCtx, in)
			if err != nil || got.ID != id || got.FileExt != "webp" || got.PrimarySubject != 2 ||
				!reflect.DeepEqual(got.OthersSubjects, []int{3}) || !reflect.DeepEqual(got.Aliases, []string{"z"}) {
				t.Fatalf("UpdatePetPicture() = (%+v, %v), want the updated fields and nil error", got, err)
			}
			ppAssertCreatedMatchesDatabase(t, pool, id, got.Created)
			after, err := s.GetPetPicture(ppCtx, id)
			if err != nil || after.FileExt != "webp" || after.PrimarySubject != 2 ||
				!reflect.DeepEqual(after.OthersSubjects, []int{3}) || !reflect.DeepEqual(after.Aliases, []string{"z"}) {
				t.Errorf("GetPetPicture() after update = (%+v, %v), want the updated fields", after, err)
			}
		})
	}
}

func TestST23to25_DeletePetPicture(t *testing.T) {
	t.Run("PPS-24_ConnectionFailure", func(t *testing.T) {
		s := &store{db: ppUnreachablePool(t)}
		if _, err := s.DeletePetPicture(ppCtx, "anything"); err == nil {
			t.Error("DeletePetPicture() error = nil, want a connection error")
		}
	})

	t.Run("PPS-23_HappyPath", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		id := ppUniqueID("pic")
		ppCreatePicture(t, pool, s, id, 1, nil, nil)
		got, err := s.DeletePetPicture(ppCtx, id)
		if err != nil || !reflect.DeepEqual(got, &PetPicture{ID: id}) {
			t.Fatalf("DeletePetPicture() = (%+v, %v), want (&PetPicture{ID: %q}, nil)", got, err, id)
		}
		if _, err := s.GetPetPicture(ppCtx, id); !errors.Is(err, ErrPetPictureNotFound) {
			t.Errorf("GetPetPicture() after delete error = %v, want ErrPetPictureNotFound", err)
		}
	})

	t.Run("PPS-25_NoMatchingRow", func(t *testing.T) {
		pool := ppLiveDatabase(t)
		s := &store{db: pool}
		id := ppUniqueID("missing")
		got, err := s.DeletePetPicture(ppCtx, id)
		if err != nil || !reflect.DeepEqual(got, &PetPicture{ID: id}) {
			t.Errorf("DeletePetPicture(missing) = (%+v, %v), want (&PetPicture{ID: %q}, nil)", got, err, id)
		}
	})
}
