package perms

// Scope is a permission node. Valued nodes carry values on the roles that grant them.
type Scope struct {
	Node        string
	Description string
}

var (
	ScopeAdminBeeNameGenerator = Scope{Node: "beenamegenerator.admin", Description: "Bee name generator"}
	ScopeAdminPetPictures      = Scope{Node: "petpictures.admin", Description: "Pet pictures"}
	ScopeAdminDataStore        = Scope{Node: "datastore.admin", Description: "Data store"}
	ScopeAdminNumberStore      = Scope{Node: "numberstore.admin", Description: "Number store"}
	ScopeAdminUsers            = Scope{Node: "users.admin", Description: "Users"}
	ScopeAdminRoles            = Scope{Node: "roles.admin", Description: "Roles and permissions"}

	// ScopePetPictures is valued with the names of the pets a role may change.
	ScopePetPictures = Scope{Node: "petpictures.pets", Description: "Pet pictures"}
	// ScopeRateLimit is valued with the requests allowed per period.
	ScopeRateLimit = Scope{Node: "ratelimit", Description: "Rate limit"}
)
