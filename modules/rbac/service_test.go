package rbac

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var errSvcStore = errors.New("store failure")

type fakeStore struct {
	calls         []string
	err           error
	role          *Role
	permission    *Permission
	createdID     string
	attached      [2]string
	attachedValue []byte
	created       []string
}

func (f *fakeStore) CreateRole(id string, name, description string) error {
	f.calls = append(f.calls, "CreateRole")
	f.created = []string{name, description}
	f.createdID = id
	return f.err
}
func (f *fakeStore) GetRole(id string) (*Role, error) {
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
func (f *fakeStore) UpdateRole(id string, name, description string) error {
	f.calls = append(f.calls, "UpdateRole:"+name+":"+description)
	return f.err
}
func (f *fakeStore) DeleteRole(id string) error {
	f.calls = append(f.calls, "DeleteRole")
	return f.err
}
func (f *fakeStore) CreatePermission(id string, node, description, valueType, merge string) error {
	f.calls = append(f.calls, "CreatePermission")
	f.created = []string{node, description, valueType, merge}
	f.createdID = id
	return f.err
}
func (f *fakeStore) GetPermission(id string) (*Permission, error) {
	f.calls = append(f.calls, "GetPermission")
	return f.permission, f.err
}
func (f *fakeStore) GetPermissionByNode(node string) (*Permission, error) {
	f.calls = append(f.calls, "GetPermissionByNode")
	return f.permission, f.err
}
func (f *fakeStore) ListPermissions() ([]*Permission, error) {
	f.calls = append(f.calls, "ListPermissions")
	return []*Permission{f.permission}, f.err
}
func (f *fakeStore) DeletePermission(id string) error {
	f.calls = append(f.calls, "DeletePermission")
	return f.err
}
func (f *fakeStore) AttachPermission(roleID, permissionID string, value []byte) error {
	f.calls = append(f.calls, "AttachPermission")
	f.attached = [2]string{roleID, permissionID}
	f.attachedValue = value
	return f.err
}
func (f *fakeStore) DetachPermission(roleID, permissionID string) error {
	f.calls = append(f.calls, "DetachPermission")
	f.attached = [2]string{roleID, permissionID}
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
		if role.ID != f.createdID || !validID(f.createdID) {
			t.Fatalf("role id %q does not match the id %q given to the store", role.ID, f.createdID)
		}
		if role.Permissions == nil || len(role.Permissions) != 0 {
			t.Fatalf("a new role must list an empty, non-nil permission set, got %#v", role.Permissions)
		}
	})
}

