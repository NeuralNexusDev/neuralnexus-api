package rbac

// Permission is a scope name and value that roles can grant
type Permission struct {
	ID         string `json:"id" xml:"id"`
	ScopeName  string `json:"scope_name" xml:"scope_name"`
	ScopeValue string `json:"scope_value" xml:"scope_value"`
}

// Role is a named set of permissions
type Role struct {
	ID          string       `json:"id" xml:"id"`
	Name        string       `json:"name" xml:"name"`
	Description string       `json:"description" xml:"description"`
	Permissions []Permission `json:"permissions" xml:"permissions"`
}
