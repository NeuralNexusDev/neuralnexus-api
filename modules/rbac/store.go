package rbac

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CREATE TABLE roles (
// 	id BIGINT PRIMARY KEY,
// 	name TEXT NOT NULL UNIQUE,
// 	description TEXT NOT NULL DEFAULT '',
// 	CONSTRAINT roles_name_not_empty CHECK (name <> '')
// );
//
// CREATE TABLE permissions (
// 	id BIGINT PRIMARY KEY,
// 	scope_name TEXT NOT NULL,
// 	scope_value TEXT NOT NULL,
// 	CONSTRAINT permissions_scope_unique UNIQUE (scope_name, scope_value),
// 	CONSTRAINT permissions_scope_name_not_empty CHECK (scope_name <> '')
// );
//
// CREATE TABLE role_permissions (
// 	role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
// 	permission_id BIGINT NOT NULL REFERENCES permissions(id),
// 	PRIMARY KEY (role_id, permission_id)
// );

var (
	ErrRoleNotFound       = errors.New("role not found")
	ErrPermissionNotFound = errors.New("permission not found")
	ErrRoleNameTaken      = errors.New("role name already exists")
	ErrPermissionExists   = errors.New("permission already exists")
	ErrRoleInUse          = errors.New("role is assigned to an account")
	ErrPermissionInUse    = errors.New("permission is granted by a role")
)

// Store is the database access for roles and permissions
type Store interface {
	CreateRole(id int64, name, description string) error
	GetRole(id int64) (*Role, error)
	GetRoleByName(name string) (*Role, error)
	ListRoles() ([]*Role, error)
	UpdateRole(id int64, name, description string) error
	DeleteRole(id int64) error
	CreatePermission(id int64, scopeName, scopeValue string) error
	GetPermission(id int64) (*Permission, error)
	GetPermissionByScope(scopeName, scopeValue string) (*Permission, error)
	ListPermissions() ([]*Permission, error)
	DeletePermission(id int64) error
	AttachPermission(roleID, permissionID int64) error
	DetachPermission(roleID, permissionID int64) error
	GetPermissionsForRoles(roleIDs []string) ([]string, error)
}

type store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store on the pool that holds the accounts table
func NewStore(db *pgxpool.Pool) Store {
	return &store{db: db}
}

const roleSelect = "SELECT r.id::text, r.name, r.description, p.id::text, p.scope_name, p.scope_value FROM roles r LEFT JOIN role_permissions rp ON rp.role_id = r.id LEFT JOIN permissions p ON p.id = rp.permission_id"

func (s *store) queryRoles(where string, args ...any) ([]*Role, error) {
	rows, err := s.db.Query(context.Background(), roleSelect+where+" ORDER BY r.id, p.id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*Role
	for rows.Next() {
		var id, name, description string
		var permissionID, scopeName, scopeValue *string
		if err := rows.Scan(&id, &name, &description, &permissionID, &scopeName, &scopeValue); err != nil {
			return nil, err
		}
		if len(roles) == 0 || roles[len(roles)-1].ID != id {
			roles = append(roles, &Role{ID: id, Name: name, Description: description, Permissions: []Permission{}})
		}
		if permissionID != nil {
			role := roles[len(roles)-1]
			role.Permissions = append(role.Permissions, Permission{ID: *permissionID, ScopeName: *scopeName, ScopeValue: *scopeValue})
		}
	}
	return roles, rows.Err()
}

func (s *store) CreateRole(id int64, name, description string) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO roles (id, name, description) VALUES ($1, $2, $3)", id, name, description)
	return translateConstraintErr(err)
}

func (s *store) GetRole(id int64) (*Role, error) {
	roles, err := s.queryRoles(" WHERE r.id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrRoleNotFound
	}
	return roles[0], nil
}

func (s *store) GetRoleByName(name string) (*Role, error) {
	roles, err := s.queryRoles(" WHERE r.name = $1", name)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, ErrRoleNotFound
	}
	return roles[0], nil
}

