package rbac

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

var errSvcStore = errors.New("store failure")

type fakeStore struct {
	calls      []string
	err        error
	role       *Role
	permission *Permission
	createdID  int64
	attached   [2]int64
	created    []string
}

func (f *fakeStore) CreateRole(id int64, name, description string) error {
	f.calls = append(f.calls, "CreateRole")
	f.created = []string{name, description}
	f.createdID = id
	return f.err
}
func (f *fakeStore) GetRole(id int64) (*Role, error) {
	f.calls = append(f.calls, "GetRole")
	return f.role, f.err
}
func (f *fakeStore) GetRoleByName(name string) (*Role, error) {
	f.calls = append(f.calls, "GetRoleByName")
	return f.role, f.err
}
func (f *fakeStore) ListRoles() ([]*Role, error) {
	f.calls = append(f.calls, "ListRoles")
	return []*Role{f.role}, f.err
}
func (f *fakeStore) UpdateRole(id int64, name, description string) error {
	f.calls = append(f.calls, "UpdateRole:"+name+":"+description)
	return f.err
}
func (f *fakeStore) DeleteRole(id int64) error {
	f.calls = append(f.calls, "DeleteRole")
	return f.err
}
func (f *fakeStore) CreatePermission(id int64, scopeName, scopeValue string) error {
	f.calls = append(f.calls, "CreatePermission")
	f.created = []string{scopeName, scopeValue}
	f.createdID = id
	return f.err
}
func (f *fakeStore) GetPermission(id int64) (*Permission, error) {
	f.calls = append(f.calls, "GetPermission")
	return f.permission, f.err
}
func (f *fakeStore) GetPermissionByScope(scopeName, scopeValue string) (*Permission, error) {
	f.calls = append(f.calls, "GetPermissionByScope")
	return f.permission, f.err
}
func (f *fakeStore) ListPermissions() ([]*Permission, error) {
	f.calls = append(f.calls, "ListPermissions")
	return []*Permission{f.permission}, f.err
}
func (f *fakeStore) DeletePermission(id int64) error {
	f.calls = append(f.calls, "DeletePermission")
	return f.err
}
func (f *fakeStore) AttachPermission(roleID, permissionID int64) error {
	f.calls = append(f.calls, "AttachPermission")
	f.attached = [2]int64{roleID, permissionID}
	return f.err
}
func (f *fakeStore) DetachPermission(roleID, permissionID int64) error {
	f.calls = append(f.calls, "DetachPermission")
	f.attached = [2]int64{roleID, permissionID}
	return f.err
}
func (f *fakeStore) GetPermissionsForRoles(roleIDs []string) ([]string, error) {
	f.calls = append(f.calls, "GetPermissionsForRoles")
	return []string{"a:b"}, f.err
}

func TestSV01to04RoleValidation(t *testing.T) {
	valid := []string{"a", "admin", "r2_d2", "a09", strings.Repeat("a", maxRoleNameLength)}
	invalid := []string{"", "Admin", "1abc", "_a", "a-b", "a b", "a:b", "aB", "a\x00", strings.Repeat("a", maxRoleNameLength+1)}

	t.Run("SV-01_ValidNamesAreCreated", func(t *testing.T) {
		for _, name := range valid {
			f := &fakeStore{}
			role, err := NewService(f).CreateRole(name, "")
			if err != nil || role.Name != name || f.created[0] != name {
				t.Fatalf("name %q: got %v, %v, store got %v", name, role, err, f.created)
			}
		}
	})
	t.Run("SV-02_InvalidNamesAreRejectedBeforeTheStore", func(t *testing.T) {
		for _, name := range invalid {
			f := &fakeStore{}
			if _, err := NewService(f).CreateRole(name, ""); !errors.Is(err, ErrInvalidRoleName) {
				t.Fatalf("name %q: got %v, want ErrInvalidRoleName", name, err)
			}
			if len(f.calls) != 0 {
				t.Fatalf("name %q reached the store: %v", name, f.calls)
			}
		}
	})
	t.Run("SV-03_DescriptionLengthBoundary", func(t *testing.T) {
		f := &fakeStore{}
		if _, err := NewService(f).CreateRole("a", strings.Repeat("d", maxDescriptionLen)); err != nil {
			t.Fatalf("a description at the limit was rejected: %v", err)
		}
		f = &fakeStore{}
		if _, err := NewService(f).CreateRole("a", strings.Repeat("d", maxDescriptionLen+1)); !errors.Is(err, ErrInvalidDescription) {
			t.Fatalf("got %v, want ErrInvalidDescription", err)
		}
		if len(f.calls) != 0 {
			t.Fatalf("an invalid description reached the store: %v", f.calls)
		}
	})
	t.Run("SV-04_CreatedRoleCarriesTheGeneratedSnowflake", func(t *testing.T) {
		f := &fakeStore{}
		role, err := NewService(f).CreateRole("a", "d")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if role.ID != strconv.FormatInt(f.createdID, 10) || f.createdID < 1 {
			t.Fatalf("role id %q does not match the id %d given to the store", role.ID, f.createdID)
		}
		if role.Permissions == nil || len(role.Permissions) != 0 {
			t.Fatalf("a new role must list an empty, non-nil permission set, got %#v", role.Permissions)
		}
	})
}

