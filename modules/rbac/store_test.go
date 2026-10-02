package rbac

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

func rbUnusedPort(t *testing.T) int {
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

const (
	rbAccountIDBase = 920000000000000000
	rbMissingID     = "900000000000000009"
)

func rbLive(t *testing.T) (Service, *pgxpool.Pool) {
	t.Helper()

	pgURL := os.Getenv("TEST_POSTGRES_URL")
	if pgURL == "" {
		t.Skip("TEST_POSTGRES_URL must be set to run these store tests")
	}
	db, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		db.Exec(ctx, "DELETE FROM accounts WHERE user_id >= $1 AND user_id < $2", int64(rbAccountIDBase), int64(rbAccountIDBase)+1000)
		db.Exec(ctx, "DELETE FROM roles WHERE name LIKE 'rbtest_%'")
		db.Exec(ctx, "DELETE FROM permissions WHERE scope_name = 'rbtest'")
		db.Close()
	})
	return NewService(NewStore(db)), db
}

func rbRole(t *testing.T, svc Service, name string) *Role {
	t.Helper()
	role, err := svc.CreateRole("rbtest_"+name, "role "+name)
	if err != nil {
		t.Fatalf("failed to create role %s: %v", name, err)
	}
	return role
}

func rbPermission(t *testing.T, svc Service, value string) *Permission {
	t.Helper()
	permission, err := svc.CreatePermission("rbtest", value)
	if err != nil {
		t.Fatalf("failed to create permission %s: %v", value, err)
	}
	return permission
}

func rbAssign(t *testing.T, db *pgxpool.Pool, accountOffset int, roleIDs ...string) {
	t.Helper()
	ids := make([]int64, 0, len(roleIDs))
	for _, id := range roleIDs {
		n, _ := strconv.ParseInt(id, 10, 64)
		ids = append(ids, n)
	}
	if _, err := db.Exec(context.Background(), "INSERT INTO accounts (user_id, role_ids) VALUES ($1, $2)", int64(rbAccountIDBase)+int64(accountOffset), ids); err != nil {
		t.Fatalf("failed to create the account holding the roles: %v", err)
	}
}

func rbScopes(role *Role) []string {
	out := make([]string, 0, len(role.Permissions))
	for _, p := range role.Permissions {
		out = append(out, p.ScopeName+":"+p.ScopeValue)
	}
	return out
}

func rbAssertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestST01to04Roles(t *testing.T) {
	svc, _ := rbLive(t)

	t.Run("ST-01_CreatedRoleIsReadBackByIDAndByName", func(t *testing.T) {
		created := rbRole(t, svc, "a")

		byID, err := svc.GetRole(created.ID)
		if err != nil {
			t.Fatalf("GetRole() err = %v", err)
		}
		byName, err := svc.GetRoleByName("rbtest_a")
		if err != nil {
			t.Fatalf("GetRoleByName() err = %v", err)
		}
		for _, got := range []*Role{byID, byName} {
			if got.ID != created.ID || got.Name != "rbtest_a" || got.Description != "role a" || got.Permissions == nil || len(got.Permissions) != 0 {
				t.Errorf("role = %+v, want ID %s, name rbtest_a, description \"role a\" and an empty, non-nil permission list", got, created.ID)
			}
		}
	})

	t.Run("ST-02_DuplicateNameIsRefused", func(t *testing.T) {
		rbRole(t, svc, "dup")

		_, err := svc.CreateRole("rbtest_dup", "again")

		if !errors.Is(err, ErrRoleNameTaken) {
			t.Errorf("CreateRole() err = %v, want %v", err, ErrRoleNameTaken)
		}
	})

	t.Run("ST-03_UnknownRoleIsNotFound", func(t *testing.T) {
		if _, err := svc.GetRole(rbMissingID); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("GetRole() err = %v, want %v", err, ErrRoleNotFound)
		}
		if _, err := svc.GetRoleByName("rbtest_missing"); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("GetRoleByName() err = %v, want %v", err, ErrRoleNotFound)
		}
	})

	t.Run("ST-04_ListIncludesEveryRoleWithItsPermissions", func(t *testing.T) {
		first := rbRole(t, svc, "list_a")
		second := rbRole(t, svc, "list_b")
		permission := rbPermission(t, svc, "list")
		if err := svc.AttachPermission(second.ID, permission.ID); err != nil {
			t.Fatalf("AttachPermission() err = %v", err)
		}

		roles, err := svc.ListRoles()

		if err != nil {
			t.Fatalf("ListRoles() err = %v", err)
		}
		found := map[string]*Role{}
		for _, r := range roles {
			found[r.ID] = r
		}
		if found[first.ID] == nil || len(found[first.ID].Permissions) != 0 {
			t.Errorf("first role = %+v, want it listed without permissions", found[first.ID])
		}
		if found[second.ID] == nil || len(found[second.ID].Permissions) != 1 || found[second.ID].Permissions[0].ID != permission.ID {
			t.Errorf("second role = %+v, want it listed with its one permission", found[second.ID])
		}
	})
}

