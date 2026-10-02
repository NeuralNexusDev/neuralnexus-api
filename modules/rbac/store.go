package rbac

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrRoleNotFound is returned when no role matches.
	ErrRoleNotFound = errors.New("role not found")
	// ErrPermissionNotFound is returned when no permission matches.
	ErrPermissionNotFound = errors.New("permission not found")
	// ErrRoleNameTaken is returned when a role name already belongs to another role.
	ErrRoleNameTaken = errors.New("role name already exists")
	// ErrPermissionExists is returned when a permission with the same node already exists.
	ErrPermissionExists = errors.New("permission already exists")
	// ErrRoleInUse is returned when deleting a role an account holds.
	ErrRoleInUse = errors.New("role is assigned to an account")
	// ErrPermissionInUse is returned when deleting a permission a role grants.
	ErrPermissionInUse = errors.New("permission is granted by a role")
)

// Store is the database access for roles and permissions
type Store interface {
	CreateRole(id int64, name, description string) error
	GetRole(id int64) (*Role, error)
	GetRoleByName(name string) (*Role, error)
	ListRoles() ([]*Role, error)
	UpdateRole(id int64, name, description string) error
	DeleteRole(id int64) error
	CreatePermission(id int64, node, description, valueType, merge string) error
	GetPermission(id int64) (*Permission, error)
	GetPermissionByNode(node string) (*Permission, error)
	ListPermissions() ([]*Permission, error)
	DeletePermission(id int64) error
	AttachPermission(roleID, permissionID int64, value []byte) error
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

const roleSelect = "SELECT r.id::text, r.name, r.description, p.id::text, p.node, p.description, p.value_type, p.merge, rp.value FROM roles r LEFT JOIN role_permissions rp ON rp.role_id = r.id LEFT JOIN permissions p ON p.id = rp.permission_id"

func (s *store) queryRoles(where string, args ...any) ([]*Role, error) {
	rows, err := s.db.Query(context.Background(), roleSelect+where+" ORDER BY r.id, p.id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*Role
	for rows.Next() {
		var id, name, description string
		var permissionID, node, permissionDescription, valueType, merge *string
		var value []byte
		if err := rows.Scan(&id, &name, &description, &permissionID, &node, &permissionDescription, &valueType, &merge, &value); err != nil {
			return nil, err
		}
		if len(roles) == 0 || roles[len(roles)-1].ID != id {
			roles = append(roles, &Role{ID: id, Name: name, Description: description, Permissions: []RolePermission{}})
		}
		if permissionID != nil {
			granted := RolePermission{Permission: Permission{ID: *permissionID, Node: *node, Description: *permissionDescription, ValueType: deref(valueType), Merge: deref(merge)}}
			if value != nil {
				dec := json.NewDecoder(bytes.NewReader(value))
				dec.UseNumber()
				if err := dec.Decode(&granted.Value); err != nil {
					return nil, err
				}
			}
			role := roles[len(roles)-1]
			role.Permissions = append(role.Permissions, granted)
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
	ctx := context.Background()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var locked int64
	if err := tx.QueryRow(ctx, "SELECT id FROM roles WHERE id = $1 FOR UPDATE", id).Scan(&locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return err
	}
	var held bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM accounts WHERE $1::bigint = ANY(role_ids))", id).Scan(&held); err != nil {
		return err
	}
	if held {
		return ErrRoleInUse
	}
	if _, err := tx.Exec(ctx, "DELETE FROM roles WHERE id = $1", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *store) CreatePermission(id int64, node, description, valueType, merge string) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO permissions (id, node, description, value_type, merge) VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))", id, node, description, valueType, merge)
	return translateConstraintErr(err)
}

func (s *store) queryPermissions(where string, args ...any) ([]*Permission, error) {
	rows, err := s.db.Query(context.Background(), "SELECT id::text, node, description, COALESCE(value_type, ''), COALESCE(merge, '') FROM permissions"+where+" ORDER BY id", args...)
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

func (s *store) GetPermissionByNode(node string) (*Permission, error) {
	permissions, err := s.queryPermissions(" WHERE node = $1", node)
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

func (s *store) AttachPermission(roleID, permissionID int64, value []byte) error {
	_, err := s.db.Exec(context.Background(), "INSERT INTO role_permissions (role_id, permission_id, value) VALUES ($1, $2, $3::jsonb) ON CONFLICT (role_id, permission_id) DO UPDATE SET value = EXCLUDED.value", roleID, permissionID, value)
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
	rows, err := s.db.Query(context.Background(), "SELECT p.node, p.value_type, p.merge, rp.value FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id WHERE rp.role_id = ANY($1::text[]::bigint[]) ORDER BY p.node, rp.role_id", roleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []grant
	for rows.Next() {
		var g grant
		var valueType, merge *string
		if err := rows.Scan(&g.node, &valueType, &merge, &g.value); err != nil {
			return nil, err
		}
		g.valueType, g.merge = deref(valueType), deref(merge)
		grants = append(grants, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return flattenGrants(grants)
}

func translateConstraintErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "23505" && pgErr.ConstraintName == "roles_name_key":
		return ErrRoleNameTaken
	case pgErr.Code == "23505" && pgErr.ConstraintName == "permissions_node_key":
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
	return n, err == nil && n >= 1 && strconv.FormatInt(n, 10) == id
}
