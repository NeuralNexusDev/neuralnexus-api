package rbac

import (
	"errors"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

const (
	maxRoleNameLength   = 63
	maxDescriptionLen   = 256
	maxScopeNameLength  = 64
	maxScopeValueLength = 128
)

var (
	ErrInvalidID          = errors.New("invalid id")
	ErrInvalidRoleName    = errors.New("invalid role name")
	ErrInvalidDescription = errors.New("invalid description")
	ErrInvalidScope       = errors.New("invalid scope")
)

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

func validScope(scopeName, scopeValue string) bool {
	return len(scopeName) > 0 && len(scopeName) <= maxScopeNameLength &&
		len(scopeValue) > 0 && len(scopeValue) <= maxScopeValueLength &&
		!strings.Contains(scopeName, ":")
}

func (s *service) CreateRole(name, description string) (*Role, error) {
	if !validRoleName(name) {
		return nil, ErrInvalidRoleName
	}
	if len(description) > maxDescriptionLen {
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
		role.Name = *name
	}
	if description != nil {
		if len(*description) > maxDescriptionLen {
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
	n, ok := parseID(id)
	if !ok {
		return ErrInvalidID
	}
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
	return s.store.DetachPermission(r, p)
}

func (s *service) GetPermissionsForRoles(roleIDs []string) ([]string, error) {
	return s.store.GetPermissionsForRoles(roleIDs)
}
