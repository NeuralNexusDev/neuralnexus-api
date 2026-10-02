package perms

import "testing"

func TestScopeNodes(t *testing.T) {
	tests := []struct {
		id       string
		scope    Scope
		wantNode string
		wantDesc string
	}{
		{"RB-01_AdminBeeNameGenerator", ScopeAdminBeeNameGenerator, "beenamegenerator.admin", "Bee name generator"},
		{"RB-02_AdminPetPictures", ScopeAdminPetPictures, "petpictures.admin", "Pet pictures"},
		{"RB-03_AdminDataStore", ScopeAdminDataStore, "datastore.admin", "Data store"},
		{"RB-04_AdminNumberStore", ScopeAdminNumberStore, "numberstore.admin", "Number store"},
		{"RB-05_AdminUsers", ScopeAdminUsers, "users.admin", "Users"},
		{"RB-06_AdminRoles", ScopeAdminRoles, "roles.admin", "Roles and permissions"},
		{"RB-07_PetPictures", ScopePetPictures, "petpictures.pets", "Pet pictures"},
		{"RB-08_RateLimit", ScopeRateLimit, "ratelimit", "Rate limit"},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			want := Scope{Node: tc.wantNode, Description: tc.wantDesc}
			if tc.scope != want {
				t.Errorf("scope = %+v, want %+v", tc.scope, want)
			}
		})
	}
}
