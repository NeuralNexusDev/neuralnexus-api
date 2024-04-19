package perms

import "errors"

// -------------- Structs --------------

type Scope struct {
	Name        string
	Description string
	Value       string
}

var (
	ScopeAdminBeeNameGenerator = Scope{
		Name:        "beenamegenerator",
		Description: "Bee name generator",
		Value:       "*",
	}

	ScopeAdminPetPictures = ScopePetPictures("*")

	ScopeAdminRateLimit = Scope{
		Name:        "ratelimit",
		Description: "Rate limit",
		Value:       "1000",
	}
)

// ScopePetPictures -- Pet pictures
func ScopePetPictures(value string) Scope {
	return Scope{
		Name:        "petpictures",
		Description: "Pet pictures",
		Value:       value,
	}
}

type Role struct {
	Name        string
	Description string
	Permissions []Scope
}

var (
	RoleSystem = Role{
		Name:        "system",
		Description: "System",
		Permissions: []Scope{
			ScopeAdminBeeNameGenerator,
			ScopeAdminPetPictures,
			ScopeAdminRateLimit,
		},
	}

	RoleOwner = Role{
		Name:        "owner",
		Description: "Owner",
		Permissions: []Scope{
			ScopeAdminBeeNameGenerator,
			ScopeAdminPetPictures,
			ScopeAdminRateLimit,
		},
	}
)

// -------------- Functions --------------

// GetRoleByName gets a role by name
func GetRoleByName(name string) (Role, error) {
	switch name {
	case RoleSystem.Name:
		return RoleSystem, nil
	case RoleOwner.Name:
		return RoleOwner, nil
	default:
		return Role{}, errors.New("Role not found")
	}
}
