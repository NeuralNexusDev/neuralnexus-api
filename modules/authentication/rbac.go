package authentication

// -------------- Structs --------------

type Scope struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var (
	ScopeBeeNameGenerator = Scope{
		Name:        "beenamegenerator",
		Description: "Bee name generator",
	}

	ScopePetPictures = Scope{
		Name:        "petpictures",
		Description: "Pet pictures",
	}
)

type Role struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

var (
	RoleSystem = Role{
		Name:        "RoleSystem",
		Description: "System role",
		Permissions: []string{
			ScopeBeeNameGenerator.Name,
			ScopePetPictures.Name,
		},
	}

	RoleOwner = Role{
		Name:        "RoleOwner",
		Description: "Owner role",
		Permissions: []string{
			ScopeBeeNameGenerator.Name,
			ScopePetPictures.Name,
		},
	}
)