func TestST05to08UpdateAndDeleteRole(t *testing.T) {
	svc, db := rbLive(t)

	t.Run("ST-05_UpdateChangesNameAndDescription", func(t *testing.T) {
		role := rbRole(t, svc, "upd")
		name, description := "rbtest_upd2", "changed"

		got, err := svc.UpdateRole(role.ID, &name, &description)

		if err != nil {
			t.Fatalf("UpdateRole() err = %v", err)
		}
		stored, _ := svc.GetRole(role.ID)
		if got.Name != name || stored.Name != name || stored.Description != description {
			t.Errorf("returned %+v, stored %+v, want name %s and description %s", got, stored, name, description)
		}
	})

	t.Run("ST-06_UpdateToATakenNameIsRefused", func(t *testing.T) {
		rbRole(t, svc, "taken")
		role := rbRole(t, svc, "taker")
		name := "rbtest_taken"

		_, err := svc.UpdateRole(role.ID, &name, nil)

		if !errors.Is(err, ErrRoleNameTaken) {
			t.Errorf("UpdateRole() err = %v, want %v", err, ErrRoleNameTaken)
		}
	})

	t.Run("ST-07_DeleteRemovesTheRoleAndItsGrantsOnly", func(t *testing.T) {
		role := rbRole(t, svc, "del")
		permission := rbPermission(t, svc, "del")
		if err := svc.AttachPermission(role.ID, permission.ID); err != nil {
			t.Fatalf("AttachPermission() err = %v", err)
		}

		if err := svc.DeleteRole(role.ID); err != nil {
			t.Fatalf("DeleteRole() err = %v", err)
		}

		if _, err := svc.GetRole(role.ID); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("GetRole() err = %v, want %v", err, ErrRoleNotFound)
		}
		if _, err := svc.GetPermission(permission.ID); err != nil {
			t.Errorf("GetPermission() err = %v, want the permission to remain", err)
		}
	})

	t.Run("ST-08_DeleteOfAHeldRoleIsRefused", func(t *testing.T) {
		role := rbRole(t, svc, "held")
		rbAssign(t, db, 1, role.ID)

		err := svc.DeleteRole(role.ID)

		if !errors.Is(err, ErrRoleInUse) {
			t.Errorf("DeleteRole() err = %v, want %v", err, ErrRoleInUse)
		}
		if _, err := svc.GetRole(role.ID); err != nil {
			t.Errorf("GetRole() err = %v, want the role to remain", err)
		}
	})

	t.Run("ST-09_DeleteOfAnUnknownRoleIsNotFound", func(t *testing.T) {
		if err := svc.DeleteRole(rbMissingID); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("DeleteRole() err = %v, want %v", err, ErrRoleNotFound)
		}
	})

	t.Run("ST-10_UpdateOfAnUnknownRoleIsNotFound", func(t *testing.T) {
		name := "rbtest_x"
		if _, err := svc.UpdateRole(rbMissingID, &name, nil); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("UpdateRole() err = %v, want %v", err, ErrRoleNotFound)
		}
	})
}

