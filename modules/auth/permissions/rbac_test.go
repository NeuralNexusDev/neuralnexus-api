package perms

import (
	"errors"
	"reflect"
	"testing"
)

func TestScopeConstructors(t *testing.T) {
	tests := []struct {
		id       string
		name     string
		fn       func(string) Scope
		value    string
		wantName string
		wantDesc string
	}{
		{"RB-01_HappyPath", "ScopePetPictures", ScopePetPictures, "*", "petpictures", "Pet pictures"},
		{"RB-02_EmptyValue", "ScopePetPictures", ScopePetPictures, "", "petpictures", "Pet pictures"},
		{"RB-03_HappyPath", "ScopeDataStore", ScopeDataStore, "read", "datastore", "Data store"},
		{"RB-04_EmptyValue", "ScopeDataStore", ScopeDataStore, "", "datastore", "Data store"},
		{"RB-05_HappyPath", "ScopeNumberStore", ScopeNumberStore, "write", "numberstore", "Number store"},
		{"RB-06_EmptyValue", "ScopeNumberStore", ScopeNumberStore, "", "numberstore", "Number store"},
		{"RB-07_HappyPath", "ScopeUsers", ScopeUsers, "read", "users", "Users"},
		{"RB-08_EmptyValue", "ScopeUsers", ScopeUsers, "", "users", "Users"},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			got := tc.fn(tc.value)
			want := Scope{Name: tc.wantName, Description: tc.wantDesc, Value: tc.value}
			if got != want {
				t.Errorf("%s(%q) = %+v, want %+v", tc.name, tc.value, got, want)
			}
		})
	}
}

func TestGetRoleByName(t *testing.T) {
	t.Run("RB-09_SystemRole", func(t *testing.T) {
		got, err := GetRoleByName("system")
		if err != nil {
			t.Fatalf("GetRoleByName(\"system\") returned unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, RoleSystem) {
			t.Errorf("GetRoleByName(\"system\") = %+v, want %+v", got, RoleSystem)
		}
	})

	t.Run("RB-10_OwnerRole", func(t *testing.T) {
		got, err := GetRoleByName("owner")
		if err != nil {
			t.Fatalf("GetRoleByName(\"owner\") returned unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, RoleOwner) {
			t.Errorf("GetRoleByName(\"owner\") = %+v, want %+v", got, RoleOwner)
		}
	})

	t.Run("RB-11_UnknownRole", func(t *testing.T) {
		got, err := GetRoleByName("unknown-role")
		if err == nil {
			t.Fatalf("GetRoleByName(\"unknown-role\") returned nil error, want an error")
		}
		if !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("GetRoleByName(\"unknown-role\") error = %v, want %v", err, ErrRoleNotFound)
		}
		if !reflect.DeepEqual(got, Role{}) {
			t.Errorf("GetRoleByName(\"unknown-role\") role = %+v, want zero value", got)
		}
	})

	t.Run("RB-12_EmptyName", func(t *testing.T) {
		got, err := GetRoleByName("")
		if err == nil {
			t.Fatalf("GetRoleByName(\"\") returned nil error, want an error")
		}
		if !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("GetRoleByName(\"\") error = %v, want %v", err, ErrRoleNotFound)
		}
		if !reflect.DeepEqual(got, Role{}) {
			t.Errorf("GetRoleByName(\"\") role = %+v, want zero value", got)
		}
	})

	t.Run("RB-13_CaseMismatch", func(t *testing.T) {
		got, err := GetRoleByName("System")
		if err == nil {
			t.Fatalf("GetRoleByName(\"System\") returned nil error, want an error")
		}
		if !errors.Is(err, ErrRoleNotFound) {
			t.Errorf("GetRoleByName(\"System\") error = %v, want %v", err, ErrRoleNotFound)
		}
		if !reflect.DeepEqual(got, Role{}) {
			t.Errorf("GetRoleByName(\"System\") role = %+v, want zero value", got)
		}
	})
}
