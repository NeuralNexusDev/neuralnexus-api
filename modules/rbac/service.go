package rbac

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

const (
	maxRoleNameLength   = 63
	maxDescriptionLen   = 256
	maxScopeNameLength  = 64
	maxScopeValueLength = 128
)

var (
	// ErrInvalidID is returned when an ID is not a canonical positive integer.
	ErrInvalidID = errors.New("invalid id")
	// ErrInvalidRoleName is returned when a role name breaks the naming rules.
	ErrInvalidRoleName = errors.New("invalid role name")
	// ErrInvalidDescription is returned when a role description is too long or not valid text.
	ErrInvalidDescription = errors.New("invalid description")
	// ErrBuiltinRole is returned when deleting or renaming a built-in role, or removing the roles permission from system or owner.
	ErrBuiltinRole = errors.New("built-in role is protected")
	// ErrInvalidScope is returned when a permission's scope name or value breaks the scope rules.
	ErrInvalidScope = errors.New("invalid scope")
)

var builtinRoles = map[string]bool{"system": true, "owner": true, "bee_admin": true}

// Service is the role and permission management
type Service interface {
	CreateRole(name, description string) (*Role, error)
	GetRole(id string) (*Role, error)
	GetRoleByName(name string) (*Role, error)
	ListRoles() ([]*Role, error)
	UpdateRole(id string, name, description *string) (*Role, error)
	DeleteRole(id string) error
	CreatePermission(scopeName, scopeValue string) (*Permission, error)
	GetPermission(id string) (*Permission, error)
	GetPermissionByScope(scopeName, scopeValue string) (*Permission, error)
	ListPermissions() ([]*Permission, error)
	DeletePermission(id string) error
	AttachPermission(roleID, permissionID string) error
	DetachPermission(roleID, permissionID string) error
	GetPermissionsForRoles(roleIDs []string) ([]string, error)
}

type service struct {
	store Store
}

// NewService creates a Service on a Store
func NewService(store Store) Service {
	return &service{store: store}
}

func newID() (int64, error) {
	id, err := database.GenSnowflake()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(id, 10, 64)
}

func validRoleName(name string) bool {
	if len(name) == 0 || len(name) > maxRoleNameLength || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

func validText(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool { return r == 0 || r == utf8.RuneError })
}

func validDescription(description string) bool {
	return validText(description) && utf8.RuneCountInString(description) <= maxDescriptionLen
}

func validScopePart(s string, maxLength int) bool {
	n := utf8.RuneCountInString(s)
	return n > 0 && n <= maxLength && validText(s) && strings.TrimSpace(s) == s &&
		!strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) })
}

func validScope(scopeName, scopeValue string) bool {
	return validScopePart(scopeName, maxScopeNameLength) && validScopePart(scopeValue, maxScopeValueLength) &&
		!strings.Contains(scopeName, ":")
}

func (s *service) CreateRole(name, description string) (*Role, error) {
	if !validRoleName(name) {
		return nil, ErrInvalidRoleName
	}
	if !validDescription(description) {
		return nil, ErrInvalidDescription
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateRole(id, name, description); err != nil {
		return nil, err
	}
	return &Role{ID: strconv.FormatInt(id, 10), Name: name, Description: description, Permissions: []Permission{}}, nil
}

func (s *service) GetRole(id string) (*Role, error) {
	n, ok := parseID(id)
	if !ok {
		return nil, ErrInvalidID
	}
	return s.store.GetRole(n)
}

func (s *service) GetRoleByName(name string) (*Role, error) {
	if !validRoleName(name) {
		return nil, ErrRoleNotFound
	}
	return s.store.GetRoleByName(name)
}

func (s *service) ListRoles() ([]*Role, error) {
	return s.store.ListRoles()
}

func (s *service) UpdateRole(id string, name, description *string) (*Role, error) {
	role, err := s.GetRole(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		if !validRoleName(*name) {
			return nil, ErrInvalidRoleName
		}
		if *name != role.Name && builtinRoles[role.Name] {
			return nil, ErrBuiltinRole
		}
		role.Name = *name
	}
	if description != nil {
		if !validDescription(*description) {
			return nil, ErrInvalidDescription
		}
		role.Description = *description
	}
	n, _ := parseID(id)
	if err := s.store.UpdateRole(n, role.Name, role.Description); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *service) DeleteRole(id string) error {
	role, err := s.GetRole(id)
	if err != nil {
		return err
	}
	if builtinRoles[role.Name] {
		return ErrBuiltinRole
	}
	n, _ := parseID(id)
	return s.store.DeleteRole(n)
}

func (s *service) CreatePermission(scopeName, scopeValue string) (*Permission, error) {
	if !validScope(scopeName, scopeValue) {
		return nil, ErrInvalidScope
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	if err := s.store.CreatePermission(id, scopeName, scopeValue); err != nil {
		return nil, err
	}
	return &Permission{ID: strconv.FormatInt(id, 10), ScopeName: scopeName, ScopeValue: scopeValue}, nil
}

func (s *service) GetPermission(id string) (*Permission, error) {
	n, ok := parseID(id)
	if !ok {
		return nil, ErrInvalidID
	}
	return s.store.GetPermission(n)
}

func (s *service) GetPermissionByScope(scopeName, scopeValue string) (*Permission, error) {
	if !validScope(scopeName, scopeValue) {
		return nil, ErrPermissionNotFound
	}
	return s.store.GetPermissionByScope(scopeName, scopeValue)
}

func (s *service) ListPermissions() ([]*Permission, error) {
	return s.store.ListPermissions()
}

func (s *service) DeletePermission(id string) error {
	n, ok := parseID(id)
	if !ok {
		return ErrInvalidID
	}
	return s.store.DeletePermission(n)
}

func (s *service) AttachPermission(roleID, permissionID string) error {
	r, rOK := parseID(roleID)
	p, pOK := parseID(permissionID)
	if !rOK || !pOK {
		return ErrInvalidID
	}
	return s.store.AttachPermission(r, p)
}

func (s *service) DetachPermission(roleID, permissionID string) error {
	r, rOK := parseID(roleID)
	p, pOK := parseID(permissionID)
	if !rOK || !pOK {
		return ErrInvalidID
	}
	role, err := s.store.GetRole(r)
	if err != nil {
		return err
	}
	if role.Name == "system" || role.Name == "owner" {
		permission, err := s.store.GetPermission(p)
		if err != nil {
			return err
		}
		if permission.ScopeName == perms.ScopeAdminRoles.Name && permission.ScopeValue == perms.ScopeAdminRoles.Value {
			return ErrBuiltinRole
		}
	}
	return s.store.DetachPermission(r, p)
}

func (s *service) GetPermissionsForRoles(roleIDs []string) ([]string, error) {
	return s.store.GetPermissionsForRoles(roleIDs)
}