func TestST11to14Permissions(t *testing.T) {
	svc, _ := rbLive(t)

	t.Run("ST-11_CreatedPermissionIsReadBackByIDAndScope", func(t *testing.T) {
		created := rbPermission(t, svc, "a")

		byID, err := svc.GetPermission(created.ID)
		if err != nil {
			t.Fatalf("GetPermission() err = %v", err)
		}
		byScope, err := svc.GetPermissionByScope("rbtest", "a")
		if err != nil {
			t.Fatalf("GetPermissionByScope() err = %v", err)
		}
		for _, got := range []*Permission{byID, byScope} {
			if *got != *created {
				t.Errorf("permission = %+v, want %+v", got, created)
			}
		}
	})

	t.Run("ST-12_DuplicateScopeIsRefused", func(t *testing.T) {
		rbPermission(t, svc, "dup")

		_, err := svc.CreatePermission("rbtest", "dup")

		if !errors.Is(err, ErrPermissionExists) {
			t.Errorf("CreatePermission() err = %v, want %v", err, ErrPermissionExists)
		}
	})

	t.Run("ST-13_UnknownPermissionIsNotFound", func(t *testing.T) {
		if _, err := svc.GetPermission(rbMissingID); !errors.Is(err, ErrPermissionNotFound) {
			t.Errorf("GetPermission() err = %v, want %v", err, ErrPermissionNotFound)
		}
		if _, err := svc.GetPermissionByScope("rbtest", "missing"); !errors.Is(err, ErrPermissionNotFound) {
			t.Errorf("GetPermissionByScope() err = %v, want %v", err, ErrPermissionNotFound)
		}
	})

	t.Run("ST-14_ListIncludesTheCreatedPermissions", func(t *testing.T) {
		a := rbPermission(t, svc, "list_a")
		b := rbPermission(t, svc, "list_b")

		permissions, err := svc.ListPermissions()

		if err != nil {
			t.Fatalf("ListPermissions() err = %v", err)
		}
		ids := map[string]bool{}
		for _, p := range permissions {
			ids[p.ID] = true
		}
		if !ids[a.ID] || !ids[b.ID] {
			t.Errorf("ListPermissions() = %v, want both created permissions", permissions)
		}
	})
}

func TestST15to16DeletePermission(t *testing.T) {
	svc, _ := rbLive(t)

	t.Run("ST-15_DeleteRemovesAnUnusedPermission", func(t *testing.T) {
		permission := rbPermission(t, svc, "free")

		if err := svc.DeletePermission(permission.ID); err != nil {
			t.Fatalf("DeletePermission() err = %v", err)
		}

		if _, err := svc.GetPermission(permission.ID); !errors.Is(err, ErrPermissionNotFound) {
			t.Errorf("GetPermission() err = %v, want %v", err, ErrPermissionNotFound)
		}
	})

	t.Run("ST-16_DeleteOfAGrantedPermissionIsRefused", func(t *testing.T) {
		role := rbRole(t, svc, "grants")
		permission := rbPermission(t, svc, "granted")
		if err := svc.AttachPermission(role.ID, permission.ID); err != nil {
			t.Fatalf("AttachPermission() err = %v", err)
		}

		err := svc.DeletePermission(permission.ID)

		if !errors.Is(err, ErrPermissionInUse) {
			t.Errorf("DeletePermission() err = %v, want %v", err, ErrPermissionInUse)
		}
	})

	t.Run("ST-17_DeleteOfAnUnknownPermissionIsNotFound", func(t *testing.T) {
		if err := svc.DeletePermission(rbMissingID); !errors.Is(err, ErrPermissionNotFound) {
			t.Errorf("DeletePermission() err = %v, want %v", err, ErrPermissionNotFound)
		}
	})
}