func TestSV05to06NodeValidation(t *testing.T) {
	t.Run("SV-05_InvalidNodesAreRejectedBeforeTheStore", func(t *testing.T) {
		bad := []string{"", "A", "aB", "1a", "a.1b", "a._b", "_a", "a..b", ".a", "a.", "a-b", "a:b", "a b", "a|b", "a\x00", "é", "a.é", strings.Repeat("a", maxNodeLength+1)}
		for _, node := range bad {
			f := &fakeStore{}
			if _, err := NewService(f).CreatePermission(node, "", "", ""); !errors.Is(err, ErrInvalidNode) {
				t.Fatalf("node %q: got %v, want ErrInvalidNode", node, err)
			}
			if len(f.calls) != 0 {
				t.Fatalf("node %q reached the store", node)
			}
		}
	})
	t.Run("SV-06_ValidNodesAreCreatedUnchanged", func(t *testing.T) {
		for _, node := range []string{"a", "roles.admin", "a_b.c2.d_3", "a0", strings.Repeat("a", maxNodeLength)} {
			f := &fakeStore{}
			p, err := NewService(f).CreatePermission(node, "d", "", "")
			if err != nil || p.Node != node || p.ID != f.createdID || f.created[0] != node || f.created[1] != "d" {
				t.Fatalf("node %q: got %v, %v, store got %v", node, p, err, f.created)
			}
		}
	})
	t.Run("SV-12_ValueTypesAndMergeRulesMustPair", func(t *testing.T) {
		valid := [][3]string{
			{"", "", ""}, {"int", "max", "max"}, {"int", "min", "min"},
			{"string", "", "first"}, {"string", "first", "first"},
			{"string_list", "", "union"}, {"string_list", "union", "union"},
		}
		for _, c := range valid {
			f := &fakeStore{}
			p, err := NewService(f).CreatePermission("a.b", "", c[0], c[1])
			if err != nil || p.ValueType != c[0] || p.Merge != c[2] || f.created[2] != c[0] || f.created[3] != c[2] {
				t.Fatalf("%v: got %v, %v, store got %v, want merge %q", c, p, err, f.created, c[2])
			}
		}
		for _, c := range [][2]string{{"int", ""}, {"int", "first"}, {"int", "union"}, {"string", "max"}, {"string_list", "min"}, {"bool", ""}, {"", "max"}} {
			f := &fakeStore{}
			if _, err := NewService(f).CreatePermission("a.b", "", c[0], c[1]); !errors.Is(err, ErrInvalidValueType) || len(f.calls) != 0 {
				t.Fatalf("%v: got %v with calls %v, want ErrInvalidValueType before the store", c, err, f.calls)
			}
		}
	})
	t.Run("SV-13_TextRulesApplyToDescriptions", func(t *testing.T) {
		for _, d := range []string{"a\x00b", "\xff", "a\ufffdb", strings.Repeat("é", maxDescriptionLen+1)} {
			f := &fakeStore{}
			if _, err := NewService(f).CreateRole("a", d); !errors.Is(err, ErrInvalidDescription) || len(f.calls) != 0 {
				t.Fatalf("description %q: got %v with calls %v, want ErrInvalidDescription before the store", d, err, f.calls)
			}
			f = &fakeStore{}
			if _, err := NewService(f).CreatePermission("a.b", d, "", ""); !errors.Is(err, ErrInvalidDescription) || len(f.calls) != 0 {
				t.Fatalf("permission description %q: got %v with calls %v, want ErrInvalidDescription before the store", d, err, f.calls)
			}
		}
		if _, err := NewService(&fakeStore{}).CreateRole("a", "line one\nline two \U0001F468\u200d\U0001F4BB"); err != nil {
			t.Fatalf("a description with a newline and a joined emoji was refused: %v", err)
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
		for _, node := range []string{"a\x00", "n:x", "N", "a..b", ""} {
			if _, err := s.GetPermissionByNode(node); !errors.Is(err, ErrPermissionNotFound) {
				t.Fatalf("GetPermissionByNode(%q) err = %v, want ErrPermissionNotFound", node, err)
			}
		}
		if len(f.calls) != 0 {
			t.Fatalf("lookups reached the store: %v", f.calls)
		}
		if _, err := s.GetRoleByName("owner"); err != nil {
			t.Fatalf("GetRoleByName() of a valid name err = %v", err)
		}
		if _, err := s.GetPermissionByNode("roles.admin"); err != nil {
			t.Fatalf("GetPermissionByNode() of a valid node err = %v", err)
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
				s.AttachPermission(id, "1", nil),
				s.AttachPermission("1", id, nil),
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
		if err := s.AttachPermission("11", "22", nil); err != nil || f.attached != [2]string{"11", "22"} {
			t.Fatalf("attach: %v %v", err, f.attached)
		}
		f.attached = [2]string{}
		if err := s.DetachPermission("33", "44"); err != nil || f.attached != [2]string{"33", "44"} {
			t.Fatalf("detach: %v %v", err, f.attached)
		}
	})
	t.Run("SV-09_StoreErrorsPassThroughUnchanged", func(t *testing.T) {
		f := &fakeStore{err: errSvcStore, role: &Role{}, permission: &Permission{}}
		s := NewService(f)
		name := "x"
		errs := []error{
			s.DeleteRole("1"), s.DeletePermission("1"), s.AttachPermission("1", "1", nil), s.DetachPermission("1", "1"),
		}
		var e error
		_, e = s.CreateRole("a", "")
		errs = append(errs, e)
		_, e = s.CreatePermission("n", "", "", "")
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
		_, e = s.GetPermissionByNode("n")
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
		for _, name := range []string{"system", "owner"} {
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
		for _, name := range []string{"mod", "bee_admin"} {
			f := &fakeStore{role: &Role{ID: "5", Name: name}}
			if err := NewService(f).DeleteRole("5"); err != nil || f.calls[len(f.calls)-1] != "DeleteRole" {
				t.Fatalf("DeleteRole(%s) err = %v, calls %v", name, err, f.calls)
			}
		}
	})
	t.Run("SV-16_BuiltinRolesCannotBeRenamedButCanBeDescribed", func(t *testing.T) {
		f := &fakeStore{role: &Role{ID: "5", Name: "owner", Description: "d"}}
		if _, err := NewService(f).UpdateRole("5", str("boss"), nil); !errors.Is(err, ErrBuiltinRole) {
			t.Fatalf("rename err = %v, want ErrBuiltinRole", err)
		}
		f = &fakeStore{role: &Role{ID: "5", Name: "bee_admin"}}
		if _, err := NewService(f).UpdateRole("5", str("bee_manager"), nil); err != nil || f.calls[len(f.calls)-1] != "UpdateRole:bee_manager:" {
			t.Fatalf("renaming bee_admin err = %v, calls %v", err, f.calls)
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
			f := &fakeStore{role: &Role{ID: "5", Name: name}, permission: &Permission{ID: "6", Node: "roles.admin"}}
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
		for _, c := range []struct{ role, node string }{
			{"owner", "users.admin"}, {"owner", "roles"}, {"owner", "roles.administrator"}, {"mod", "roles.admin"}, {"bee_admin", "roles.admin"},
		} {
			f := &fakeStore{role: &Role{Name: c.role}, permission: &Permission{Node: c.node}}
			if err := NewService(f).DetachPermission("5", "6"); err != nil || f.calls[len(f.calls)-1] != "DetachPermission" {
				t.Fatalf("detach %s from %s err = %v, calls %v", c.node, c.role, err, f.calls)
			}
		}
	})
}

func TestSV19to23GrantedValues(t *testing.T) {
	attach := func(valueType string, value any) (*fakeStore, error) {
		f := &fakeStore{role: &Role{Name: "mod"}, permission: &Permission{ID: "6", Node: "a.b", ValueType: valueType}}
		return f, NewService(f).AttachPermission("5", "6", value)
	}
	longText := strings.Repeat("v", maxScopeValueLength+1)

	t.Run("SV-19_PermissionsWithoutATypeTakeNoValue", func(t *testing.T) {
		f, err := attach("", nil)
		if err != nil || f.attachedValue != nil || f.calls[len(f.calls)-1] != "AttachPermission" {
			t.Fatalf("no value: err %v, stored %q", err, f.attachedValue)
		}
		for _, v := range []any{"x", 5, []string{"a"}, false} {
			f, err := attach("", v)
			if !errors.Is(err, ErrInvalidValue) || slices.Contains(f.calls, "AttachPermission") {
				t.Fatalf("value %v: err %v, calls %v, want ErrInvalidValue before the store", v, err, f.calls)
			}
		}
	})
	t.Run("SV-20_IntValuesMustBeSafeWholeNumbers", func(t *testing.T) {
		for _, v := range []any{json.Number("1000"), float64(1000), 1000, int64(1000)} {
			f, err := attach("int", v)
			if err != nil || string(f.attachedValue) != "1000" {
				t.Fatalf("value %#v: err %v, stored %q", v, err, f.attachedValue)
			}
		}
		for _, c := range []struct {
			value any
			want  string
		}{
			{json.Number("9007199254740992"), "9007199254740992"}, {json.Number("-9007199254740992"), "-9007199254740992"},
			{int64(1 << 53), "9007199254740992"}, {int64(-(1 << 53)), "-9007199254740992"},
			{1 << 53, "9007199254740992"}, {-(1 << 53), "-9007199254740992"},
			{float64(1 << 53), "9007199254740992"}, {float64(-(1 << 53)), "-9007199254740992"},
		} {
			f, err := attach("int", c.value)
			if err != nil || string(f.attachedValue) != c.want {
				t.Fatalf("boundary value %#v: err %v, stored %q, want %s", c.value, err, f.attachedValue, c.want)
			}
		}
		for _, v := range []any{nil, "5", 1.5, json.Number("1.5"), float64(1e300), int64(1<<53 + 1), true, []any{1},
			json.Number("9007199254740993"), json.Number("-9007199254740993"), int64(-(1<<53 + 1)), 1<<53 + 1, -(1<<53 + 1),
			float64(1<<53 + 2), float64(-(1<<53 + 2))} {
			f, err := attach("int", v)
			if !errors.Is(err, ErrInvalidValue) || slices.Contains(f.calls, "AttachPermission") {
				t.Fatalf("value %#v: err %v, calls %v, want ErrInvalidValue before the store", v, err, f.calls)
			}
		}
	})
	t.Run("SV-21_StringValuesFollowTheTextRules", func(t *testing.T) {
		f, err := attach("string", "abc:d")
		if err != nil || string(f.attachedValue) != `"abc:d"` {
			t.Fatalf("err %v, stored %q", err, f.attachedValue)
		}
		for _, v := range []any{nil, "", " a", "a ", "a\x00", "a\u200bb", "a\nb", "\xff", longText, 5, []string{"a"}} {
			f, err := attach("string", v)
			if !errors.Is(err, ErrInvalidValue) || slices.Contains(f.calls, "AttachPermission") {
				t.Fatalf("value %#v: err %v, calls %v, want ErrInvalidValue before the store", v, err, f.calls)
			}
		}
	})
	t.Run("SV-22_ListValuesAreSortedAndDeduplicated", func(t *testing.T) {
		for _, v := range []any{[]any{"b", "a", "a"}, []string{"b", "a", "a"}} {
			f, err := attach("string_list", v)
			if err != nil || string(f.attachedValue) != `["a","b"]` {
				t.Fatalf("value %#v: err %v, stored %q", v, err, f.attachedValue)
			}
		}
		tooMany := make([]string, maxListValues+1)
		for i := range tooMany {
			tooMany[i] = strconv.Itoa(i)
		}
		for _, v := range []any{nil, "a", []any{}, []string{}, tooMany, []any{"a", 5}, []any{"a", ""}, []string{"a", longText}, []string{"a\x00"}} {
			f, err := attach("string_list", v)
			if !errors.Is(err, ErrInvalidValue) || slices.Contains(f.calls, "AttachPermission") {
				t.Fatalf("value %#v: err %v, calls %v, want ErrInvalidValue before the store", v, err, f.calls)
			}
		}
		atLimit := make([]string, maxListValues)
		for i := range atLimit {
			atLimit[i] = strconv.Itoa(i)
		}
		if _, err := attach("string_list", atLimit); err != nil {
			t.Fatalf("a list of exactly the limit was refused: %v", err)
		}
	})
	t.Run("SV-23_UnknownPermissionsFailBeforeTheValueIsChecked", func(t *testing.T) {
		f := &fakeStore{err: ErrPermissionNotFound}
		if err := NewService(f).AttachPermission("5", "6", "x"); !errors.Is(err, ErrPermissionNotFound) || slices.Contains(f.calls, "AttachPermission") {
			t.Fatalf("err %v, calls %v, want ErrPermissionNotFound without an attach", err, f.calls)
		}
	})
}
