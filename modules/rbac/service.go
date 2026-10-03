package rbac

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
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
	maxNodeLength       = 128
	maxScopeValueLength = 128
	maxListValues       = 64
)

var (
	// ErrInvalidID is returned when an ID is not a canonical positive integer.
	ErrInvalidID = errors.New("invalid id")
	// ErrInvalidRoleName is returned when a role name breaks the naming rules.
	ErrInvalidRoleName = errors.New("invalid role name")
	// ErrInvalidDescription is returned when a role description is too long or not valid text.
	ErrInvalidDescription = errors.New("invalid description")
	// ErrBuiltinRole is returned when deleting or renaming a built-in role, or removing roles.admin from system or owner.
	ErrBuiltinRole = errors.New("built-in role is protected")
	// ErrInvalidNode is returned when a permission node breaks the node rules.
	ErrInvalidNode = errors.New("invalid node")
	// ErrInvalidValueType is returned when a permission's value type and merge rule do not form a valid pair.
	ErrInvalidValueType = errors.New("invalid value type")
	// ErrInvalidValue is returned when a granted value does not match its permission's type, or a permission without a type is given one.
	ErrInvalidValue = errors.New("invalid value")
)

const (
	// RoleSystem is the built-in role the service protects from deletion and renaming.
	RoleSystem = "system"
	// RoleOwner is the built-in role the service protects from deletion and renaming.
	RoleOwner = "owner"
)

var builtinRoles = map[string]bool{RoleSystem: true, RoleOwner: true}

// Service is the role and permission management
type Service interface {
	CreateRole(name, description string) (*Role, error)
	GetRole(id string) (*Role, error)
	GetRoleByName(name string) (*Role, error)
	ListRoles() ([]*Role, error)
	UpdateRole(id string, name, description *string) (*Role, error)
	DeleteRole(id string) error
	CreatePermission(node, description, valueType, merge string) (*Permission, error)
	GetPermission(id string) (*Permission, error)
	GetPermissionByNode(node string) (*Permission, error)
	ListPermissions() ([]*Permission, error)
	DeletePermission(id string) error
	AttachPermission(roleID, permissionID string, value any) error
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

func validID(id string) bool {
	n, err := strconv.ParseInt(id, 10, 64)
	return err == nil && n >= 1 && strconv.FormatInt(n, 10) == id
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

func validNode(node string) bool {
	if len(node) == 0 || len(node) > maxNodeLength {
		return false
	}
	segmentStart := true
	for i := 0; i < len(node); i++ {
		c := node[i]
		switch {
		case c >= 'a' && c <= 'z':
		case (c >= '0' && c <= '9') || c == '_':
			if segmentStart {
				return false
			}
		case c == '.':
			if segmentStart {
				return false
			}
			segmentStart = true
			continue
		default:
			return false
		}
		segmentStart = false
	}
	return !segmentStart
}

func validValueText(s string) bool {
	n := utf8.RuneCountInString(s)
	if n == 0 || n > maxScopeValueLength {
		return false
	}
	if !validText(s) || strings.TrimSpace(s) != s {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) })
}

func (s *service) CreateRole(name, description string) (*Role, error) {
	if !validRoleName(name) {
		return nil, ErrInvalidRoleName
	}
	if !validDescription(description) {
		return nil, ErrInvalidDescription
	}
	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateRole(id, name, description); err != nil {
		return nil, err
	}
	return &Role{ID: id, Name: name, Description: description, Permissions: []RolePermission{}}, nil
}

