package perms

import (
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
		{"RB-09_HappyPath", "ScopeRoles", ScopeRoles, "read", "roles", "Roles"},
		{"RB-10_EmptyValue", "ScopeRoles", ScopeRoles, "", "roles", "Roles"},
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

func TestScopeAdminRoles(t *testing.T) {
	t.Run("RB-11_AdminRolesIsTheWildcardRolesScope", func(t *testing.T) {
		want := Scope{Name: "roles", Description: "Roles", Value: "*"}
		if ScopeAdminRoles != want {
			t.Errorf("ScopeAdminRoles = %+v, want %+v", ScopeAdminRoles, want)
		}
	})
}
