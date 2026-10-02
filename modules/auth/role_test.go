package auth

type rsFakeRoleStore struct {
	permissions map[string][]string
	err         error
	calls       [][]string
}

func (f *rsFakeRoleStore) GetPermissionsForRoles(roleIDs []string) ([]string, error) {
	f.calls = append(f.calls, roleIDs)
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, id := range roleIDs {
		out = append(out, f.permissions[id]...)
	}
	return out, nil
}

func rsDefaultRoleStore() *rsFakeRoleStore {
	return &rsFakeRoleStore{permissions: map[string][]string{
		"1": {"beenamegenerator|*", "petpictures|*", "ratelimit|1000"},
		"2": {"users|*", "datastore|*"},
	}}
}