func TestSV05to06ScopeValidation(t *testing.T) {
	t.Run("SV-05_InvalidScopesAreRejectedBeforeTheStore", func(t *testing.T) {
		cases := [][2]string{
			{"", "v"}, {"n", ""}, {"a:b", "v"},
			{strings.Repeat("n", maxScopeNameLength+1), "v"},
			{"n", strings.Repeat("v", maxScopeValueLength+1)},
		}
		for _, c := range cases {
			f := &fakeStore{}
			if _, err := NewService(f).CreatePermission(c[0], c[1]); !errors.Is(err, ErrInvalidScope) {
				t.Fatalf("scope %q/%q: got %v, want ErrInvalidScope", c[0], c[1], err)
			}
			if len(f.calls) != 0 {
				t.Fatalf("scope %q/%q reached the store", c[0], c[1])
			}
		}
	})
	t.Run("SV-06_ScopesAtTheLimitAreCreated", func(t *testing.T) {
		f := &fakeStore{}
		p, err := NewService(f).CreatePermission(strings.Repeat("n", maxScopeNameLength), strings.Repeat("v", maxScopeValueLength))
		if err != nil || p.ID != strconv.FormatInt(f.createdID, 10) || f.created[0] != strings.Repeat("n", maxScopeNameLength) || f.created[1] != strings.Repeat("v", maxScopeValueLength) {
			t.Fatalf("got %v, %v, store got %v lengths", p, err, len(f.created))
		}
	})
	t.Run("SV-12_ValuesMayContainAColon", func(t *testing.T) {
		f := &fakeStore{}
		p, err := NewService(f).CreatePermission("n", "a:b:c")
		if err != nil || p.ScopeValue != "a:b:c" || f.created[1] != "a:b:c" {
			t.Fatalf("got %v, %v, store got %v", p, err, f.created)
		}
	})
	t.Run("SV-13_TextRulesApplyToScopesAndDescriptions", func(t *testing.T) {
		badScopes := [][2]string{
			{"a\x00b", "v"}, {"n", "a\x00b"}, {"\xff", "v"}, {"n", "\xff"}, {"n\t", "v"}, {"n", "a\nb"},
			{" n", "v"}, {"n ", "v"}, {"n", " v"}, {"n", "v "},
			{strings.Repeat("é", maxScopeNameLength+1), "v"}, {"n", strings.Repeat("é", maxScopeValueLength+1)},
		}
		for _, c := range badScopes {
			f := &fakeStore{}
			if _, err := NewService(f).CreatePermission(c[0], c[1]); !errors.Is(err, ErrInvalidScope) || len(f.calls) != 0 {
				t.Fatalf("scope %q/%q: got %v with calls %v, want ErrInvalidScope before the store", c[0], c[1], err, f.calls)
			}
		}
		f := &fakeStore{}
		if _, err := NewService(f).CreatePermission(strings.Repeat("é", maxScopeNameLength), strings.Repeat("é", maxScopeValueLength)); err != nil {
			t.Fatalf("scope parts of exactly the limit in characters were refused: %v", err)
		}
		for _, d := range []string{"a\x00b", "\xff", strings.Repeat("é", maxDescriptionLen+1)} {
			f := &fakeStore{}
			if _, err := NewService(f).CreateRole("a", d); !errors.Is(err, ErrInvalidDescription) || len(f.calls) != 0 {
				t.Fatalf("description %q: got %v with calls %v, want ErrInvalidDescription before the store", d, err, f.calls)
			}
		}
		if _, err := NewService(&fakeStore{}).CreateRole("a", strings.Repeat("é", maxDescriptionLen)); err != nil {
			t.Fatalf("a description of exactly the limit in characters was refused: %v", err)
		}
	})
	t.Run("SV-14_LookupsOfNamesThatCannotExistNeverReachTheStore", func(t *testing.T) {
		f := &fakeStore{role: &Role{}, permission: &Permission{}}
		s := NewService(f)
		for _, name := range []string{"own\x00er", "Owner", "123", ""} {
			if _, err := s.GetRoleByName(name); !errors.Is(err, ErrRoleNotFound) {
				t.Fatalf("GetRoleByName(%q) err = %v, want ErrRoleNotFound", name, err)
			}
		}
		for _, c := range [][2]string{{"a\x00", "v"}, {"n:x", "v"}, {"n", ""}} {
			if _, err := s.GetPermissionByScope(c[0], c[1]); !errors.Is(err, ErrPermissionNotFound) {
				t.Fatalf("GetPermissionByScope(%q, %q) err = %v, want ErrPermissionNotFound", c[0], c[1], err)
			}
		}
		if len(f.calls) != 0 {
			t.Fatalf("lookups reached the store: %v", f.calls)
		}
		if _, err := s.GetRoleByName("owner"); err != nil {
			t.Fatalf("GetRoleByName() of a valid name err = %v", err)
		}
	})
}

