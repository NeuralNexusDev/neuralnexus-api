package datastore

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NeuralNexusDev/neuralnexus-api/db"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

const (
	dsSchema    = "dsstore"
	dsOwnerID   = "910000000000070001"
	dsOtherID   = "910000000000070002"
	dsMissingID = "910000000000070999"
)

func newDSStore(t *testing.T) (DSStore, *pgxpool.Pool) {
	t.Helper()
	pgURL := testutil.PostgresURL(t, "neuralnexus")
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec(ctx, "DROP SCHEMA IF EXISTS "+dsSchema+" CASCADE")
		admin.Close(ctx)
	})
	schema, err := db.FS.ReadFile("neuralnexus.sql")
	if err != nil {
		t.Fatalf("failed to read the neuralnexus schema: %v", err)
	}
	for _, q := range []string{
		"DROP SCHEMA IF EXISTS " + dsSchema + " CASCADE",
		"CREATE SCHEMA " + dsSchema,
		"SET search_path TO " + dsSchema,
		string(schema),
		"INSERT INTO accounts (user_id) VALUES (" + dsOwnerID + "), (" + dsOtherID + ")",
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatalf("failed to prepare the %s schema: %v", dsSchema, err)
		}
	}
	cfg, err := pgxpool.ParseConfig(pgURL)
	if err != nil {
		t.Fatalf("failed to parse the postgres url: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = dsSchema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewStore(pool), pool
}

func TestDSS_DataStoreErrors(t *testing.T) {
	s, pool := newDSStore(t)
	ctx := context.Background()

	t.Run("DSS-01_DeleteUnknownStoreNotFound", func(t *testing.T) {
		if err := s.DeleteDataStore(ctx, dsMissingID); !errors.Is(err, ErrDataStoreNotFound) {
			t.Fatalf("err = %v, want ErrDataStoreNotFound", err)
		}
	})

	t.Run("DSS-02_DeleteExistingStore", func(t *testing.T) {
		const id = "910000000000071002"
		if _, err := s.CreateNewDataStore(ctx, id, dsOwnerID); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := s.DeleteDataStore(ctx, id); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := s.GetDataStore(ctx, id); !errors.Is(err, ErrDataStoreNotFound) {
			t.Fatalf("get after delete: err = %v, want ErrDataStoreNotFound", err)
		}
	})

	t.Run("DSS-03_DeleteStoreHoldingNumberInUse", func(t *testing.T) {
		const id = "910000000000071003"
		if _, err := s.CreateNewDataStore(ctx, id, dsOwnerID); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO datastore_numbers (store_id, user_id, value) VALUES ($1, $2, 1)", id, dsOwnerID); err != nil {
			t.Fatalf("insert number: %v", err)
		}
		if err := s.DeleteDataStore(ctx, id); !errors.Is(err, ErrDataStoreInUse) {
			t.Fatalf("err = %v, want ErrDataStoreInUse", err)
		}
	})

	t.Run("DSS-04_UpdateUnknownOwnerNotFound", func(t *testing.T) {
		const id = "910000000000071004"
		if _, err := s.CreateNewDataStore(ctx, id, dsOwnerID); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := s.UpdateDataStore(ctx, id, dsMissingID); !errors.Is(err, ErrDataStoreOwnerNotFound) {
			t.Fatalf("err = %v, want ErrDataStoreOwnerNotFound", err)
		}
	})

	t.Run("DSS-05_CreateUnknownOwnerNotFound", func(t *testing.T) {
		if _, err := s.CreateNewDataStore(ctx, "910000000000071005", dsMissingID); !errors.Is(err, ErrDataStoreOwnerNotFound) {
			t.Fatalf("err = %v, want ErrDataStoreOwnerNotFound", err)
		}
	})

	t.Run("DSS-06_NonNumericIDInvalid", func(t *testing.T) {
		if _, err := s.GetDataStore(ctx, "abc"); !errors.Is(err, ErrDataStoreInvalidID) {
			t.Errorf("get: err = %v, want ErrDataStoreInvalidID", err)
		}
		if _, err := s.UpdateDataStore(ctx, "abc", dsOwnerID); !errors.Is(err, ErrDataStoreInvalidID) {
			t.Errorf("update: err = %v, want ErrDataStoreInvalidID", err)
		}
		if err := s.DeleteDataStore(ctx, "abc"); !errors.Is(err, ErrDataStoreInvalidID) {
			t.Errorf("delete: err = %v, want ErrDataStoreInvalidID", err)
		}
		if _, err := s.CreateNewDataStore(ctx, "abc", dsOwnerID); !errors.Is(err, ErrDataStoreInvalidID) {
			t.Errorf("create: err = %v, want ErrDataStoreInvalidID", err)
		}
	})
}
