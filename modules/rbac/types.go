package rbac

// Value types a permission can carry
const (
	ValueTypeInt        = "int"
	ValueTypeString     = "string"
	ValueTypeStringList = "string_list"
)

// How a permission's values combine when an account holds it through several roles
const (
	MergeMax   = "max"
	MergeMin   = "min"
	MergeFirst = "first"
	MergeUnion = "union"
)

// Permission is a node roles can grant, with a value type when roles grant it with a value
type Permission struct {
	ID          string `json:"id" xml:"id"`
	Node        string `json:"node" xml:"node"`
	Description string `json:"description" xml:"description"`
	ValueType   string `json:"value_type,omitempty" xml:"value_type,omitempty"`
	Merge       string `json:"merge,omitempty" xml:"merge,omitempty"`
}

// RolePermission is a permission as a role grants it, with the granted value if the permission takes one
type RolePermission struct {
	Permission
	Value any `json:"value,omitempty" xml:"value,omitempty"`
}

// Role is a named set of permissions
type Role struct {
	ID          string           `json:"id" xml:"id"`
	Name        string           `json:"name" xml:"name"`
	Description string           `json:"description" xml:"description"`
	Permissions []RolePermission `json:"permissions" xml:"permissions"`
}