func TestST18to19AttachAndDetach(t *testing.T) {
	svc, _ := rbLive(t)

	t.Run("ST-18_AttachIsIdempotent", func(t *testing.T) {
		role := rbRole(t, svc, "attach")
		permission := rbPermission(t, svc, "attach")

		for i := 0; i < 2; i++ {
			if err := svc.AttachPermission(role.ID, permission.ID); err != nil {
				t.Fatalf("AttachPermission() call %d err = %v", i+1, err)
			}
		}

		got, _ := svc.GetRole(role.ID)
		rbAssertStrings(t, rbScopes(got), []string{"rbtest:attach"})
	})

	t.Run("ST-19_AttachNeedsBothToExist", func(t *testing.T) {
		role := rbRole(t, svc, "attach_missing")
		permission := rbPermission(t, svc, "attach_missing")

		if err := svc.AttachPermission(rbMissingID, permission.ID); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("AttachPermission() with an unknown role err = %v, want %v", err, ErrRoleNotFound)
		}
		if err := svc.AttachPermission(role.ID, rbMissingID); !errors.Is(err, ErrPermissionNotFound) {
			t.Errorf("AttachPermission() with an unknown permission err = %v, want %v", err, ErrPermissionNotFound)
		}
	})

	t.Run("ST-20_DetachRemovesTheGrantAndIsIdempotent", func(t *testing.T) {
		role := rbRole(t, svc, "detach")
		permission := rbPermission(t, svc, "detach")
		if err := svc.AttachPermission(role.ID, permission.ID); err != nil {
			t.Fatalf("AttachPermission() err = %v", err)
		}

		for i := 0; i < 2; i++ {
			if err := svc.DetachPermission(role.ID, permission.ID); err != nil {
				t.Fatalf("DetachPermission() call %d err = %v", i+1, err)
			}
		}

		got, _ := svc.GetRole(role.ID)
		if len(got.Permissions) != 0 {
			t.Errorf("permissions = %v, want none", got.Permissions)
		}
		if _, err := svc.GetPermission(permission.ID); err != nil {
			t.Errorf("GetPermission() err = %v, want the permission to remain", err)
		}
	})

	t.Run("ST-21_DetachNeedsBothToExist", func(t *testing.T) {
		role := rbRole(t, svc, "detach_missing")
		permission := rbPermission(t, svc, "detach_missing")

		if err := svc.DetachPermission(rbMissingID, permission.ID); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("DetachPermission() with an unknown role err = %v, want %v", err, ErrRoleNotFound)
		}
		if err := svc.DetachPermission(role.ID, rbMissingID); !errors.Is(err, ErrPermissionNotFound) {
			t.Errorf("DetachPermission() with an unknown permission err = %v, want %v", err, ErrPermissionNotFound)
		}
	})
}

func TestST22to26GetPermissionsForRoles(t *testing.T) {
	svc, db := rbLive(t)

	t.Run("ST-22_ReturnsTheDistinctPermissionsOfAllTheRoles", func(t *testing.T) {
		a := rbRole(t, svc, "perms_a")
		b := rbRole(t, svc, "perms_b")
		empty := rbRole(t, svc, "perms_empty")
		shared := rbPermission(t, svc, "shared")
		only := rbPermission(t, svc, "only_b")
		for _, pair := range [][2]string{{a.ID, shared.ID}, {b.ID, shared.ID}, {b.ID, only.ID}} {
			if err := svc.AttachPermission(pair[0], pair[1]); err != nil {
				t.Fatalf("AttachPermission() err = %v", err)
			}
		}

		got, err := svc.GetPermissionsForRoles([]string{a.ID, b.ID, empty.ID})

		if err != nil {
			t.Fatalf("GetPermissionsForRoles() err = %v", err)
		}
		rbAssertStrings(t, got, []string{"rbtest:only_b", "rbtest:shared"})
	})

	t.Run("ST-23_UnknownAndNoRolesGrantNothing", func(t *testing.T) {
		for _, ids := range [][]string{nil, {rbMissingID}} {
			got, err := svc.GetPermissionsForRoles(ids)
			if err != nil || got == nil || len(got) != 0 {
				t.Errorf("GetPermissionsForRoles(%v) = (%#v, %v), want (empty non-nil, nil)", ids, got, err)
			}
		}
	})

	t.Run("ST-25_ColonInAScopeNameIsRefusedByTheDatabase", func(t *testing.T) {
		_, err := db.Exec(context.Background(), "INSERT INTO permissions (id, scope_name, scope_value) VALUES ($1, 'rbtest:x', 'v')", int64(rbAccountIDBase))

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "permissions_scope_name_no_colon" {
			t.Errorf("insert err = %v, want a violation of permissions_scope_name_no_colon", err)
		}
	})

	t.Run("ST-26_ValueWithAColonIsJoinedAndMatchesTheSessionScope", func(t *testing.T) {
		role := rbRole(t, svc, "colon_value")
		permission, err := svc.CreatePermission("rbtest", "a:b")
		if err != nil {
			t.Fatalf("CreatePermission() err = %v", err)
		}
		if err := svc.AttachPermission(role.ID, permission.ID); err != nil {
			t.Fatalf("AttachPermission() err = %v", err)
		}

		got, err := svc.GetPermissionsForRoles([]string{role.ID})

		if err != nil {
			t.Fatalf("GetPermissionsForRoles() err = %v", err)
		}
		rbAssertStrings(t, got, []string{"rbtest:a:b"})
		s := &auth.Session{Permissions: got}
		if !s.HasPermission(perms.Scope{Name: "rbtest", Value: "a:b"}) {
			t.Errorf("a session holding %v does not match the scope rbtest / a:b", got)
		}
	})
}