func (s *store) ListRoles() ([]*Role, error) {
	roles, err := s.queryRoles("")
	if roles == nil {
		roles = []*Role{}
	}
	return roles, err
}

func (s *store) UpdateRole(id int64, name, description string) error {
	tag, err := s.db.Exec(context.Background(), "UPDATE roles SET name = $2, description = $3 WHERE id = $1", id, name, description)
	if err != nil {
		return translateConstraintErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func (s *store) DeleteRole(id int64) error {
	tag, err := s.db.Exec(context.Background(), "DELETE FROM roles WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM accounts WHERE $1::text = ANY(role_ids::text[]))", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var exists bool
	if err := s.db.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM roles WHERE id = $1)", id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrRoleNotFound
	}
	return ErrRoleInUse
}

func (s *store) CreatePermission(id int64, scopeName, scopeValue string) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO permissions (id, scope_name, scope_value) VALUES ($1, $2, $3)", id, scopeName, scopeValue)
	return translateConstraintErr(err)
}

func (s *store) queryPermissions(where string, args ...any) ([]*Permission, error) {
	rows, err := s.db.Query(context.Background(), "SELECT id::text, scope_name, scope_value FROM permissions"+where+" ORDER BY id", args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByPos[Permission])
}

func (s *store) GetPermission(id int64) (*Permission, error) {
	permissions, err := s.queryPermissions(" WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	if len(permissions) == 0 {
		return nil, ErrPermissionNotFound
	}
	return permissions[0], nil
}

func (s *store) GetPermissionByScope(scopeName, scopeValue string) (*Permission, error) {
	permissions, err := s.queryPermissions(" WHERE scope_name = $1 AND scope_value = $2", scopeName, scopeValue)
	if err != nil {
		return nil, err
	}
	if len(permissions) == 0 {
		return nil, ErrPermissionNotFound
	}
	return permissions[0], nil
}

func (s *store) ListPermissions() ([]*Permission, error) {
	permissions, err := s.queryPermissions("")
	if permissions == nil {
		permissions = []*Permission{}
	}
	return permissions, err
}

func (s *store) DeletePermission(id int64) error {
	tag, err := s.db.Exec(context.Background(), "DELETE FROM permissions WHERE id = $1", id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrPermissionInUse
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPermissionNotFound
	}
	return nil
}

func (s *store) AttachPermission(roleID, permissionID int64) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", roleID, permissionID)
	return translateConstraintErr(err)
}

func (s *store) DetachPermission(roleID, permissionID int64) error {
	tag, err := s.db.Exec(context.Background(), "DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2", roleID, permissionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var roleExists, permissionExists bool
	if err := s.db.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM roles WHERE id = $1), EXISTS (SELECT 1 FROM permissions WHERE id = $2)", roleID, permissionID).Scan(&roleExists, &permissionExists); err != nil {
		return err
	}
	if !roleExists {
		return ErrRoleNotFound
	}
	if !permissionExists {
		return ErrPermissionNotFound
	}
	return nil
}

func (s *store) GetPermissionsForRoles(roleIDs []string) ([]string, error) {
	rows, err := s.db.Query(context.Background(), "SELECT DISTINCT p.scope_name, p.scope_value FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id WHERE rp.role_id = ANY($1::text[]::bigint[]) ORDER BY p.scope_name, p.scope_value", roleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		permissions = append(permissions, name+"|"+value)
	}
	return permissions, rows.Err()
}

func translateConstraintErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "23505" && pgErr.ConstraintName == "roles_name_key":
		return ErrRoleNameTaken
	case pgErr.Code == "23505" && pgErr.ConstraintName == "permissions_scope_unique":
		return ErrPermissionExists
	case pgErr.Code == "23503" && pgErr.ConstraintName == "role_permissions_role_id_fkey":
		return ErrRoleNotFound
	case pgErr.Code == "23503" && pgErr.ConstraintName == "role_permissions_permission_id_fkey":
		return ErrPermissionNotFound
	}
	return err
}

func parseID(id string) (int64, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	return n, err == nil && n >= 1
}
