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

	ScopeAdminDataStore   = ScopeDataStore("*")
	ScopeAdminNumberStore = ScopeNumberStore("*")
	ScopeAdminUsers       = ScopeUsers("*")
)

// ScopePetPictures -- Pet pictures
func ScopePetPictures(value string) Scope {
	return Scope{
		Name:        "petpictures",
		Description: "Pet pictures",
		Value:       value,
	}
}

// ScopeDataStore -- Data store
func ScopeDataStore(value string) Scope {
	return Scope{
		Name:        "datastore",
		Description: "Data store",
		Value:       value,
	}
}

// ScopeNumberStore -- Number store
func ScopeNumberStore(value string) Scope {
	return Scope{
		Name:        "numberstore",
		Description: "Number store",
		Value:       value,
	}
}

// ScopeUsers -- Admin users
func ScopeUsers(value string) Scope {
	return Scope{
		Name:        "users",
		Description: "Users",
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
			ScopeAdminDataStore,
			ScopeAdminNumberStore,
			ScopeAdminUsers,
		},
	}

	RoleOwner = Role{
		Name:        "owner",
		Description: "Owner",
		Permissions: []Scope{
			ScopeAdminBeeNameGenerator,
			ScopeAdminPetPictures,
			ScopeAdminRateLimit,
			ScopeAdminDataStore,
			ScopeAdminNumberStore,
			ScopeAdminUsers,
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
		return Role{}, errors.New("role not found")
	}
}
