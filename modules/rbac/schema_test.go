package rbac

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSC01InitSqlCarriesTheRbacSchema(t *testing.T) {
	t.Run("SC-01_TestDatabaseInitContainsTheRbacSchemaVerbatim", func(t *testing.T) {
		schema, err := os.ReadFile("../../docker/rbac.sql")
		if err != nil {
			t.Fatalf("failed to read the rbac schema: %v", err)
		}
		init, err := os.ReadFile("../../docker/testdb/init.sql")
		if err != nil {
			t.Fatalf("failed to read the test database init: %v", err)
		}
		if !strings.Contains(string(init), strings.TrimSpace(string(schema))) {
			t.Fatal("docker/testdb/init.sql has drifted from docker/rbac.sql")
		}
	})
}

func rbScratch(t *testing.T) *pgx.Conn {
	t.Helper()
	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run the migration test")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(ctx, "DROP SCHEMA IF EXISTS rbmigration CASCADE")
		conn.Close(ctx)
	})
	for _, q := range []string{
		"DROP SCHEMA IF EXISTS rbmigration CASCADE",
		"CREATE SCHEMA rbmigration",
		"SET search_path TO rbmigration",
	} {
		if _, err := conn.Exec(ctx, q); err != nil {
			t.Fatalf("failed to prepare the scratch schema: %v", err)
		}
	}
	return conn
}

func TestMG01Migration(t *testing.T) {
	conn := rbScratch(t)
	ctx := context.Background()
	schema, _ := os.ReadFile("../../docker/rbac.sql")
	migration, err := os.ReadFile("../../docker/rbac_migration.sql")
	if err != nil {
		t.Fatalf("failed to read the migration: %v", err)
	}
	for _, q := range []string{
		string(schema),
		`CREATE TABLE accounts (user_id BIGINT PRIMARY KEY, roles TEXT[] NOT NULL DEFAULT '{}')`,
		`CREATE TABLE sessions (session_id BIGINT PRIMARY KEY)`,
		`INSERT INTO accounts (user_id, roles) VALUES (1, '{system}'), (2, '{bee_admin,gone}'), (3, '{}'), (4, '{owner,bee_admin}')`,
		`INSERT INTO sessions (session_id) VALUES (10), (11)`,
	} {
		if _, err := conn.Exec(ctx, q); err != nil {
			t.Fatalf("failed to set up the pre-migration state: %v", err)
		}
	}
	if _, err := conn.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	t.Run("MG-01_AccountRolesBecomeRoleIDsAndUnknownNamesAreDropped", func(t *testing.T) {
		rows, err := conn.Query(ctx, `SELECT a.user_id, COALESCE(array_agg(r.name ORDER BY r.name) FILTER (WHERE r.id IS NOT NULL), '{}')
			FROM accounts a LEFT JOIN roles r ON r.id = ANY(a.role_ids) GROUP BY a.user_id ORDER BY a.user_id`)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		defer rows.Close()
		got := map[int64]string{}
		for rows.Next() {
			var id int64
			var names []string
			if err := rows.Scan(&id, &names); err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			got[id] = strings.Join(names, ",")
		}
		want := map[int64]string{1: "system", 2: "bee_admin", 3: "", 4: "bee_admin,owner"}
		for id, w := range want {
			if got[id] != w {
				t.Fatalf("account %d: got %q, want %q", id, got[id], w)
			}
		}
	})
	t.Run("MG-02_OldRolesColumnIsDroppedAndSessionsAreCleared", func(t *testing.T) {
		var cols int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'rbmigration' AND table_name = 'accounts' AND column_name = 'roles'`).Scan(&cols); err != nil || cols != 0 {
			t.Fatalf("roles column count %d, err %v", cols, err)
		}
		var sessions int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions); err != nil || sessions != 0 {
			t.Fatalf("sessions left %d, err %v", sessions, err)
		}
	})
	t.Run("MG-03_BuiltInRolesGrantTheirPermissions", func(t *testing.T) {
		grants := func(role string) string {
			var out string
			err := conn.QueryRow(ctx, `SELECT COALESCE(string_agg(p.scope_name || ':' || p.scope_value, ',' ORDER BY p.scope_name), '')
				FROM roles r JOIN role_permissions rp ON rp.role_id = r.id JOIN permissions p ON p.id = rp.permission_id WHERE r.name = $1`, role).Scan(&out)
			if err != nil {
				t.Fatalf("query failed: %v", err)
			}
			return out
		}
		all := "beenamegenerator:*,datastore:*,numberstore:*,petpictures:*,ratelimit:1000,roles:*,users:*"
		for role, want := range map[string]string{"system": all, "owner": all, "bee_admin": "beenamegenerator:*"} {
			if got := grants(role); got != want {
				t.Fatalf("%s: got %q, want %q", role, got, want)
			}
		}
	})
}

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
	schema, _ := os.ReadFile("../../docker/rbac.sql")
	for _, q := range []string{"DROP SCHEMA IF EXISTS rbempty CASCADE", "CREATE SCHEMA rbempty", "SET search_path TO rbempty", string(schema), "DELETE FROM role_permissions", "DELETE FROM roles", "DELETE FROM permissions"} {
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
