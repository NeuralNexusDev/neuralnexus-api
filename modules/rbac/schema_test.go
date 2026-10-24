package rbac

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func TestEM01EmptyTables(t *testing.T) {
	pgURL := testutil.PostgresURL(t, "neuralnexus")
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec(ctx, "DROP SCHEMA IF EXISTS rbempty CASCADE")
		admin.Close(ctx)
	})
	schema, err := os.ReadFile(filepath.Join("..", "..", "db", "neuralnexus.sql"))
	if err != nil {
		t.Fatalf("failed to read the neuralnexus schema: %v", err)
	}
	for _, q := range []string{
		"DROP SCHEMA IF EXISTS rbempty CASCADE",
		"CREATE SCHEMA rbempty",
		"SET search_path TO rbempty",
		string(schema),
		"TRUNCATE rbempty.role_permissions, rbempty.roles, rbempty.permissions",
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatalf("failed to prepare the empty schema: %v", err)
		}
	}
	cfg, err := pgxpool.ParseConfig(pgURL)
	if err != nil {
		t.Fatalf("failed to parse the url: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "rbempty"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create the pool: %v", err)
	}
	t.Cleanup(pool.Close)
	st := NewStore(pool)

	t.Run("EM-01_EmptyListsAreNonNil", func(t *testing.T) {
		roles, err := st.ListRoles(context.Background())
		if err != nil || roles == nil || len(roles) != 0 {
			t.Errorf("ListRoles() = (%#v, %v), want (empty non-nil, nil)", roles, err)
		}
		permissions, err := st.ListPermissions(context.Background())
		if err != nil || permissions == nil || len(permissions) != 0 {
			t.Errorf("ListPermissions() = (%#v, %v), want (empty non-nil, nil)", permissions, err)
		}
	})
}
