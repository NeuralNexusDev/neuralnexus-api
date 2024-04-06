package auth

import "errors"

// -------------- Structs --------------

type Scope struct {
	Name        string `json:"name" xml:"name"`
	Description string `json:"description,omitempty" xml:"description,omitempty"`
	Value       string `json:"value,omitempty" xml:"value,omitempty"`
}

var (
	ScopeAdminBeeNameGenerator = Scope{
		Name:        "beenamegenerator",
		Description: "Bee name generator",
		Value:       "*",
	}

	ScopeAdminPetPictures = Scope{
		Name:        "petpictures",
		Description: "Pet pictures",
		Value:       "*",
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
	Name        string  `json:"name" xml:"name"`
	Description string  `json:"description,omitempty" xml:"description,omitempty"`
	Permissions []Scope `json:"permissions" xml:"permissions"`
}

var (
	RoleSystem = Role{
		Name:        "RoleSystem",
		Description: "System role",
		Permissions: []Scope{
			ScopeAdminBeeNameGenerator,
			ScopeAdminPetPictures,
		},
	}

	RoleOwner = Role{
		Name:        "RoleOwner",
		Description: "Owner role",
		Permissions: []Scope{
			ScopeAdminBeeNameGenerator,
			ScopeAdminPetPictures,
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