func (s *service) GetRole(id string) (*Role, error) {
	if !validID(id) {
		return nil, ErrInvalidID
	}
	return s.store.GetRole(id)
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
	if err := s.store.UpdateRole(id, role.Name, role.Description); err != nil {
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
	return s.store.DeleteRole(id)
}

func (s *service) CreatePermission(node, description, valueType, merge string) (*Permission, error) {
	if !validNode(node) {
		return nil, ErrInvalidNode
	}
	if !validDescription(description) {
		return nil, ErrInvalidDescription
	}
	switch valueType {
	case "":
		if merge != "" {
			return nil, ErrInvalidValueType
		}
	case ValueTypeInt:
		if merge != MergeMax && merge != MergeMin {
			return nil, ErrInvalidValueType
		}
	case ValueTypeString:
		if merge == "" {
			merge = MergeFirst
		}
		if merge != MergeFirst {
			return nil, ErrInvalidValueType
		}
	case ValueTypeStringList:
		if merge == "" {
			merge = MergeUnion
		}
		if merge != MergeUnion {
			return nil, ErrInvalidValueType
		}
	default:
		return nil, ErrInvalidValueType
	}
	id, err := database.GenSnowflake()
	if err != nil {
		return nil, err
	}
	if err := s.store.CreatePermission(id, node, description, valueType, merge); err != nil {
		return nil, err
	}
	return &Permission{ID: id, Node: node, Description: description, ValueType: valueType, Merge: merge}, nil
}

func (s *service) GetPermission(id string) (*Permission, error) {
	if !validID(id) {
		return nil, ErrInvalidID
	}
	return s.store.GetPermission(id)
}

func (s *service) GetPermissionByNode(node string) (*Permission, error) {
	if !validNode(node) {
		return nil, ErrPermissionNotFound
	}
	return s.store.GetPermissionByNode(node)
}

func (s *service) ListPermissions() ([]*Permission, error) {
	return s.store.ListPermissions()
}

func (s *service) DeletePermission(id string) error {
	if !validID(id) {
		return ErrInvalidID
	}
	return s.store.DeletePermission(id)
}

func (s *service) AttachPermission(roleID, permissionID string, value any) error {
	if !validID(roleID) || !validID(permissionID) {
		return ErrInvalidID
	}
	permission, err := s.store.GetPermission(permissionID)
	if err != nil {
		return err
	}
	var encoded []byte
	switch permission.ValueType {
	case "":
		if value != nil {
			return ErrInvalidValue
		}
	case ValueTypeInt:
		const maxSafe = 1 << 53
		var n int64
		switch v := value.(type) {
		case int:
			if v < -maxSafe || v > maxSafe {
				return ErrInvalidValue
			}
			n = int64(v)
		case int64:
			if v < -maxSafe || v > maxSafe {
				return ErrInvalidValue
			}
			n = v
		case float64:
			if v != math.Trunc(v) || v < -maxSafe || v > maxSafe {
				return ErrInvalidValue
			}
			n = int64(v)
		case json.Number:
			parsed, err := v.Int64()
			if err != nil || parsed < -maxSafe || parsed > maxSafe {
				return ErrInvalidValue
			}
			n = parsed
		default:
			return ErrInvalidValue
		}
		encoded, err = json.Marshal(n)
		if err != nil {
			return err
		}
	case ValueTypeString:
		text, ok := value.(string)
		if !ok || !validValueText(text) {
			return ErrInvalidValue
		}
		encoded, err = json.Marshal(text)
		if err != nil {
			return err
		}
	case ValueTypeStringList:
		var items []string
		switch list := value.(type) {
		case []string:
			items = list
		case []any:
			for _, item := range list {
				text, ok := item.(string)
				if !ok {
					return ErrInvalidValue
				}
				items = append(items, text)
			}
		default:
			return ErrInvalidValue
		}
		if len(items) == 0 || len(items) > maxListValues {
			return ErrInvalidValue
		}
		for _, item := range items {
			if !validValueText(item) {
				return ErrInvalidValue
			}
		}
		encoded, err = json.Marshal(slices.Compact(slices.Sorted(slices.Values(items))))
		if err != nil {
			return err
		}
	default:
		return ErrInvalidValue
	}
	return s.store.AttachPermission(roleID, permissionID, encoded)
}

func (s *service) DetachPermission(roleID, permissionID string) error {
	if !validID(roleID) || !validID(permissionID) {
		return ErrInvalidID
	}
	role, err := s.store.GetRole(roleID)
	if err != nil {
		return err
	}
	if role.Name == RoleSystem || role.Name == RoleOwner {
		permission, err := s.store.GetPermission(permissionID)
		if err != nil {
			return err
		}
		if permission.Node == perms.ScopeAdminRoles.Node {
			return ErrBuiltinRole
		}
	}
	return s.store.DetachPermission(roleID, permissionID)
}

func (s *service) GetPermissionsForRoles(roleIDs []string) ([]string, error) {
	return s.store.GetPermissionsForRoles(roleIDs)
}