func TestST24ConnectionErrors(t *testing.T) {
	t.Run("ST-24_UnreachableDatabasePassesTheErrorThroughEveryStoreMethod", func(t *testing.T) {
		cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://user:pass@127.0.0.1:%d/db?sslmode=disable&connect_timeout=2", rbUnusedPort(t)))
		if err != nil {
			t.Fatalf("failed to parse pool config: %v", err)
		}
		pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			t.Fatalf("failed to create pool: %v", err)
		}
		t.Cleanup(pool.Close)
		st := NewStore(pool)

		var errs []error
		collect := func(_ any, err error) { errs = append(errs, err) }
		errs = append(errs,
			st.CreateRole(1, "a", ""), st.UpdateRole(1, "a", ""), st.DeleteRole(1),
			st.CreatePermission(1, "a", "b"), st.DeletePermission(1),
			st.AttachPermission(1, 1), st.DetachPermission(1, 1))
		collect(st.GetRole(1))
		collect(st.GetRoleByName("a"))
		collect(st.ListRoles())
		collect(st.GetPermission(1))
		collect(st.GetPermissionByScope("a", "b"))
		collect(st.ListPermissions())
		collect(st.GetPermissionsForRoles([]string{"1"}))

		if len(errs) != 14 {
			t.Fatalf("checked %d store methods, want all 14", len(errs))
		}
		sentinels := []error{ErrRoleNotFound, ErrPermissionNotFound, ErrRoleNameTaken, ErrPermissionExists, ErrRoleInUse, ErrPermissionInUse}
		for i, err := range errs {
			if err == nil {
				t.Errorf("method %d returned no error", i)
			}
			for _, sentinel := range sentinels {
				if errors.Is(err, sentinel) {
					t.Errorf("method %d err = %v, want the raw connection error", i, err)
				}
			}
		}
	})
}

