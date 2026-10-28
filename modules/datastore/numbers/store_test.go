package numbersds

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NeuralNexusDev/neuralnexus-api/db"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

const (
	ndsAccountID  = 930000000000000001
	ndsStoreIDMin = 930000000000000100
	ndsStoreIDMax = 930000000000000199
	ndsMissingID  = "930000000000009999"
)

func ndsLiveStore(t *testing.T) (*numberStore, string, string) {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testutil.PostgresURL(t, "numtest"))
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := context.Background()
	schema, err := db.FS.ReadFile("neuralnexus.sql")
	if err != nil {
		t.Fatalf("failed to read the schema: %v", err)
	}
	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		t.Fatalf("failed to apply the schema: %v", err)
	}

	cleanup := func() {
		pool.Exec(ctx, "DELETE FROM datastore_numbers WHERE store_id BETWEEN $1 AND $2", int64(ndsStoreIDMin), int64(ndsStoreIDMax))
		pool.Exec(ctx, "DELETE FROM datastores WHERE store_id BETWEEN $1 AND $2", int64(ndsStoreIDMin), int64(ndsStoreIDMax))
		pool.Exec(ctx, "DELETE FROM accounts WHERE user_id = $1", int64(ndsAccountID))
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx, "INSERT INTO accounts (user_id) VALUES ($1)", int64(ndsAccountID)); err != nil {
		t.Fatalf("failed to insert the account: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO datastores (store_id, owner_id) VALUES ($1, $2)", int64(ndsStoreIDMin), int64(ndsAccountID)); err != nil {
		t.Fatalf("failed to insert the datastore: %v", err)
	}
	return &numberStore{db: pool}, "930000000000000100", "930000000000000001"
}

func TestNumberStoreErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("NDS-01_CreateDuplicateExists", func(t *testing.T) {
		s, storeID, userID := ndsLiveStore(t)
		if _, err := s.Create(ctx, storeID, userID, 1); err != nil {
			t.Fatalf("first Create failed: %v", err)
		}
		if _, err := s.Create(ctx, storeID, userID, 2); !errors.Is(err, ErrNumberExists) {
			t.Fatalf("second Create error = %v, want ErrNumberExists", err)
		}
	})

	t.Run("NDS-02_CreateUnknownStoreNotFound", func(t *testing.T) {
		s, _, userID := ndsLiveStore(t)
		if _, err := s.Create(ctx, ndsMissingID, userID, 1); !errors.Is(err, ErrNumberStoreOrUserNotFound) {
			t.Fatalf("Create error = %v, want ErrNumberStoreOrUserNotFound", err)
		}
	})

	t.Run("NDS-03_CreateUnknownUserNotFound", func(t *testing.T) {
		s, storeID, _ := ndsLiveStore(t)
		if _, err := s.Create(ctx, storeID, ndsMissingID, 1); !errors.Is(err, ErrNumberStoreOrUserNotFound) {
			t.Fatalf("Create error = %v, want ErrNumberStoreOrUserNotFound", err)
		}
	})

	t.Run("NDS-04_UpdateMissingRowNotFound", func(t *testing.T) {
		s, storeID, userID := ndsLiveStore(t)
		if _, err := s.Update(ctx, storeID, userID, 5); !errors.Is(err, ErrNumberNotFound) {
			t.Fatalf("Update error = %v, want ErrNumberNotFound", err)
		}
	})

	t.Run("NDS-05_DeleteMissingRowNotFound", func(t *testing.T) {
		s, storeID, userID := ndsLiveStore(t)
		if err := s.Delete(ctx, storeID, userID); !errors.Is(err, ErrNumberNotFound) {
			t.Fatalf("Delete error = %v, want ErrNumberNotFound", err)
		}
	})

	t.Run("NDS-06_DeleteExistingRow", func(t *testing.T) {
		s, storeID, userID := ndsLiveStore(t)
		if _, err := s.Create(ctx, storeID, userID, 1); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if err := s.Delete(ctx, storeID, userID); err != nil {
			t.Fatalf("Delete error = %v, want nil", err)
		}
		if _, err := s.Read(ctx, storeID, userID); !errors.Is(err, ErrNumberNotFound) {
			t.Fatalf("Read after Delete error = %v, want ErrNumberNotFound", err)
		}
	})

	t.Run("NDS-07_NonNumericIDsInvalid", func(t *testing.T) {
		s, _, userID := ndsLiveStore(t)
		if _, err := s.Create(ctx, "abc", userID, 1); !errors.Is(err, ErrNumberInvalidID) {
			t.Errorf("Create error = %v, want ErrNumberInvalidID", err)
		}
		if _, err := s.Read(ctx, "abc", userID); !errors.Is(err, ErrNumberInvalidID) {
			t.Errorf("Read error = %v, want ErrNumberInvalidID", err)
		}
		if _, err := s.Update(ctx, "abc", userID, 1); !errors.Is(err, ErrNumberInvalidID) {
			t.Errorf("Update error = %v, want ErrNumberInvalidID", err)
		}
		if err := s.Delete(ctx, "abc", userID); !errors.Is(err, ErrNumberInvalidID) {
			t.Errorf("Delete error = %v, want ErrNumberInvalidID", err)
		}
	})
}