func TestSV07to09IDs(t *testing.T) {
	bad := []string{"", "abc", "0", "-5", "1.5", "99999999999999999999", "007", "+8", " 8"}

	t.Run("SV-07_BadIDsAreRejectedBeforeTheStoreByEveryIDMethod", func(t *testing.T) {
		for _, id := range bad {
			f := &fakeStore{}
			s := NewService(f)
			name := "x"
			errs := []error{
				s.DeleteRole(id),
				s.DeletePermission(id),
				s.AttachPermission(id, "1"),
				s.AttachPermission("1", id),
				s.DetachPermission(id, "1"),
				s.DetachPermission("1", id),
			}
			_, e := s.GetRole(id)
			errs = append(errs, e)
			_, e = s.GetPermission(id)
			errs = append(errs, e)
			_, e = s.UpdateRole(id, &name, nil)
			errs = append(errs, e)
			for i, err := range errs {
				if !errors.Is(err, ErrInvalidID) {
					t.Fatalf("id %q call %d: got %v, want ErrInvalidID", id, i, err)
				}
			}
			if len(f.calls) != 0 {
				t.Fatalf("id %q reached the store: %v", id, f.calls)
			}
		}
	})
	t.Run("SV-08_AttachAndDetachPassTheParsedIDsInOrder", func(t *testing.T) {
		f := &fakeStore{role: &Role{Name: "mod"}, permission: &Permission{}}
		s := NewService(f)
		if err := s.AttachPermission("11", "22"); err != nil || f.attached != [2]int64{11, 22} {
			t.Fatalf("attach: %v %v", err, f.attached)
		}
		f.attached = [2]int64{}
		if err := s.DetachPermission("33", "44"); err != nil || f.attached != [2]int64{33, 44} {
			t.Fatalf("detach: %v %v", err, f.attached)
		}
	})
	t.Run("SV-09_StoreErrorsPassThroughUnchanged", func(t *testing.T) {
		f := &fakeStore{err: errSvcStore, role: &Role{}, permission: &Permission{}}
		s := NewService(f)
		name := "x"
		errs := []error{
			s.DeleteRole("1"), s.DeletePermission("1"), s.AttachPermission("1", "1"), s.DetachPermission("1", "1"),
		}
		var e error
		_, e = s.CreateRole("a", "")
		errs = append(errs, e)
		_, e = s.CreatePermission("n", "v")
		errs = append(errs, e)
		_, e = s.GetRole("1")
		errs = append(errs, e)
		_, e = s.GetRoleByName("a")
		errs = append(errs, e)
		_, e = s.ListRoles()
		errs = append(errs, e)
		_, e = s.UpdateRole("1", &name, nil)
		errs = append(errs, e)
		_, e = s.GetPermission("1")
		errs = append(errs, e)
		_, e = s.GetPermissionByScope("n", "v")
		errs = append(errs, e)
		_, e = s.ListPermissions()
		errs = append(errs, e)
		_, e = s.GetPermissionsForRoles([]string{"1"})
		errs = append(errs, e)
		for i, err := range errs {
			if !errors.Is(err, errSvcStore) {
				t.Fatalf("call %d: got %v, want the store error", i, err)
			}
		}
	})
}