func TestST27to33RoleIntegrity(t *testing.T) {
	svc, db := rbLive(t)
	st := NewStore(db)
	ctx := context.Background()

	t.Run("ST-27_DeleteWaitsForAnInFlightAssignmentAndThenRefuses", func(t *testing.T) {
		role := rbRole(t, svc, "race")
		id, _ := strconv.ParseInt(role.ID, 10, 64)
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin: %v", err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, "SELECT id FROM roles WHERE id = $1 FOR SHARE", id); err != nil {
			t.Fatalf("failed to lock the role: %v", err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO accounts (user_id, role_ids) VALUES ($1, $2)", int64(rbAccountIDBase)+50, []int64{id}); err != nil {
			t.Fatalf("failed to assign the role: %v", err)
		}

		done := make(chan error, 1)
		go func() { done <- svc.DeleteRole(role.ID) }()
		select {
		case err := <-done:
			t.Fatalf("DeleteRole() returned %v before the assignment committed", err)
		case <-time.After(300 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		if err := <-done; !errors.Is(err, ErrRoleInUse) {
			t.Errorf("DeleteRole() err = %v, want %v", err, ErrRoleInUse)
		}
		if _, err := svc.GetRole(role.ID); err != nil {
			t.Errorf("the assigned role is gone: %v", err)
		}
	})

	t.Run("ST-28_BuiltinRolesAreSeededWithTheirPermissions", func(t *testing.T) {
		all := []string{"beenamegenerator:*", "datastore:*", "numberstore:*", "petpictures:*", "ratelimit:1000", "roles:*", "users:*"}
		for name, want := range map[string][]string{"system": all, "owner": all, "bee_admin": {"beenamegenerator:*"}} {
			role, err := svc.GetRoleByName(name)
			if err != nil {
				t.Fatalf("GetRoleByName(%s) err = %v", name, err)
			}
			got := rbScopes(role)
			sort.Strings(got)
			rbAssertStrings(t, got, want)
		}
	})

	t.Run("ST-29_NameLookupIgnoresIDs", func(t *testing.T) {
		role := rbRole(t, svc, "byid")

		if _, err := st.GetRoleByName(role.ID); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("GetRoleByName(id) err = %v, want %v", err, ErrRoleNotFound)
		}
	})

	t.Run("ST-30_UpdateOfAnUnknownRoleIsNotFoundAtTheStore", func(t *testing.T) {
		if err := st.UpdateRole(900000000000000009, "rbtest_ghost", ""); !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("UpdateRole() err = %v, want %v", err, ErrRoleNotFound)
		}
	})

	t.Run("ST-31_RolesAndPermissionsListInIDOrder", func(t *testing.T) {
		role := rbRole(t, svc, "order")
		first := rbPermission(t, svc, "order_1")
		second := rbPermission(t, svc, "order_2")
		third := rbPermission(t, svc, "order_3")
		for _, p := range []*Permission{third, first, second} {
			if err := svc.AttachPermission(role.ID, p.ID); err != nil {
				t.Fatalf("AttachPermission() err = %v", err)
			}
		}

		cfg, err := pgxpool.ParseConfig(os.Getenv("TEST_POSTGRES_URL"))
		if err != nil {
			t.Fatalf("failed to parse the url: %v", err)
		}
		for _, setting := range []string{"enable_nestloop", "enable_mergejoin", "enable_indexscan", "enable_indexonlyscan", "enable_bitmapscan"} {
			cfg.ConnConfig.RuntimeParams[setting] = "off"
		}
		hashed, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create the pool: %v", err)
		}
		t.Cleanup(hashed.Close)

		got, err := NewStore(hashed).GetRole(mustParse(t, role.ID))

		if err != nil {
			t.Fatalf("GetRole() err = %v", err)
		}
		byID := []*Permission{first, second, third}
		sort.Slice(byID, func(i, j int) bool {
			x, _ := strconv.ParseInt(byID[i].ID, 10, 64)
			y, _ := strconv.ParseInt(byID[j].ID, 10, 64)
			return x < y
		})
		want := make([]string, len(byID))
		for i, p := range byID {
			want[i] = p.ScopeName + ":" + p.ScopeValue
		}
		rbAssertStrings(t, rbScopes(got), want)
		roles, err := svc.ListRoles()
		if err != nil {
			t.Fatalf("ListRoles() err = %v", err)
		}
		for i := 1; i < len(roles); i++ {
			a, _ := strconv.ParseInt(roles[i-1].ID, 10, 64)
			b, _ := strconv.ParseInt(roles[i].ID, 10, 64)
			if a >= b {
				t.Fatalf("ListRoles() is not in ID order at %d: %s then %s", i, roles[i-1].ID, roles[i].ID)
			}
		}
	})
}

func mustParse(t *testing.T, id string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		t.Fatalf("failed to parse id %q: %v", id, err)
	}
	return n
}
