package testutil

import "context"

// FakeRoleStore is a role store that maps role IDs to permissions and records each lookup.
type FakeRoleStore struct {
	Permissions map[string][]string
	Err         error
	Calls       [][]string
}

// GetPermissionsForRoles returns the permissions of roleIDs, or Err.
func (f *FakeRoleStore) GetPermissionsForRoles(_ context.Context, roleIDs []string) ([]string, error) {
	f.Calls = append(f.Calls, roleIDs)
	if f.Err != nil {
		return nil, f.Err
	}
	var out []string
	for _, id := range roleIDs {
		out = append(out, f.Permissions[id]...)
	}
	return out, nil
}

// DefaultRoleStore returns a FakeRoleStore with permissions for roles "1" and "2".
func DefaultRoleStore() *FakeRoleStore {
	return &FakeRoleStore{Permissions: map[string][]string{
		"1": {"beenamegenerator.admin", "petpictures.admin", "ratelimit:1000"},
		"2": {"users.admin", "datastore.admin"},
	}}
}