func TestSV10UpdateRoleMerge(t *testing.T) {
	newStore := func() *fakeStore { return &fakeStore{role: &Role{ID: "1", Name: "old", Description: "olddesc"}} }
	str := func(s string) *string { return &s }

	t.Run("SV-10_OmittedFieldsKeepTheirCurrentValue", func(t *testing.T) {
		f := newStore()
		role, err := NewService(f).UpdateRole("1", str("new"), nil)
		if err != nil || role.Name != "new" || role.Description != "olddesc" {
			t.Fatalf("got %v, %v", role, err)
		}
		if f.calls[len(f.calls)-1] != "UpdateRole:new:olddesc" {
			t.Fatalf("store got %v", f.calls)
		}
		f = newStore()
		role, err = NewService(f).UpdateRole("1", nil, str(""))
		if err != nil || role.Name != "old" || role.Description != "" {
			t.Fatalf("an empty description must clear it: %v, %v", role, err)
		}
		if f.calls[len(f.calls)-1] != "UpdateRole:old:" {
			t.Fatalf("store got %v", f.calls)
		}
	})
	t.Run("SV-11_InvalidUpdatesNeverWrite", func(t *testing.T) {
		for _, c := range []struct {
			name, desc *string
			want       error
		}{
			{str("Bad"), nil, ErrInvalidRoleName},
			{nil, str(strings.Repeat("d", maxDescriptionLen+1)), ErrInvalidDescription},
		} {
			f := newStore()
			if _, err := NewService(f).UpdateRole("1", c.name, c.desc); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
			for _, call := range f.calls {
				if strings.HasPrefix(call, "UpdateRole") {
					t.Fatalf("an invalid update was written: %v", f.calls)
				}
			}
		}
	})
}

func TestSV15to18BuiltinRoles(t *testing.T) {
	str := func(s string) *string { return &s }

	t.Run("SV-15_BuiltinRolesCannotBeDeleted", func(t *testing.T) {
		for _, name := range []string{"system", "owner", "bee_admin"} {
			f := &fakeStore{role: &Role{ID: "5", Name: name}}
			if err := NewService(f).DeleteRole("5"); !errors.Is(err, ErrBuiltinRole) {
				t.Fatalf("DeleteRole(%s) err = %v, want ErrBuiltinRole", name, err)
			}
			for _, call := range f.calls {
				if call == "DeleteRole" {
					t.Fatalf("DeleteRole(%s) reached the store", name)
				}
			}
		}
		f := &fakeStore{role: &Role{ID: "5", Name: "mod"}}
		if err := NewService(f).DeleteRole("5"); err != nil || f.calls[len(f.calls)-1] != "DeleteRole" {
			t.Fatalf("DeleteRole(mod) err = %v, calls %v", err, f.calls)
		}
	})
	t.Run("SV-16_BuiltinRolesCannotBeRenamedButCanBeDescribed", func(t *testing.T) {
		f := &fakeStore{role: &Role{ID: "5", Name: "owner", Description: "d"}}
		if _, err := NewService(f).UpdateRole("5", str("boss"), nil); !errors.Is(err, ErrBuiltinRole) {
			t.Fatalf("rename err = %v, want ErrBuiltinRole", err)
		}
		f = &fakeStore{role: &Role{ID: "5", Name: "owner", Description: "d"}}
		if _, err := NewService(f).UpdateRole("5", str("owner"), str("new")); err != nil {
			t.Fatalf("a description change that repeats the name err = %v", err)
		}
		if f.calls[len(f.calls)-1] != "UpdateRole:owner:new" {
			t.Fatalf("store got %v", f.calls)
		}
	})
	t.Run("SV-17_RolesPermissionStaysOnSystemAndOwner", func(t *testing.T) {
		for _, name := range []string{"system", "owner"} {
			f := &fakeStore{role: &Role{ID: "5", Name: name}, permission: &Permission{ID: "6", ScopeName: "roles", ScopeValue: "*"}}
			if err := NewService(f).DetachPermission("5", "6"); !errors.Is(err, ErrBuiltinRole) {
				t.Fatalf("detach from %s err = %v, want ErrBuiltinRole", name, err)
			}
			for _, call := range f.calls {
				if call == "DetachPermission" {
					t.Fatalf("detach from %s reached the store", name)
				}
			}
		}
	})
	t.Run("SV-18_OtherDetachesAreAllowed", func(t *testing.T) {
		for _, c := range []struct{ role, name, value string }{
			{"owner", "users", "*"}, {"owner", "roles", "read"}, {"mod", "roles", "*"}, {"bee_admin", "roles", "*"},
		} {
			f := &fakeStore{role: &Role{Name: c.role}, permission: &Permission{ScopeName: c.name, ScopeValue: c.value}}
			if err := NewService(f).DetachPermission("5", "6"); err != nil || f.calls[len(f.calls)-1] != "DetachPermission" {
				t.Fatalf("detach %s:%s from %s err = %v, calls %v", c.name, c.value, c.role, err, f.calls)
			}
		}
	})
}
