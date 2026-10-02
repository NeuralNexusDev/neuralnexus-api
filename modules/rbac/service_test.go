package rbac

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

var errSvcStore = errors.New("store failure")

// fakeStore records the calls reaching it and returns the configured values.
type fakeStore struct {
	calls      []string
	err        error
	role       *Role
	permission *Permission
	createdID  int64
	attached   [2]int64
}

func (f *fakeStore) CreateRole(id int64, name, description string) error {
	f.calls = append(f.calls, "CreateRole")
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
	return []string{"a|b"}, f.err
}

func TestSV01to04RoleValidation(t *testing.T) {
	valid := []string{"a", "admin", "r2_d2", "a09", strings.Repeat("a", maxRoleNameLength)}
	invalid := []string{"", "Admin", "1abc", "_a", "a-b", "a b", "a|b", strings.Repeat("a", maxRoleNameLength+1)}

	t.Run("SV-01_ValidNamesAreCreated", func(t *testing.T) {
		for _, name := range valid {
			f := &fakeStore{}
			role, err := NewService(f).CreateRole(name, "")
			if err != nil || role.Name != name {
				t.Fatalf("name %q: got %v, %v", name, role, err)
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
			{"", "v"}, {"n", ""}, {"a|b", "v"}, {"n", "a|b"},
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
		if err != nil || p.ID != strconv.FormatInt(f.createdID, 10) {
			t.Fatalf("got %v, %v", p, err)
		}
	})
}

func TestSV07to09IDs(t *testing.T) {
	bad := []string{"", "abc", "0", "-5", "1.5", "99999999999999999999"}

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
		f := &fakeStore{}
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
