package account

// RoleStore - The source of the permissions that roles grant
type RoleStore interface {
	GetPermissionsForRoles(roleIDs []string) ([]string, error)
}
