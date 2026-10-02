package perms

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
	ScopeAdminRoles       = ScopeRoles("*")
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

// ScopeRoles -- Roles and permissions
func ScopeRoles(value string) Scope {
	return Scope{
		Name:        "roles",
		Description: "Roles",
		Value:       value,
	}
}
