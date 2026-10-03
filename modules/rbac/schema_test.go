package rbac

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEM01EmptyTables(t *testing.T) {
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run the empty table test")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec(ctx, "DROP SCHEMA IF EXISTS rbempty CASCADE")
		admin.Close(ctx)
	})
	for _, q := range []string{
		"DROP SCHEMA IF EXISTS rbempty CASCADE",
		"CREATE SCHEMA rbempty",
		"CREATE TABLE rbempty.roles (LIKE public.roles INCLUDING ALL)",
		"CREATE TABLE rbempty.permissions (LIKE public.permissions INCLUDING ALL)",
		"CREATE TABLE rbempty.role_permissions (LIKE public.role_permissions INCLUDING ALL)",
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
		roles, err := st.ListRoles()
		if err != nil || roles == nil || len(roles) != 0 {
			t.Errorf("ListRoles() = (%#v, %v), want (empty non-nil, nil)", roles, err)
		}
		permissions, err := st.ListPermissions()
		if err != nil || permissions == nil || len(permissions) != 0 {
			t.Errorf("ListPermissions() = (%#v, %v), want (empty non-nil, nil)", permissions, err)
		}
	})
}
